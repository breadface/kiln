package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/breadface/kiln/internal/plugin"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"demoapp":      "demoapp",
		"Demo App":     "demo-app",
		"demo_app":     "demo-app",
		"  Demo!App  ": "demoapp",
		"---":          "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGhcrOwnerFromModule(t *testing.T) {
	cases := map[string]string{
		"github.com/damilolao328/demoapp-api": "damilolao328",
		"gitlab.com/someone/demoapp-api":      "",
		"":                                    "",
	}
	for in, want := range cases {
		if got := ghcrOwnerFromModule(in); got != want {
			t.Errorf("ghcrOwnerFromModule(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCollectValuesDerivedNames(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})

	values, err := collectValues(cmd, nil, newOptions{Yes: true}, "demoapp")
	if err != nil {
		t.Fatalf("collectValues: %v", err)
	}
	want := map[string]string{
		"APP_NAME":           "demoapp",
		"SERVICE_NAME":       "demoapp-api",
		"SERVICE_NAME_UPPER": "DEMOAPP_API",
		"STACK_NAME":         "demoapp",
	}
	for k, v := range want {
		if values[k] != v {
			t.Errorf("values[%q] = %q, want %q", k, values[k], v)
		}
	}
}

func TestCollectValuesMissingRequired(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})

	selected := []*plugin.Plugin{{
		Manifest: plugin.Manifest{
			Name: "go-api", Kind: plugin.KindBackend,
			Placeholders: []plugin.Placeholder{
				{Name: "MODULE_PATH", Required: true},
			},
		},
	}}

	if _, err := collectValues(cmd, selected, newOptions{Yes: true}, "demoapp"); err == nil {
		t.Fatal("expected an error for a missing required placeholder with --yes, got nil")
	}
}

func TestValidateRequiresConflicts(t *testing.T) {
	backend := &plugin.Plugin{Manifest: plugin.Manifest{Name: "go-api", Kind: plugin.KindBackend}}
	needsBackend := &plugin.Plugin{Manifest: plugin.Manifest{
		Name: "flutterwave", Kind: plugin.KindPayments,
		Requires: []string{"backend"},
	}}
	conflictsWithGoAPI := &plugin.Plugin{Manifest: plugin.Manifest{
		Name: "other-backend", Kind: plugin.KindGlue,
		Conflicts: []string{"backend/go-api"},
	}}

	if err := validateRequiresConflicts([]*plugin.Plugin{backend, needsBackend}); err != nil {
		t.Errorf("expected requires to be satisfied, got: %v", err)
	}
	if err := validateRequiresConflicts([]*plugin.Plugin{needsBackend}); err == nil {
		t.Error("expected a requires error with no backend selected, got nil")
	}
	if err := validateRequiresConflicts([]*plugin.Plugin{backend, conflictsWithGoAPI}); err == nil {
		t.Error("expected a conflicts error, got nil")
	}
}
