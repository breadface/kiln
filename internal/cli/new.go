package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/manifest"
	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/internal/scaffold"
	"github.com/breadface/kiln/plugins"
)

type newOptions struct {
	ProjectName string
	Backend     string
	Hosting     string
	ModulePath  string
	Domain      string
	AdminEmail  string
	GHCROwner   string
	OutDir      string
	Set         []string
	Yes         bool
	Force       bool
}

func newNewCmd() *cobra.Command {
	opts := newOptions{}

	cmd := &cobra.Command{
		Use:   "new <project-name>",
		Short: "Scaffold a new project from selected plugins",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ProjectName = args[0]
			return runNew(cmd, opts)
		},
	}

	f := cmd.Flags()
	f.StringVar(&opts.Backend, "backend", "", "backend plugin to use (auto-selected if only one is available)")
	f.StringVar(&opts.Hosting, "hosting", "", "hosting plugin to use (auto-selected if only one is available)")
	f.StringVar(&opts.ModulePath, "module", "", "Go module path for the generated API")
	f.StringVar(&opts.Domain, "domain", "", "primary domain the hosting plugin will serve")
	f.StringVar(&opts.AdminEmail, "admin-email", "", "email Caddy uses for TLS certificate registration")
	f.StringVar(&opts.GHCROwner, "ghcr-owner", "", "GHCR namespace to push images to (defaults to the module path's org)")
	f.StringVar(&opts.OutDir, "dir", ".", "parent directory to generate the project into")
	f.StringArrayVar(&opts.Set, "set", nil, "set a placeholder value directly, KEY=VALUE (repeatable)")
	f.BoolVar(&opts.Yes, "yes", false, "don't prompt; use flags/defaults, error if a required value is missing")
	f.BoolVar(&opts.Force, "force", false, "overwrite the output directory if it already exists")

	return cmd
}

func runNew(cmd *cobra.Command, opts newOptions) error {
	out := cmd.OutOrStdout()
	loader := plugin.NewEmbedLoader(plugins.FS)

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

	result, err := scaffold.Generate(scaffold.Options{Plugins: selected, Values: values})
	if err != nil {
		return err
	}
	manifestBytes, err := manifest.Marshal(generate.BuildManifest(Version, selected, values))
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

	fmt.Fprintf(out, "\nGenerated %s\n\n", projectDir)
	fmt.Fprintln(out, "Next steps:")
	fmt.Fprintf(out, "  cd %s\n", projectDir)
	fmt.Fprintf(out, "  cp %s-api/.env.example %s-api/.env\n", appName, appName)
	fmt.Fprintf(out, "  cd %s-api && make run\n", appName)
	return nil
}

func requestFromOptions(opts newOptions) (generate.Request, error) {
	set := map[string]string{}
	for _, kv := range opts.Set {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return generate.Request{}, fmt.Errorf("--set %q is not in KEY=VALUE form", kv)
		}
		set[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return generate.Request{
		ProjectName: opts.ProjectName,
		Backend:     opts.Backend,
		Hosting:     opts.Hosting,
		ModulePath:  opts.ModulePath,
		Domain:      opts.Domain,
		AdminEmail:  opts.AdminEmail,
		GHCROwner:   opts.GHCROwner,
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
