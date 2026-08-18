// Package generate holds the UI-agnostic core of `kiln new`: resolving
// which plugins apply and what values they need. It's imported by both the
// terminal CLI and the WebAssembly build behind the web app — neither
// Cobra nor anything that reads from stdin belongs here, since only the
// CLI can prompt a human.
package generate

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/breadface/kiln/internal/manifest"
	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/internal/scaffold"
)

// Request is every value a caller (CLI flags/--set, or a web form) can
// supply, independent of how it was collected.
type Request struct {
	ProjectName string
	Backend     string
	Hosting     string
	ModulePath  string
	Domain      string
	AdminEmail  string
	GHCROwner   string
	// Set holds explicit placeholder overrides, keyed by placeholder name —
	// the parsed form of the CLI's repeatable --set KEY=VALUE flag. Always
	// wins over every other source.
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

// SelectPlugins resolves one plugin per Kind: an explicit Backend/Hosting
// name if given, otherwise the sole available plugin of that kind,
// otherwise (zero available) that kind is skipped. More than one available
// plugin of a kind with no explicit selection is an error asking to
// disambiguate — today's plugin set never hits that case (every kind has
// at most one plugin), but the logic doesn't need to change once more
// plugins exist.
func SelectPlugins(loader plugin.Loader, req Request) ([]*plugin.Plugin, error) {
	explicit := map[plugin.Kind]string{
		plugin.KindBackend: req.Backend,
		plugin.KindHosting: req.Hosting,
	}
	kinds := []plugin.Kind{
		plugin.KindBackend, plugin.KindFrontend, plugin.KindMobile,
		plugin.KindPayments, plugin.KindHosting, plugin.KindGlue,
	}

	var selected []*plugin.Plugin
	for _, k := range kinds {
		if name := explicit[k]; name != "" {
			p, err := loader.Resolve(k, name)
			if err != nil {
				return nil, err
			}
			selected = append(selected, p)
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
	return string(p.Manifest.Kind) + "/" + p.Manifest.Name
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
// plugins, without prompting: request fields and req.Set always win over a
// manifest default. Anything required, still unset, and without a default
// comes back as *MissingValuesError.
func ResolveValues(selected []*plugin.Plugin, req Request, appName string) (scaffold.Values, error) {
	serviceName := appName + "-api"
	values := scaffold.Values{
		"APP_NAME":     appName,
		"SERVICE_NAME": serviceName,
		// Upper-snake form of SERVICE_NAME, e.g. "demoapp-api" -> "DEMOAPP_API" —
		// used to build a per-service image-tag env var name like
		// ${DEMOAPP_API_TAG:-latest}, matching afterknot's convention.
		"SERVICE_NAME_UPPER": strings.ToUpper(strings.ReplaceAll(serviceName, "-", "_")),
		"STACK_NAME":         appName,
	}

	named := map[string]string{
		"MODULE_PATH": req.ModulePath,
		"DOMAIN":      req.Domain,
		"ADMIN_EMAIL": req.AdminEmail,
		"GHCR_OWNER":  req.GHCROwner,
	}
	for k, v := range named {
		if v != "" {
			values[k] = v
		}
	}

	for k, v := range req.Set {
		if v != "" {
			values[k] = v
		}
	}

	// Convenience default: derive GHCR_OWNER from the module path's org
	// segment (github.com/<owner>/...) before falling back to "missing".
	if _, ok := values["GHCR_OWNER"]; !ok {
		if owner := ghcrOwnerFromModule(values["MODULE_PATH"]); owner != "" {
			values["GHCR_OWNER"] = owner
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

	var missing []plugin.Placeholder
	for _, name := range order {
		if _, ok := values[name]; ok {
			continue
		}
		ph := declared[name]
		if ph.Default != "" {
			values[name] = ph.Default
		} else if ph.Required {
			missing = append(missing, ph)
		}
	}

	if len(missing) > 0 {
		return nil, &MissingValuesError{Missing: missing}
	}
	return values, nil
}

func ghcrOwnerFromModule(modulePath string) string {
	if modulePath == "" {
		return ""
	}
	parts := strings.Split(modulePath, "/")
	if len(parts) >= 2 && parts[0] == "github.com" {
		return parts[1]
	}
	return ""
}

// BuildManifest assembles the kiln.yaml payload for a generated project.
// Serialization stays in internal/manifest.Marshal.
func BuildManifest(kilnVersion string, selected []*plugin.Plugin, values scaffold.Values) manifest.Project {
	records := make([]manifest.PluginRecord, len(selected))
	for i, p := range selected {
		records[i] = manifest.PluginRecord{
			Kind:    string(p.Manifest.Kind),
			Name:    p.Manifest.Name,
			Version: p.Manifest.Version,
			Source:  p.Source,
		}
	}
	return manifest.Project{
		KilnVersion: kilnVersion,
		GeneratedAt: time.Now().UTC(),
		Module:      values["MODULE_PATH"],
		Plugins:     records,
		Values:      values,
	}
}
