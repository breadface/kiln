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

// TestInfrastructureShipsAnyProvider is the point of contracts: a plugin
// none of the infrastructure plugins has heard of — here a stand-in for a
// payments plugin that ships its own webhook service, image and API key —
// gets routed by the edge, built and pushed by CI, run and verified by the
// runtime, and its secret wired up, with zero changes to any of them.
func TestInfrastructureShipsAnyProvider(t *testing.T) {
	loader := plugin.NewEmbedLoader(plugins.FS)
	resolve := func(k plugin.Kind, name string) *plugin.Plugin {
		t.Helper()
		p, err := loader.Resolve(k, name)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	payments := &plugin.Plugin{
		Manifest: plugin.Manifest{
			Name: "stripe", Kind: plugin.KindPayments,
			Provides: []plugin.Provide{
				{Contract: "http-service@2", With: map[string]string{
					"SERVICE_NAME": "{{APP_NAME}}-payments", "PORT": "4242", "PATH_PREFIX": "/webhooks/stripe/",
				}},
				{Contract: "oci-image@1", With: map[string]string{
					"SERVICE_NAME": "{{APP_NAME}}-payments", "BUILD_CONTEXT": "{{APP_NAME}}-payments",
				}},
				{Contract: "env@1", With: map[string]string{"ENV_NAME": "STRIPE_SECRET_KEY", "ENV_DESCRIPTION": "Stripe API key"}},
			},
		},
		FS: fstest.MapFS{},
	}
	selected := []*plugin.Plugin{
		resolve(plugin.KindBackend, "go-api"),
		payments,
		resolve(plugin.KindEdge, "caddy"),
		resolve(plugin.KindRegistry, "ghcr"),
		resolve(plugin.KindRuntime, "docker-swarm"),
		resolve(plugin.KindCI, "github-actions"),
	}

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
		// edge: one route per service
		"Caddyfile": {
			"handle /* {", "reverse_proxy demo-api:8080",
			"handle /webhooks/stripe/* {", "reverse_proxy demo-payments:4242",
		},
		// ci: build and push every image, .env from every env@1
		".github/workflows/deploy.yml": {
			"context: ./demo-api", "context: ./demo-payments",
			"ghcr.io/you/demo-payments:${{ github.sha }}",
			"STRIPE_SECRET_KEY: ${{ secrets.STRIPE_SECRET_KEY }}",
			`printf '%s=%s\n' STRIPE_SECRET_KEY "$STRIPE_SECRET_KEY"`,
			"DEPLOY_HOST: ${{ secrets.DEPLOY_HOST }}",
			"run: ./deploy.sh",
		},
		".github/DEPLOY.md": {"`STRIPE_SECRET_KEY` — Stripe API key", "`DEPLOY_SSH_KEY`", "`GHCR_TOKEN`"},
		// runtime: run, pull and verify every image
		"docker-compose.swarm.yml": {
			"image: ghcr.io/you/demo-api:${IMAGE_TAG:-latest}",
			"image: ghcr.io/you/demo-payments:${IMAGE_TAG:-latest}",
			"name: demo_caddyfile_${IMAGE_TAG:-latest}",
		},
		"deploy.sh": {
			`docker pull "ghcr.io/you/demo-payments:$IMAGE_TAG"`,
			`service="demo_demo-payments"`,
		},
	}
	for name, wants := range checks {
		for _, want := range wants {
			if !strings.Contains(file(name), want) {
				t.Errorf("%s missing %q:\n%s", name, want, file(name))
			}
		}
	}
	// payments provides no migration, so only the backend's runs.
	if n := strings.Count(file("deploy.sh"), "=== Running migrations"); n != 1 {
		t.Errorf("deploy.sh runs %d migrations, want 1", n)
	}
}
