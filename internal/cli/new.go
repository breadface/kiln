package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/breadface/kiln/contracts"
	"github.com/breadface/kiln/internal/contract"
	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/manifest"
	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/internal/scaffold"
	"github.com/breadface/kiln/plugins"
)

type newOptions struct {
	ProjectName string
	// Plugins holds explicit selections from the per-Kind flags, e.g.
	// --backend go-api.
	Plugins map[plugin.Kind]string
	// Values holds placeholder values from plugin-declared flags, e.g.
	// --module sets MODULE_PATH. --set overrides these.
	Values map[string]string
	OutDir string
	Set    []string
	Yes    bool
	Force  bool
}

// valueFlag is a CLI flag some plugin's placeholder declared.
type valueFlag struct {
	placeholder string
	value       string
}

func newNewCmd() *cobra.Command {
	opts := newOptions{}
	kindFlags := map[plugin.Kind]*string{}
	valueFlags := map[string]*valueFlag{}

	cmd := &cobra.Command{
		Use:   "new <project-name>",
		Short: "Scaffold a new project from selected plugins",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ProjectName = args[0]
			opts.Plugins = map[plugin.Kind]string{}
			for k, v := range kindFlags {
				if *v != "" {
					opts.Plugins[k] = *v
				}
			}
			opts.Values = map[string]string{}
			for name, vf := range valueFlags {
				if cmd.Flags().Changed(name) {
					opts.Values[vf.placeholder] = vf.value
				}
			}
			return runNew(cmd, opts)
		},
	}

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
		kindFlags[k] = f.String(string(k), "", usage)
	}

	// One value flag per placeholder that asks for one. Two plugins may
	// declare the same flag only for the same placeholder.
	for _, p := range all {
		for _, ph := range p.Manifest.Placeholders {
			if ph.Flag == "" {
				continue
			}
			if existing, ok := valueFlags[ph.Flag]; ok {
				if existing.placeholder != ph.Name {
					panic(fmt.Sprintf("plugin %s: flag --%s is already bound to %s, not %s", p.Ref(), ph.Flag, existing.placeholder, ph.Name))
				}
				continue
			}
			vf := &valueFlag{placeholder: ph.Name}
			valueFlags[ph.Flag] = vf
			f.StringVar(&vf.value, ph.Flag, "", fmt.Sprintf("%s (sets %s; used by %s)", ph.Description, ph.Name, p.Ref()))
		}
	}

	f.StringVar(&opts.OutDir, "dir", ".", "parent directory to generate the project into")
	f.StringArrayVar(&opts.Set, "set", nil, "set a placeholder value directly, KEY=VALUE (repeatable)")
	f.BoolVar(&opts.Yes, "yes", false, "don't prompt; use flags/defaults, error if a required value is missing")
	f.BoolVar(&opts.Force, "force", false, "overwrite the output directory if it already exists")

	return cmd
}

func runNew(cmd *cobra.Command, opts newOptions) error {
	out := cmd.OutOrStdout()
	loader := plugin.NewEmbedLoader(plugins.FS)
	registry, err := contract.Load(contracts.FS)
	if err != nil {
		return err
	}

	req, err := requestFromOptions(opts)
	if err != nil {
		return err
	}

	selected, err := generate.SelectPlugins(loader, req)
	if err != nil {
		return err
	}
	if err := generate.ValidateRequiresConflicts(selected); err != nil {
		return err
	}

	appName := generate.Slugify(opts.ProjectName)
	if appName == "" {
		return fmt.Errorf("%q is not a usable project name", opts.ProjectName)
	}

	values, err := collectValues(cmd, selected, req, appName, opts.Yes)
	if err != nil {
		return err
	}
	bindings, values, err := generate.ResolveBindings(selected, registry, values)
	if err != nil {
		return err
	}

	result, err := scaffold.Generate(scaffold.Options{Plugins: selected, Bindings: bindings, Values: values})
	if err != nil {
		return err
	}
	manifestBytes, err := manifest.Marshal(generate.BuildManifest(Version, selected, bindings, values))
	if err != nil {
		return err
	}
	result.Files[manifest.FileName] = scaffold.GeneratedFile{Content: manifestBytes}

	projectDir := filepath.Join(opts.OutDir, appName)
	if _, err := os.Stat(projectDir); err == nil && !opts.Force {
		return fmt.Errorf("%s already exists (use --force to overwrite)", projectDir)
	}

	if err := scaffold.WriteToDisk(result, projectDir); err != nil {
		return err
	}
	if err := scaffold.RunHooks(selected, projectDir, values); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nGenerated %s from %d plugin(s), %d contract binding(s)\n\n", projectDir, len(selected), len(bindings))
	fmt.Fprintln(out, "Next steps:")
	fmt.Fprintf(out, "  cd %s\n", projectDir)
	for _, p := range selected {
		for _, step := range p.Manifest.NextSteps {
			fmt.Fprintf(out, "  %s\n", scaffold.RenderString(step, values))
		}
	}
	return nil
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
