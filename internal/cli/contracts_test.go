package cli

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/breadface/kiln/contracts"
	"github.com/breadface/kiln/internal/contract"
	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/internal/scaffold"
	"github.com/breadface/kiln/plugins"
)

// TestHostingDeploysAnyProvider is the point of contracts: a plugin the
// real hosting plugin has never heard of — here a stand-in for a payments
// plugin that ships its own webhook service and API key — gets built,
// deployed, routed, and its secret wired up, with zero changes to the
// hosting plugin.
func TestHostingDeploysAnyProvider(t *testing.T) {
	loader := plugin.NewEmbedLoader(plugins.FS)
	backend, err := loader.Resolve(plugin.KindBackend, "go-api")
	if err != nil {
		t.Fatal(err)
	}
	hosting, err := loader.Resolve(plugin.KindHosting, "docker-swarm-hetzner")
	if err != nil {
		t.Fatal(err)
	}
	payments := &plugin.Plugin{
		Manifest: plugin.Manifest{
			Name: "stripe", Kind: plugin.KindPayments,
			Provides: []plugin.Provide{
				{Contract: "http-service@1", With: map[string]string{
					"SERVICE_NAME": "{{APP_NAME}}-payments", "PORT": "4242",
					"BUILD_CONTEXT": "{{APP_NAME}}-payments", "PATH_PREFIX": "/webhooks/stripe/",
				}},
				{Contract: "env@1", With: map[string]string{"ENV_NAME": "STRIPE_SECRET_KEY"}},
			},
		},
		FS: fstest.MapFS{},
	}
	selected := []*plugin.Plugin{backend, payments, hosting}

	registry, err := contract.Load(contracts.FS)
	if err != nil {
		t.Fatal(err)
	}
	values, err := generate.ResolveValues(selected, generate.Request{Set: map[string]string{
		"MODULE_PATH": "github.com/you/demo-api", "DOMAIN": "demo.example.com", "ADMIN_EMAIL": "a@b.co",
	}}, "demo")
	if err != nil {
		t.Fatalf("ResolveValues: %v", err)
	}
	bindings, values, err := generate.ResolveBindings(selected, registry, values)
	if err != nil {
		t.Fatalf("ResolveBindings: %v", err)
	}
	result, err := scaffold.Generate(scaffold.Options{Plugins: selected, Bindings: bindings, Values: values})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	file := func(name string) string { return string(result.Files[name].Content) }
	checks := map[string][]string{
		"Caddyfile": {
			"handle /* {", "reverse_proxy demo-api:8080",
			"handle /webhooks/stripe/* {", "reverse_proxy demo-payments:4242",
		},
		".github/workflows/deploy.yml": {
			"context: ./demo-api", "context: ./demo-payments",
			"STRIPE_SECRET_KEY=${{ secrets.STRIPE_SECRET_KEY }}",
			"DATABASE_URL=${{ secrets.DATABASE_URL }}",
			`service="demo_demo-payments"`,
		},
		"docker-compose.swarm.yml": {"demo-api:", "demo-payments:"},
	}
	for name, wants := range checks {
		for _, want := range wants {
			if !strings.Contains(file(name), want) {
				t.Errorf("%s missing %q:\n%s", name, want, file(name))
			}
		}
	}
	// payments provides no migration, so only the backend's runs.
	if n := strings.Count(file(".github/workflows/deploy.yml"), "=== Running migrations"); n != 1 {
		t.Errorf("deploy.yml runs %d migrations, want 1", n)
	}
}
