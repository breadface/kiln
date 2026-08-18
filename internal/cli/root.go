// Package cli wires Kiln's Cobra commands.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags, e.g.:
//
//	go build -ldflags "-X github.com/surebits/kiln/internal/cli.Version=v0.1.0"
var Version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "kiln",
		Short: "Kiln scaffolds new projects from composable plugins",
		Long: "Kiln composes independent, reusable plugins — a backend, a hosting\n" +
			"setup, and so on — into a new project, instead of copy-pasting\n" +
			"boilerplate between projects.",
		SilenceUsage: true,
	}
	root.AddCommand(newVersionCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the kiln version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), Version)
			return nil
		},
	}
}

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}
