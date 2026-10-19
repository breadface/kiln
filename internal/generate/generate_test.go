package generate

import (
	"errors"
	"testing"

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
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveValuesOnlyAppNameIsBuiltIn(t *testing.T) {
	values, err := ResolveValues(nil, Request{}, "demoapp")
	if err != nil {
		t.Fatalf("ResolveValues: %v", err)
	}
	if len(values) != 1 || values["APP_NAME"] != "demoapp" {
		t.Errorf("values = %v, want only APP_NAME — every other value belongs to a plugin", values)
	}
}

// TestResolveValuesDefaultsAndDerive covers the manifest-driven value
// rules that replaced Kiln's old hardcoded ones: a default referencing
// another placeholder, a regexp derive, a replace derive, and a chain
// that only resolves across multiple passes (declared out of order).
func TestResolveValuesDefaultsAndDerive(t *testing.T) {
	selected := []*plugin.Plugin{
		{Manifest: plugin.Manifest{Name: "registry", Kind: plugin.KindRegistry, Placeholders: []plugin.Placeholder{
			{Name: "IMAGE", Default: "{{OWNER}}/{{SERVICE_NAME}}"},
			{Name: "OWNER", Required: true, Derive: &plugin.Derive{
				From: "MODULE_PATH", Match: `^github\.com/([^/]+)/`, Template: "$1",
			}},
		}}},
		{Manifest: plugin.Manifest{Name: "api", Kind: plugin.KindBackend, Placeholders: []plugin.Placeholder{
			{Name: "SERVICE_NAME", Default: "{{APP_NAME}}-api"},
			{Name: "MODULE_PATH", Required: true},
		}}},
		{Manifest: plugin.Manifest{Name: "mobile", Kind: plugin.KindMobile, Placeholders: []plugin.Placeholder{
			{Name: "PKG", Required: true},
			{Name: "PKG_PATH", Derive: &plugin.Derive{From: "PKG", Replace: map[string]string{".": "/"}}},
		}}},
	}

	values, err := ResolveValues(selected, Request{Set: map[string]string{
		"MODULE_PATH": "github.com/you/demoapp-api",
		"PKG":         "com.example.demoapp",
	}}, "demoapp")
	if err != nil {
		t.Fatalf("ResolveValues: %v", err)
	}
	want := map[string]string{
		"SERVICE_NAME": "demoapp-api",
		"OWNER":        "you",
		"IMAGE":        "you/demoapp-api",
		"PKG_PATH":     "com/example/demoapp",
	}
	for k, v := range want {
		if values[k] != v {
			t.Errorf("values[%q] = %q, want %q", k, values[k], v)
		}
	}

	// A derive that doesn't match leaves a required value missing rather
	// than inventing one.
	_, err = ResolveValues(selected, Request{Set: map[string]string{
		"MODULE_PATH": "gitlab.com/you/demoapp-api",
		"PKG":         "com.example.demoapp",
	}}, "demoapp")
	var missingErr *MissingValuesError
	if !errors.As(err, &missingErr) || len(missingErr.Missing) != 1 || missingErr.Missing[0].Name != "OWNER" {
		t.Errorf("err = %v, want OWNER reported missing", err)
	}

	// An explicit value always beats a derive or default.
	values, err = ResolveValues(selected, Request{Set: map[string]string{
		"MODULE_PATH": "github.com/you/demoapp-api", "PKG": "a.b", "OWNER": "org", "SERVICE_NAME": "core",
	}}, "demoapp")
	if err != nil {
		t.Fatalf("ResolveValues: %v", err)
	}
	if values["IMAGE"] != "org/core" {
		t.Errorf("IMAGE = %q, want explicit values to win", values["IMAGE"])
	}
}

func TestResolveValuesMissingRequired(t *testing.T) {
	selected := []*plugin.Plugin{{
		Manifest: plugin.Manifest{
			Name: "go-api", Kind: plugin.KindBackend,
			Placeholders: []plugin.Placeholder{
				{Name: "MODULE_PATH", Required: true},
			},
		},
	}}

	_, err := ResolveValues(selected, Request{}, "demoapp")
	if err == nil {
		t.Fatal("expected an error for a missing required placeholder, got nil")
	}
	var missingErr *MissingValuesError
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected a *MissingValuesError, got %T: %v", err, err)
	}
	if len(missingErr.Missing) != 1 || missingErr.Missing[0].Name != "MODULE_PATH" {
		t.Errorf("Missing = %+v, want just MODULE_PATH", missingErr.Missing)
	}
}

func TestResolveValuesSetOverridesFillMissing(t *testing.T) {
	selected := []*plugin.Plugin{{
		Manifest: plugin.Manifest{
			Name: "go-api", Kind: plugin.KindBackend,
			Placeholders: []plugin.Placeholder{
				{Name: "MODULE_PATH", Required: true},
			},
		},
	}}

	values, err := ResolveValues(selected, Request{Set: map[string]string{"MODULE_PATH": "github.com/you/demoapp-api"}}, "demoapp")
	if err != nil {
		t.Fatalf("ResolveValues: %v", err)
	}
	if values["MODULE_PATH"] != "github.com/you/demoapp-api" {
		t.Errorf("MODULE_PATH = %q, want the --set override", values["MODULE_PATH"])
	}
}

// fakeLoader is a minimal in-memory plugin.Loader for testing SelectPlugins
// without needing real embedded plugin content.
type fakeLoader struct {
	plugins []*plugin.Plugin
}

func (l *fakeLoader) Resolve(kind plugin.Kind, name string) (*plugin.Plugin, error) {
	for _, p := range l.plugins {
		if p.Manifest.Kind == kind && p.Manifest.Name == name {
			return p, nil
		}
	}
	return nil, errors.New("not found")
}

func (l *fakeLoader) List(kind plugin.Kind) ([]*plugin.Plugin, error) {
	var out []*plugin.Plugin
	for _, p := range l.plugins {
		if p.Manifest.Kind == kind {
			out = append(out, p)
		}
	}
	return out, nil
}

func TestSelectPluginsAutoSelectsBackendAndInfrastructure(t *testing.T) {
	loader := &fakeLoader{plugins: []*plugin.Plugin{
		{Manifest: plugin.Manifest{Name: "go-api", Kind: plugin.KindBackend}},
		{Manifest: plugin.Manifest{Name: "docker-swarm", Kind: plugin.KindRuntime}},
		{Manifest: plugin.Manifest{Name: "kmp-compose", Kind: plugin.KindMobile}},
	}}

	selected, err := SelectPlugins(loader, Request{})
	if err != nil {
		t.Fatalf("SelectPlugins: %v", err)
	}

	var gotKinds []plugin.Kind
	for _, p := range selected {
		gotKinds = append(gotKinds, p.Manifest.Kind)
	}
	for _, want := range []plugin.Kind{plugin.KindBackend, plugin.KindRuntime} {
		found := false
		for _, k := range gotKinds {
			if k == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %s to be auto-selected, got kinds %v", want, gotKinds)
		}
	}
	for _, k := range gotKinds {
		if k == plugin.KindMobile {
			t.Errorf("mobile should never be auto-selected even with exactly one plugin available, got kinds %v", gotKinds)
		}
	}
}

func TestSelectPluginsExplicitMobileOptIn(t *testing.T) {
	loader := &fakeLoader{plugins: []*plugin.Plugin{
		{Manifest: plugin.Manifest{Name: "kmp-compose", Kind: plugin.KindMobile}},
	}}

	selected, err := SelectPlugins(loader, Request{Plugins: map[plugin.Kind]string{plugin.KindMobile: "kmp-compose"}})
	if err != nil {
		t.Fatalf("SelectPlugins: %v", err)
	}
	if len(selected) != 1 || selected[0].Manifest.Name != "kmp-compose" {
		t.Errorf("selected = %+v, want just kmp-compose", selected)
	}
}

func TestSelectPluginsRejectsUnknownKind(t *testing.T) {
	if _, err := SelectPlugins(&fakeLoader{}, Request{Plugins: map[plugin.Kind]string{"database": "postgres"}}); err == nil {
		t.Fatal("expected an error selecting a plugin of an unknown kind")
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

	if err := ValidateRequiresConflicts([]*plugin.Plugin{backend, needsBackend}); err != nil {
		t.Errorf("expected requires to be satisfied, got: %v", err)
	}
	if err := ValidateRequiresConflicts([]*plugin.Plugin{needsBackend}); err == nil {
		t.Error("expected a requires error with no backend selected, got nil")
	}
	if err := ValidateRequiresConflicts([]*plugin.Plugin{backend, conflictsWithGoAPI}); err == nil {
		t.Error("expected a conflicts error, got nil")
	}
}
