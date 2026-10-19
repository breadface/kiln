package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/scaffold"
)

func newNewCmd() *cobra.Command {
	var outDir string
	var force bool
	var sel *selectionFlags

	cmd := &cobra.Command{
		Use:   "new <project-name>",
		Short: "Scaffold a new project from selected plugins",
		Long: "Scaffold a new project from selected plugins. Equivalent to\n" +
			"`kiln plan --out plan.yaml` followed by `kiln apply plan.yaml`.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := planFromOptions(cmd, sel.options(cmd, args[0]))
			if err != nil {
				return err
			}
			return writePlan(cmd, plan, outDir, force)
		},
	}

	sel = addSelectionFlags(cmd)
	cmd.Flags().StringVar(&outDir, "dir", ".", "parent directory to generate the project into")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the output directory if it already exists")
	return cmd
}

// writePlan writes plan's files under outDir/<APP_NAME>, runs post_copy
// hooks, and prints next steps — the only part of generation with side
// effects.
func writePlan(cmd *cobra.Command, plan *generate.Plan, outDir string, force bool) error {
	out := cmd.OutOrStdout()
	projectDir := filepath.Join(outDir, plan.Values["APP_NAME"])
	if _, err := os.Stat(projectDir); err == nil && !force {
		return fmt.Errorf("%s already exists (use --force to overwrite)", projectDir)
	}

	if err := scaffold.WriteToDisk(plan.Result, projectDir); err != nil {
		return err
	}
	if err := scaffold.RunHooks(plan.Plugins, projectDir, plan.Values); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nGenerated %s from %d plugin(s), %d contract binding(s)\n\n", projectDir, len(plan.Plugins), len(plan.Bindings))
	fmt.Fprintln(out, "Next steps:")
	fmt.Fprintf(out, "  cd %s\n", projectDir)
	for _, p := range plan.Plugins {
		for _, step := range p.Manifest.NextSteps {
			fmt.Fprintf(out, "  %s\n", scaffold.RenderString(step, plan.Values))
		}
	}
	return nil
}
