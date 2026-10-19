package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/breadface/kiln/contracts"
	"github.com/breadface/kiln/internal/contract"
	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/internal/scaffold"
	"github.com/breadface/kiln/plugins"
)

// newOptions is everything a person can choose when generating a project.
type newOptions struct {
	ProjectName string
	// Plugins holds explicit selections from the per-Kind flags, e.g.
	// --backend go-api.
	Plugins map[plugin.Kind]string
	// Values holds placeholder values from plugin-declared flags, e.g.
	// --module sets MODULE_PATH. --set overrides these.
	Values map[string]string
	Set    []string
	Yes    bool
}

// valueFlag is a CLI flag some plugin's placeholder declared.
type valueFlag struct {
	placeholder string
	value       string
}

// selectionFlags are the flags shared by every command that selects
// plugins and values (new, plan). None of them are hardcoded: there's one
// selection flag per Kind, and one value flag per placeholder a plugin
// declared a `flag:` for.
type selectionFlags struct {
	kinds  map[plugin.Kind]*string
	values map[string]*valueFlag
	set    []string
	yes    bool
}

func addSelectionFlags(cmd *cobra.Command) *selectionFlags {
	s := &selectionFlags{kinds: map[plugin.Kind]*string{}, values: map[string]*valueFlag{}}
	f := cmd.Flags()

	all, err := plugin.NewEmbedLoader(plugins.FS).List("")
	if err != nil {
		// The embedded plugin set is fixed at build time, so this only
		// happens with a broken plugin.yaml — which every test catches.
		panic(fmt.Sprintf("loading embedded plugins: %v", err))
	}

	// One selection flag per Kind, listing what's available for it.
	for _, k := range plugin.Kinds() {
		var names []string
		for _, p := range all {
			if p.Manifest.Kind == k {
				names = append(names, p.Manifest.Name)
			}
		}
		usage := fmt.Sprintf("%s plugin to use", k)
		if len(names) > 0 {
			usage += " (" + strings.Join(names, ", ") + ")"
		} else {
			usage += " (none available yet)"
		}
		if generate.AutoSelects(k) {
			usage += "; auto-selected if only one is available"
		} else {
			usage += "; opt-in"
		}
		s.kinds[k] = f.String(string(k), "", usage)
	}

	// One value flag per placeholder that asks for one. Two plugins may
	// declare the same flag only for the same placeholder.
	for _, p := range all {
		for _, ph := range p.Manifest.Placeholders {
			if ph.Flag == "" {
				continue
			}
			if existing, ok := s.values[ph.Flag]; ok {
				if existing.placeholder != ph.Name {
					panic(fmt.Sprintf("plugin %s: flag --%s is already bound to %s, not %s", p.Ref(), ph.Flag, existing.placeholder, ph.Name))
				}
				continue
			}
			vf := &valueFlag{placeholder: ph.Name}
			s.values[ph.Flag] = vf
			f.StringVar(&vf.value, ph.Flag, "", fmt.Sprintf("%s (sets %s; used by %s)", ph.Description, ph.Name, p.Ref()))
		}
	}

	f.StringArrayVar(&s.set, "set", nil, "set a placeholder value directly, KEY=VALUE (repeatable)")
	f.BoolVar(&s.yes, "yes", false, "don't prompt; use flags/defaults, error if a required value is missing")
	return s
}

// options collects the parsed flags into newOptions.
func (s *selectionFlags) options(cmd *cobra.Command, projectName string) newOptions {
	opts := newOptions{
		ProjectName: projectName,
		Plugins:     map[plugin.Kind]string{},
		Values:      map[string]string{},
		Set:         s.set,
		Yes:         s.yes,
	}
	for k, v := range s.kinds {
		if *v != "" {
			opts.Plugins[k] = *v
		}
	}
	for name, vf := range s.values {
		if cmd.Flags().Changed(name) {
			opts.Values[vf.placeholder] = vf.value
		}
	}
	return opts
}

// planFromOptions selects plugins, resolves (and if allowed, prompts for)
// values, and computes the full plan — everything short of writing it.
func planFromOptions(cmd *cobra.Command, opts newOptions) (*generate.Plan, error) {
	req, err := requestFromOptions(opts)
	if err != nil {
		return nil, err
	}
	selected, err := generate.SelectPlugins(plugin.NewEmbedLoader(plugins.FS), req)
	if err != nil {
		return nil, err
	}
	if err := generate.ValidateRequiresConflicts(selected); err != nil {
		return nil, err
	}

	appName := generate.Slugify(opts.ProjectName)
	if appName == "" {
		return nil, fmt.Errorf("%q is not a usable project name", opts.ProjectName)
	}
	values, err := collectValues(cmd, selected, req, appName, opts.Yes)
	if err != nil {
		return nil, err
	}
	return makePlan(selected, values)
}

func makePlan(selected []*plugin.Plugin, values scaffold.Values) (*generate.Plan, error) {
	registry, err := contract.Load(contracts.FS)
	if err != nil {
		return nil, err
	}
	return generate.MakePlan(Version, selected, registry, values)
}

func requestFromOptions(opts newOptions) (generate.Request, error) {
	set := map[string]string{}
	for k, v := range opts.Values {
		set[k] = v
	}
	for _, kv := range opts.Set {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return generate.Request{}, fmt.Errorf("--set %q is not in KEY=VALUE form", kv)
		}
		set[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return generate.Request{
		ProjectName: opts.ProjectName,
		Plugins:     opts.Plugins,
		Set:         set,
	}, nil
}

// collectValues resolves every placeholder declared across the selected
// plugins via generate.ResolveValues; if that reports required values
// still missing and the caller hasn't passed --yes, it prompts on stdin
// for exactly those and resolves once more.
func collectValues(cmd *cobra.Command, selected []*plugin.Plugin, req generate.Request, appName string, yes bool) (scaffold.Values, error) {
	values, err := generate.ResolveValues(selected, req, appName)

	var missingErr *generate.MissingValuesError
	if err != nil {
		if yes || !errors.As(err, &missingErr) {
			return nil, err
		}

		reader := bufio.NewReader(cmd.InOrStdin())
		for _, ph := range missingErr.Missing {
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): ", ph.Name, ph.Description)
			line, _ := reader.ReadString('\n')
			if line = strings.TrimSpace(line); line != "" {
				req.Set[ph.Name] = line
			}
		}

		values, err = generate.ResolveValues(selected, req, appName)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}
