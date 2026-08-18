// Command kiln scaffolds new projects from composable plugins.
package main

import (
	"fmt"
	"os"

	"github.com/breadface/kiln/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
