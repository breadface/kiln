package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	Version = "v0.1.0-test"
	defer func() { Version = "dev" }()

	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("kiln version: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "v0.1.0-test" {
		t.Errorf("output = %q, want %q", got, "v0.1.0-test")
	}
}

func TestRootHelp(t *testing.T) {
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("kiln --help: %v", err)
	}
	if !strings.Contains(buf.String(), "kiln") {
		t.Error("--help output should mention the command name")
	}
}
