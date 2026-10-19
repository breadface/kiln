// Package generate holds the UI-agnostic core of `kiln new`: resolving
// which plugins apply and what values they need. It's imported by both the
// terminal CLI and the WebAssembly build behind the web app — neither
// Cobra nor anything that reads from stdin belongs here, since only the
// CLI can prompt a human.
package generate

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/breadface/kiln/internal/contract"
	"github.com/breadface/kiln/internal/manifest"
	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/internal/scaffold"
)

// Request is every value a caller (CLI flags/--set, or a web form) can
// supply, independent of how it was collected. It deliberately has no
// plugin-specific fields: plugin selection is keyed by Kind, and every
// value a plugin needs is a placeholder it declares itself.
type Request struct {
	ProjectName string
	// Plugins holds explicit plugin selections, keyed by Kind — e.g.
	// {backend: go-api, mobile: kmp-compose}.
	Plugins map[plugin.Kind]string
	// Set holds explicit placeholder values, keyed by placeholder name —
	// from a plugin-declared CLI flag (e.g. --module) or --set KEY=VALUE.
	Set map[string]string
}

var slugDisallowed = regexp.MustCompile(`[^a-z0-9-]+`)

// Slugify turns a project name into a safe APP_NAME: lowercase, spaces and
// underscores become hyphens, anything else disallowed is dropped.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = slugDisallowed.ReplaceAllString(s, "")
	return strings.Trim(s, "-")
}

// autoSelectKinds get chosen automatically when exactly one plugin exists
// for them — nearly every project wants a backend and a hosting setup.
// Every other kind (frontend, mobile, payments, glue) is opt-in only, even
// with just one plugin available: not every project wants, say, a mobile
// client, so silently adding one on every `kiln new` would be a real
// regression, not a convenience.
var autoSelectKinds = map[plugin.Kind]bool{
	plugin.KindBackend: true,
	plugin.KindHosting: true,
}

// AutoSelects reports whether k is chosen automatically when exactly one
// plugin of that kind is available.
func AutoSelects(k plugin.Kind) bool {
	return autoSelectKinds[k]
}

// SelectPlugins resolves one plugin per Kind: an explicit name (from
// req.Plugins) if given, otherwise — for auto-select kinds only
// — the sole available plugin of that kind, otherwise that kind is
// skipped. More than one available plugin of a kind with no explicit
// selection is an error asking to disambiguate — today's plugin set never
// hits that case (every kind has at most one plugin), but the logic
// doesn't need to change once more plugins exist.
func SelectPlugins(loader plugin.Loader, req Request) ([]*plugin.Plugin, error) {
	for k := range req.Plugins {
		if !k.Known() {
			return nil, fmt.Errorf("unknown plugin kind %q", k)
		}
	}

	var selected []*plugin.Plugin
	for _, k := range plugin.Kinds() {
		if name := req.Plugins[k]; name != "" {
			p, err := loader.Resolve(k, name)
			if err != nil {
				return nil, err
			}
			selected = append(selected, p)
			continue
		}

		if !autoSelectKinds[k] {
			continue
		}

		available, err := loader.List(k)
		if err != nil {
			return nil, err
		}
		switch len(available) {
		case 0:
			continue
		case 1:
			selected = append(selected, available[0])
		default:
			names := make([]string, len(available))
			for i, p := range available {
				names[i] = p.Manifest.Name
			}
			return nil, fmt.Errorf("multiple %s plugins available (%s) — pick one explicitly", k, strings.Join(names, ", "))
		}
	}
	return selected, nil
}

// ValidateRequiresConflicts checks every selected plugin's Requires and
// Conflicts against the full selection.
func ValidateRequiresConflicts(selected []*plugin.Plugin) error {
	selectedKinds := map[plugin.Kind]bool{}
	selectedRefs := map[string]bool{}
	for _, p := range selected {
		selectedKinds[p.Manifest.Kind] = true
		selectedRefs[pluginRefStr(p)] = true
	}

	has := func(ref string) bool {
		if strings.Contains(ref, "/") {
			return selectedRefs[ref]
		}
		return selectedKinds[plugin.Kind(ref)]
	}

	for _, p := range selected {
		for _, req := range p.Manifest.Requires {
			if !has(req) {
				return fmt.Errorf("plugin %s requires %s, which is not selected", pluginRefStr(p), req)
			}
		}
		for _, conf := range p.Manifest.Conflicts {
			if has(conf) {
				return fmt.Errorf("plugin %s conflicts with %s, which is also selected", pluginRefStr(p), conf)
			}
		}
	}
	return nil
}

func pluginRefStr(p *plugin.Plugin) string {
	return p.Ref()
}

// MissingValuesError reports required placeholders ResolveValues couldn't
// fill from the request or a manifest default. Callers that can prompt a
// human (the CLI) do so for exactly these and call ResolveValues again;
// callers that can't (the web form) surface it as a validation error.
type MissingValuesError struct {
	Missing []plugin.Placeholder
}

func (e *MissingValuesError) Error() string {
	names := make([]string, len(e.Missing))
	for i, ph := range e.Missing {
		names[i] = ph.Name
	}
	return "missing required value(s): " + strings.Join(names, ", ")
}

// ResolveValues resolves every placeholder declared across the selected
// plugins, without prompting. APP_NAME is the only value Kiln itself
// defines; everything else comes from, in priority order:
//
//  1. req.Set (explicit flags / --set),
//  2. a placeholder's derive rule, once its source value is known,
//  3. a placeholder's default, which may reference other placeholders
//     (e.g. "{{APP_NAME}}-api") and resolves once they're known.
//
// Anything required and still unset comes back as *MissingValuesError.
func ResolveValues(selected []*plugin.Plugin, req Request, appName string) (scaffold.Values, error) {
	values := scaffold.Values{"APP_NAME": appName}
	for k, v := range req.Set {
		if v != "" {
			values[k] = v
		}
	}

	declared := map[string]plugin.Placeholder{}
	var order []string
	for _, p := range selected {
		for _, ph := range p.Manifest.Placeholders {
			if _, seen := declared[ph.Name]; !seen {
				order = append(order, ph.Name)
			}
			declared[ph.Name] = ph
		}
	}

	// Derived values and defaults can depend on each other in any order,
	// so keep making passes until one resolves nothing new.
	for changed := true; changed; {
		changed = false
		for _, name := range order {
			if _, ok := values[name]; ok {
				continue
			}
			ph := declared[name]
			if v, ok, err := derive(ph, values); err != nil {
				return nil, err
			} else if ok {
				values[name], changed = v, true
				continue
			}
			if ph.Default != "" {
				if v := scaffold.RenderString(ph.Default, values); !scaffold.HasUnresolved([]byte(v)) {
					values[name], changed = v, true
				}
			}
		}
	}

	// Required values are reported first: a default that depends on one
	// will resolve once the caller supplies it (e.g. via a prompt).
	var missing []plugin.Placeholder
	var unresolvable string
	for _, name := range order {
		if _, ok := values[name]; ok {
			continue
		}
		ph := declared[name]
		switch {
		case ph.Required:
			missing = append(missing, ph)
		case ph.Default != "" && unresolvable == "":
			unresolvable = fmt.Sprintf("default for %s (%q) references a value that never resolved", name, ph.Default)
		}
	}
	if len(missing) > 0 {
		return nil, &MissingValuesError{Missing: missing}
	}
	if unresolvable != "" {
		return nil, errors.New(unresolvable)
	}
	return values, nil
}

// derive applies ph's Derive rule, if it has one and its source is known.
func derive(ph plugin.Placeholder, values scaffold.Values) (string, bool, error) {
	d := ph.Derive
	if d == nil {
		return "", false, nil
	}
	src, ok := values[d.From]
	if !ok {
		return "", false, nil
	}
	if d.Match != "" {
		re, err := regexp.Compile(d.Match)
		if err != nil {
			return "", false, fmt.Errorf("placeholder %s: invalid derive.match: %w", ph.Name, err)
		}
		m := re.FindStringSubmatchIndex(src)
		if m == nil {
			return "", false, nil
		}
		return string(re.ExpandString(nil, d.Template, src, m)), true, nil
	}
	keys := make([]string, 0, len(d.Replace))
	for k := range d.Replace {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic when replacements overlap
	pairs := make([]string, 0, 2*len(keys))
	for _, k := range keys {
		pairs = append(pairs, k, d.Replace[k])
	}
	return strings.NewReplacer(pairs...).Replace(src), true, nil
}

// ResolveBindings connects every contract a selected plugin consumes to the
// selected plugins providing it, and returns the bindings plus values
// extended with each `as`-exported consume (e.g. API_PORT).
//
// Every provide is validated against its contract schema even when nothing
// consumes it, so a broken plugin manifest fails fast regardless of what
// it's combined with.
func ResolveBindings(selected []*plugin.Plugin, registry *contract.Registry, values scaffold.Values) ([]plugin.Binding, scaffold.Values, error) {
	type provided struct {
		plugin *plugin.Plugin
		values map[string]string
	}
	providers := map[string][]provided{}
	for _, p := range selected {
		for _, pr := range p.Manifest.Provides {
			c, ok := registry.Get(pr.Contract)
			if !ok {
				return nil, nil, fmt.Errorf("plugin %s provides unknown contract %q", p.Ref(), pr.Contract)
			}
			rendered := map[string]string{}
			for k, v := range pr.With {
				r := scaffold.RenderString(v, values)
				if scaffold.HasUnresolved([]byte(r)) {
					return nil, nil, fmt.Errorf("plugin %s provides %s with %s=%q, which references an unresolved value", p.Ref(), pr.Contract, k, v)
				}
				rendered[k] = r
			}
			applied, err := c.Apply(rendered)
			if err != nil {
				return nil, nil, fmt.Errorf("plugin %s: %w", p.Ref(), err)
			}
			providers[pr.Contract] = append(providers[pr.Contract], provided{p, applied})
		}
	}

	out := scaffold.Values{}
	for k, v := range values {
		out[k] = v
	}

	var bindings []plugin.Binding
	for _, consumer := range selected {
		for _, cn := range consumer.Manifest.Consumes {
			c, ok := registry.Get(cn.Contract)
			if !ok {
				return nil, nil, fmt.Errorf("plugin %s consumes unknown contract %q", consumer.Ref(), cn.Contract)
			}
			if cn.As != "" && cn.Max != 1 {
				return nil, nil, fmt.Errorf("plugin %s consumes %s as %s, which requires max: 1", consumer.Ref(), cn.Contract, cn.As)
			}
			bound := providers[cn.Contract]
			if len(bound) < cn.Min {
				return nil, nil, fmt.Errorf("plugin %s needs at least %d %s provider(s), but %s", consumer.Ref(), cn.Min, cn.Contract, describeProviders(bound))
			}
			if cn.Max > 0 && len(bound) > cn.Max {
				return nil, nil, fmt.Errorf("plugin %s accepts at most %d %s provider(s), but %s", consumer.Ref(), cn.Max, cn.Contract, describeProviders(bound))
			}
			for _, b := range bound {
				bindings = append(bindings, plugin.Binding{
					Contract: c.ID(), Consumer: consumer, Provider: b.plugin, Values: b.values,
				})
				if cn.As == "" {
					continue
				}
				for k, v := range b.values {
					name := cn.As + "_" + k
					if existing, ok := out[name]; ok && existing != v {
						return nil, nil, fmt.Errorf("plugin %s exports %s=%q, but it's already %q", consumer.Ref(), name, v, existing)
					}
					out[name] = v
				}
			}
		}
	}
	return bindings, out, nil
}

func describeProviders[T any](bound []T) string {
	switch len(bound) {
	case 0:
		return "no selected plugin provides it"
	case 1:
		return "1 selected plugin provides it"
	default:
		return fmt.Sprintf("%d selected plugins provide it", len(bound))
	}
}

// BuildManifest assembles the kiln.yaml payload for a generated project.
// Serialization stays in internal/manifest.Marshal.
func BuildManifest(kilnVersion string, selected []*plugin.Plugin, bindings []plugin.Binding, values scaffold.Values) manifest.Project {
	records := make([]manifest.PluginRecord, len(selected))
	for i, p := range selected {
		records[i] = manifest.PluginRecord{
			Kind:    string(p.Manifest.Kind),
			Name:    p.Manifest.Name,
			Version: p.Manifest.Version,
			Source:  p.Source,
		}
	}
	bindingRecords := make([]manifest.BindingRecord, len(bindings))
	for i, b := range bindings {
		bindingRecords[i] = manifest.BindingRecord{
			Contract: b.Contract,
			Consumer: b.Consumer.Ref(),
			Provider: b.Provider.Ref(),
			Values:   b.Values,
		}
	}
	return manifest.Project{
		KilnVersion: kilnVersion,
		GeneratedAt: time.Now().UTC(),
		Plugins:     records,
		Bindings:    bindingRecords,
		Values:      values,
	}
}
