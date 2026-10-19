package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/plugin"
)

func TestRequestFromOptionsParsesSet(t *testing.T) {
	req, err := requestFromOptions(newOptions{Set: []string{"DOMAIN=demo.example.com", " PORT = 9090 "}})
	if err != nil {
		t.Fatalf("requestFromOptions: %v", err)
	}
	if req.Set["DOMAIN"] != "demo.example.com" || req.Set["PORT"] != "9090" {
		t.Errorf("Set = %+v, want trimmed KEY=VALUE pairs", req.Set)
	}
}

func TestRequestFromOptionsRejectsMalformedSet(t *testing.T) {
	if _, err := requestFromOptions(newOptions{Set: []string{"not-a-kv-pair"}}); err == nil {
		t.Fatal("expected an error for a --set value with no '=', got nil")
	}
}

func TestCollectValuesYesSurfacesMissing(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})

	selected := []*plugin.Plugin{{
		Manifest: plugin.Manifest{
			Name: "go-api", Kind: plugin.KindBackend,
			Placeholders: []plugin.Placeholder{{Name: "MODULE_PATH", Required: true}},
		},
	}}

	req := generate.Request{Set: map[string]string{}}
	if _, err := collectValues(cmd, selected, req, "demoapp", true); err == nil {
		t.Fatal("expected an error for a missing required placeholder with --yes, got nil")
	}
}

func TestCollectValuesPromptsForMissing(t *testing.T) {
	cmd := &cobra.Command{}
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetIn(strings.NewReader("github.com/you/demoapp-api\n"))

	selected := []*plugin.Plugin{{
		Manifest: plugin.Manifest{
			Name: "go-api", Kind: plugin.KindBackend,
			Placeholders: []plugin.Placeholder{
				{Name: "MODULE_PATH", Description: "Go module path", Required: true},
			},
		},
	}}

	req := generate.Request{Set: map[string]string{}}
	values, err := collectValues(cmd, selected, req, "demoapp", false)
	if err != nil {
		t.Fatalf("collectValues: %v", err)
	}
	if values["MODULE_PATH"] != "github.com/you/demoapp-api" {
		t.Errorf("MODULE_PATH = %q, want the piped stdin answer", values["MODULE_PATH"])
	}
	if !strings.Contains(out.String(), "MODULE_PATH") {
		t.Error("expected the prompt to mention the missing placeholder's name")
	}
}

func TestCollectValuesPromptLeftBlankStillMissing(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader("\n"))

	selected := []*plugin.Plugin{{
		Manifest: plugin.Manifest{
			Name: "go-api", Kind: plugin.KindBackend,
			Placeholders: []plugin.Placeholder{{Name: "MODULE_PATH", Required: true}},
		},
	}}

	req := generate.Request{Set: map[string]string{}}
	if _, err := collectValues(cmd, selected, req, "demoapp", false); err == nil {
		t.Fatal("expected an error when the interactive prompt is left blank, got nil")
	}
}

// TestNewFlagsComeFromPlugins guards that `kiln new` has no hardcoded
// plugin knowledge: one selection flag per Kind (including kinds with no
// plugins yet), and value flags only because a plugin declared them.
func TestNewFlagsComeFromPlugins(t *testing.T) {
	cmd := newNewCmd()
	for _, name := range []string{
		"backend", "frontend", "mobile", "payments", "edge", "registry", "runtime", "ci", "glue",
		"module", "domain", "admin-email", "ghcr-owner", "ghcr-username", "mobile-package",
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("kiln new is missing --%s", name)
		}
	}
	if usage := cmd.Flags().Lookup("module").Usage; !strings.Contains(usage, "MODULE_PATH") {
		t.Errorf("--module usage %q should name the placeholder it sets", usage)
	}
}
