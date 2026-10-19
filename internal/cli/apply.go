package cli

import (
	"github.com/spf13/cobra"

	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/manifest"
	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/plugins"
)

func newApplyCmd() *cobra.Command {
	var outDir string
	var force bool

	cmd := &cobra.Command{
		Use:   "apply [plan-file]",
		Short: "Generate a project from a saved plan or an existing kiln.yaml",
		Long: "Generate a project from a plan saved with `kiln plan --out`, or from\n" +
			"any generated project's kiln.yaml (default: ./kiln.yaml). Output is\n" +
			"identical to the original as long as every plugin is unchanged —\n" +
			"apply refuses to run if a plugin's version or content differs from\n" +
			"what the plan recorded.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := manifest.FileName
			if len(args) == 1 {
				path = args[0]
			}
			plan, err := planFromFile(path)
			if err != nil {
				return err
			}
			return writePlan(cmd, plan, outDir, force)
		},
	}

	cmd.Flags().StringVar(&outDir, "dir", ".", "parent directory to generate the project into")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the output directory if it already exists")
	return cmd
}

// planFromFile recomputes the plan a kiln.yaml-format file recorded.
func planFromFile(path string) (*generate.Plan, error) {
	m, err := manifest.ReadFile(path)
	if err != nil {
		return nil, err
	}
	selected, req, appName, err := generate.FromManifest(plugin.NewEmbedLoader(plugins.FS), m)
	if err != nil {
		return nil, err
	}
	if err := generate.ValidateRequiresConflicts(selected); err != nil {
		return nil, err
	}
	values, err := generate.ResolveValues(selected, req, appName)
	if err != nil {
		return nil, err
	}
	return makePlan(selected, values)
}
