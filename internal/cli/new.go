package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

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

	selected, err := selectPlugins(loader, opts)
	if err != nil {
		return err
	}
	if err := validateRequiresConflicts(selected); err != nil {
		return err
	}

	appName := slugify(opts.ProjectName)
	if appName == "" {
		return fmt.Errorf("%q is not a usable project name", opts.ProjectName)
	}

	values, err := collectValues(cmd, selected, opts, appName)
	if err != nil {
		return err
	}

	result, err := scaffold.Generate(scaffold.Options{Plugins: selected, Values: values})
	if err != nil {
		return err
	}
	manifestBytes, err := manifest.Marshal(buildProjectManifest(selected, values))
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

// selectPlugins resolves one plugin per Kind: an explicit --backend/--hosting
// name if given, otherwise the sole available plugin of that kind, otherwise
// (zero available) that kind is skipped. More than one available plugin of a
// kind with no explicit selection is an error asking to disambiguate — v0
// never hits that case, since every kind has at most one plugin, but the
// logic doesn't need to change once more plugins exist.
func selectPlugins(loader plugin.Loader, opts newOptions) ([]*plugin.Plugin, error) {
	explicit := map[plugin.Kind]string{
		plugin.KindBackend: opts.Backend,
		plugin.KindHosting: opts.Hosting,
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

func validateRequiresConflicts(selected []*plugin.Plugin) error {
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

var slugDisallowed = regexp.MustCompile(`[^a-z0-9-]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = slugDisallowed.ReplaceAllString(s, "")
	return strings.Trim(s, "-")
}

// collectValues resolves every placeholder declared across the selected
// plugins, in priority order: named flags and --set always win; with --yes,
// a manifest default fills the rest and a missing required value is an
// error; otherwise the user is prompted on stdin.
func collectValues(cmd *cobra.Command, selected []*plugin.Plugin, opts newOptions, appName string) (scaffold.Values, error) {
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
		"MODULE_PATH": opts.ModulePath,
		"DOMAIN":      opts.Domain,
		"ADMIN_EMAIL": opts.AdminEmail,
		"GHCR_OWNER":  opts.GHCROwner,
	}
	for k, v := range named {
		if v != "" {
			values[k] = v
		}
	}

	for _, kv := range opts.Set {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("--set %q is not in KEY=VALUE form", kv)
		}
		values[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	// Convenience default: derive GHCR_OWNER from the module path's org
	// segment (github.com/<owner>/...) before falling back to a prompt.
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

	var missing []string
	reader := bufio.NewReader(cmd.InOrStdin())
	for _, name := range order {
		if _, ok := values[name]; ok {
			continue
		}
		ph := declared[name]

		if opts.Yes {
			if ph.Default != "" {
				values[name] = ph.Default
			} else if ph.Required {
				missing = append(missing, name)
			}
			continue
		}

		prompt := ph.Description
		if ph.Default != "" {
			prompt += fmt.Sprintf(" [%s]", ph.Default)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): ", name, prompt)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			line = ph.Default
		}
		if line == "" && ph.Required {
			missing = append(missing, name)
			continue
		}
		if line != "" {
			values[name] = line
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required value(s): %s", strings.Join(missing, ", "))
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

func buildProjectManifest(selected []*plugin.Plugin, values scaffold.Values) manifest.Project {
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
		KilnVersion: Version,
		GeneratedAt: time.Now().UTC(),
		Module:      values["MODULE_PATH"],
		Plugins:     records,
		Values:      values,
	}
}
