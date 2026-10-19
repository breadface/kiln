package generate

import (
	"strings"
	"testing"

	"github.com/breadface/kiln/contracts"
	"github.com/breadface/kiln/internal/contract"
	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/internal/scaffold"
)

func registry(t *testing.T) *contract.Registry {
	t.Helper()
	r, err := contract.Load(contracts.FS)
	if err != nil {
		t.Fatalf("loading contracts: %v", err)
	}
	return r
}

func service(name, port string) *plugin.Plugin {
	return &plugin.Plugin{Manifest: plugin.Manifest{
		Name: name, Kind: plugin.KindBackend,
		Provides: []plugin.Provide{
			{Contract: "http-service@1", With: map[string]string{
				"SERVICE_NAME": "{{APP_NAME}}-" + name, "PORT": port, "BUILD_CONTEXT": name,
			}},
			{Contract: "env@1", With: map[string]string{"ENV_NAME": strings.ToUpper(name) + "_KEY"}},
		},
	}}
}

func TestResolveBindingsBindsEveryProvider(t *testing.T) {
	host := &plugin.Plugin{Manifest: plugin.Manifest{
		Name: "host", Kind: plugin.KindHosting,
		Consumes: []plugin.Consume{{Contract: "http-service@1", Min: 1}, {Contract: "env@1"}},
	}}
	selected := []*plugin.Plugin{service("api", "8080"), service("worker", "9000"), host}

	bindings, _, err := ResolveBindings(selected, registry(t), scaffold.Values{"APP_NAME": "demo"})
	if err != nil {
		t.Fatalf("ResolveBindings: %v", err)
	}
	if len(bindings) != 4 {
		t.Fatalf("got %d bindings, want 2 http-service + 2 env", len(bindings))
	}
	b := bindings[0]
	if b.Contract != "http-service@1" || b.Consumer != host || b.Values["SERVICE_NAME"] != "demo-api" {
		t.Errorf("first binding = %+v, want host bound to demo-api", b)
	}
	if b.Values["PATH_PREFIX"] != "/" {
		t.Errorf("PATH_PREFIX = %q, want the contract default", b.Values["PATH_PREFIX"])
	}
}

func TestResolveBindingsCardinality(t *testing.T) {
	host := &plugin.Plugin{Manifest: plugin.Manifest{
		Name: "host", Kind: plugin.KindHosting,
		Consumes: []plugin.Consume{{Contract: "http-service@1", Min: 1}},
	}}
	if _, _, err := ResolveBindings([]*plugin.Plugin{host}, registry(t), scaffold.Values{}); err == nil {
		t.Error("expected an error when min: 1 has no provider")
	}

	single := &plugin.Plugin{Manifest: plugin.Manifest{
		Name: "single", Kind: plugin.KindMobile,
		Consumes: []plugin.Consume{{Contract: "http-service@1", Max: 1}},
	}}
	selected := []*plugin.Plugin{service("api", "8080"), service("worker", "9000"), single}
	if _, _, err := ResolveBindings(selected, registry(t), scaffold.Values{"APP_NAME": "demo"}); err == nil {
		t.Error("expected an error when max: 1 has two providers")
	}
}

func TestResolveBindingsExportsAs(t *testing.T) {
	mobile := &plugin.Plugin{Manifest: plugin.Manifest{
		Name: "mobile", Kind: plugin.KindMobile,
		Consumes: []plugin.Consume{{Contract: "http-service@1", Min: 1, Max: 1, As: "API"}},
	}}
	_, values, err := ResolveBindings([]*plugin.Plugin{service("api", "8080"), mobile}, registry(t), scaffold.Values{"APP_NAME": "demo"})
	if err != nil {
		t.Fatalf("ResolveBindings: %v", err)
	}
	if values["API_PORT"] != "8080" || values["API_SERVICE_NAME"] != "demo-api" {
		t.Errorf("exported values = %v, want API_PORT and API_SERVICE_NAME", values)
	}

	mobile.Manifest.Consumes[0].Max = 0
	if _, _, err := ResolveBindings([]*plugin.Plugin{service("api", "8080"), mobile}, registry(t), scaffold.Values{"APP_NAME": "demo"}); err == nil {
		t.Error("expected an error for `as` without max: 1")
	}
}

func TestResolveBindingsValidatesProvides(t *testing.T) {
	cases := map[string]plugin.Provide{
		"unknown contract":       {Contract: "queue@1"},
		"missing required field": {Contract: "env@1"},
		"unknown field":          {Contract: "env@1", With: map[string]string{"ENV_NAME": "X", "SECRET": "true"}},
		"unresolved value":       {Contract: "env@1", With: map[string]string{"ENV_NAME": "{{NOPE}}"}},
	}
	for name, pr := range cases {
		p := &plugin.Plugin{Manifest: plugin.Manifest{Name: "p", Kind: plugin.KindGlue, Provides: []plugin.Provide{pr}}}
		if _, _, err := ResolveBindings([]*plugin.Plugin{p}, registry(t), scaffold.Values{}); err == nil {
			t.Errorf("%s: expected an error even with no consumer", name)
		}
	}
}

func TestResolveBindingsLabelIsFirstField(t *testing.T) {
	host := &plugin.Plugin{Manifest: plugin.Manifest{
		Name: "host", Kind: plugin.KindHosting,
		Consumes: []plugin.Consume{{Contract: "env@1"}},
	}}
	bindings, _, err := ResolveBindings([]*plugin.Plugin{service("api", "8080"), host}, registry(t), scaffold.Values{"APP_NAME": "demo"})
	if err != nil {
		t.Fatalf("ResolveBindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].Label != "API_KEY" {
		t.Errorf("bindings = %+v, want one labelled API_KEY", bindings)
	}
}
