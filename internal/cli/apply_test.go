package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/manifest"
)

// scenarios are the plugin combinations Kiln supports today. They drive
// the reproducibility and golden tests, so a new plugin should add its
// combinations here.
var scenarios = map[string][]string{
	"go-api": {
		"--backend", "go-api",
	},
	"go-grpc-gateway": {
		"--backend", "go-grpc-gateway",
	},
	"go-api+kmp-compose": {
		"--backend", "go-api",
		"--mobile", "kmp-compose", "--mobile-package", "com.example.demo",
	},
	"go-grpc-gateway+kmp-compose": {
		"--backend", "go-grpc-gateway",
		"--mobile", "kmp-compose", "--mobile-package", "com.example.demo",
	},
	"go-api+stripe": {
		"--backend", "go-api",
		"--payments", "stripe",
	},
	"go-grpc-gateway+stripe": {
		"--backend", "go-grpc-gateway",
		"--payments", "stripe",
	},
}

var commonArgs = []string{
	"--module", "github.com/example/demo-api",
	"--domain", "demo.example.com",
	"--admin-email", "admin@example.com",
	"--yes",
}

// planScenario computes a scenario's plan exactly as `kiln plan` would.
func planScenario(t *testing.T, name string) *generate.Plan {
	t.Helper()
	cmd := &cobra.Command{}
	sel := addSelectionFlags(cmd)
	if err := cmd.ParseFlags(append(append([]string{}, scenarios[name]...), commonArgs...)); err != nil {
		t.Fatalf("%s: parsing flags: %v", name, err)
	}
	cmd.SetOut(&bytes.Buffer{})
	plan, err := planFromOptions(cmd, sel.options(cmd, "demo"))
	if err != nil {
		t.Fatalf("%s: planning: %v", name, err)
	}
	return plan
}

// TestApplyReproducesPlan: a generated project's own kiln.yaml must
// regenerate it byte for byte. That's what makes kiln.yaml a lockfile
// rather than a log, and it catches any nondeterminism in generation.
func TestApplyReproducesPlan(t *testing.T) {
	for name := range scenarios {
		t.Run(name, func(t *testing.T) {
			original := planScenario(t, name)

			path := filepath.Join(t.TempDir(), manifest.FileName)
			if err := os.WriteFile(path, original.Result.Files[manifest.FileName].Content, 0o644); err != nil {
				t.Fatal(err)
			}
			replayed, err := planFromFile(path)
			if err != nil {
				t.Fatalf("planFromFile: %v", err)
			}

			if len(replayed.Result.Files) != len(original.Result.Files) {
				t.Errorf("replay produced %d files, original %d", len(replayed.Result.Files), len(original.Result.Files))
			}
			for p, f := range original.Result.Files {
				if p == manifest.FileName {
					continue // differs only in generated_at
				}
				got, ok := replayed.Result.Files[p]
				if !ok {
					t.Errorf("replay is missing %s", p)
					continue
				}
				if !bytes.Equal(got.Content, f.Content) || got.Executable != f.Executable {
					t.Errorf("replay changed %s", p)
				}
			}
			for p, hash := range original.Manifest.Files {
				if got := generate.HashContent(replayed.Result.Files[p].Content); got != hash {
					t.Errorf("%s: kiln.yaml records %s, replay hashes to %s", p, hash, got)
				}
			}
		})
	}
}

func TestApplyRefusesChangedPlugin(t *testing.T) {
	original := planScenario(t, "go-api")
	m := original.Manifest
	m.Plugins[0].Digest = "sha256:0000"

	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), manifest.FileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = planFromFile(path)
	if err == nil || !strings.Contains(err.Error(), "has changed since this plan was made") {
		t.Fatalf("err = %v, want a changed-plugin error", err)
	}
}
