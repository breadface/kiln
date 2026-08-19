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

func TestResolveValuesDerivedNames(t *testing.T) {
	values, err := ResolveValues(nil, Request{}, "demoapp")
	if err != nil {
		t.Fatalf("ResolveValues: %v", err)
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
	if values["GHCR_OWNER"] != "you" {
		t.Errorf("GHCR_OWNER = %q, want derived from MODULE_PATH's org segment", values["GHCR_OWNER"])
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

func TestSelectPluginsAutoSelectsOnlyBackendAndHosting(t *testing.T) {
	loader := &fakeLoader{plugins: []*plugin.Plugin{
		{Manifest: plugin.Manifest{Name: "go-api", Kind: plugin.KindBackend}},
		{Manifest: plugin.Manifest{Name: "docker-swarm-hetzner", Kind: plugin.KindHosting}},
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
	for _, want := range []plugin.Kind{plugin.KindBackend, plugin.KindHosting} {
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

	selected, err := SelectPlugins(loader, Request{Mobile: "kmp-compose"})
	if err != nil {
		t.Fatalf("SelectPlugins: %v", err)
	}
	if len(selected) != 1 || selected[0].Manifest.Name != "kmp-compose" {
		t.Errorf("selected = %+v, want just kmp-compose", selected)
	}
}

func TestResolveValuesMobilePackagePath(t *testing.T) {
	selected := []*plugin.Plugin{{
		Manifest: plugin.Manifest{
			Name: "kmp-compose", Kind: plugin.KindMobile,
			Placeholders: []plugin.Placeholder{{Name: "MOBILE_PACKAGE", Required: true}},
		},
	}}

	values, err := ResolveValues(selected, Request{MobilePackage: "com.example.demoapp"}, "demoapp")
	if err != nil {
		t.Fatalf("ResolveValues: %v", err)
	}
	if values["MOBILE_PACKAGE_PATH"] != "com/example/demoapp" {
		t.Errorf("MOBILE_PACKAGE_PATH = %q, want %q", values["MOBILE_PACKAGE_PATH"], "com/example/demoapp")
	}
}

func TestResolveValuesMobilePackagePathAbsentWithoutMobile(t *testing.T) {
	values, err := ResolveValues(nil, Request{}, "demoapp")
	if err != nil {
		t.Fatalf("ResolveValues: %v", err)
	}
	if _, ok := values["MOBILE_PACKAGE_PATH"]; ok {
		t.Error("MOBILE_PACKAGE_PATH should not be present for a project without the mobile plugin selected")
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
