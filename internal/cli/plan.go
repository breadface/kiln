package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/manifest"
)

func newPlanCmd() *cobra.Command {
	var asJSON bool
	var outFile string
	var sel *selectionFlags

	cmd := &cobra.Command{
		Use:   "plan <project-name>",
		Short: "Show what `kiln new` would generate, without writing anything",
		Long: "Resolve plugins, values and contract bindings and render every file\n" +
			"in memory, then print a summary. Nothing is written and no hooks run.\n\n" +
			"--out saves the plan in kiln.yaml format for `kiln apply`; --json\n" +
			"prints it as JSON for tools and agents.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := planFromOptions(cmd, sel.options(cmd, args[0]))
			if err != nil {
				return err
			}
			if outFile != "" {
				data := plan.Result.Files[manifest.FileName].Content
				if err := os.WriteFile(outFile, data, 0o644); err != nil {
					return fmt.Errorf("writing plan: %w", err)
				}
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(plan.Manifest)
			}
			printPlan(cmd.OutOrStdout(), plan)
			if outFile != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nSaved plan to %s — generate it with: kiln apply %s\n", outFile, outFile)
			}
			return nil
		},
	}

	sel = addSelectionFlags(cmd)
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the plan as JSON")
	cmd.Flags().StringVar(&outFile, "out", "", "save the plan (kiln.yaml format) to this file")
	return cmd
}

// printPlan writes a human-readable summary of plan to w.
func printPlan(w io.Writer, plan *generate.Plan) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintln(tw, "Plugins")
	for _, p := range plan.Plugins {
		fmt.Fprintf(tw, "  %s\t%s\n", p.Ref(), p.Manifest.Version)
	}

	fmt.Fprintln(tw, "\nBindings")
	if len(plan.Bindings) == 0 {
		fmt.Fprintln(tw, "  (none)")
	}
	for _, b := range plan.Bindings {
		fmt.Fprintf(tw, "  %s\t%s\t%s -> %s\n", b.Contract, b.Label, b.Provider.Ref(), b.Consumer.Ref())
	}

	fmt.Fprintln(tw, "\nValues")
	for _, k := range sortedKeys(plan.Values) {
		fmt.Fprintf(tw, "  %s\t%s\n", k, plan.Values[k])
	}

	tw.Flush()

	fmt.Fprintf(w, "\nFiles (%d)\n", len(plan.Result.Files))
	for _, path := range sortedKeys(plan.Result.Files) {
		suffix := ""
		if plan.Result.Files[path].Executable {
			suffix = " (executable)"
		}
		fmt.Fprintf(w, "  %s%s\n", path, suffix)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
