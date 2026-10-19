package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/breadface/kiln/internal/plugin"
)

func fakeBackendPlugin() *plugin.Plugin {
	return &plugin.Plugin{
		Manifest: plugin.Manifest{
			Name: "go-api", Kind: plugin.KindBackend,
			ContributesGoModule: true,
			Executable:          []string{"deploy-helper.sh"},
			PostCopy:            "hooks/post_copy.sh",
		},
		FS: fstest.MapFS{
			"files/main.go":          &fstest.MapFile{Data: []byte("// {{APP_NAME}} entrypoint\n")},
			"files/deploy-helper.sh": &fstest.MapFile{Data: []byte("#!/bin/sh\necho ok\n")},
			"files/go.mod.tmpl":      &fstest.MapFile{Data: []byte("module {{APP_NAME}}\n")},
			"files/dot__env.example": &fstest.MapFile{Data: []byte("PORT={{PORT}}\n")},
			"fragments/docker-compose.fragment.yml": &fstest.MapFile{
				Data: []byte("services:\n  api:\n    image: {{APP_NAME}}-api\n"),
			},
			"hooks/post_copy.sh": &fstest.MapFile{
				Data: []byte("#!/bin/sh\necho \"domain=$KILN_DOMAIN\" > post_copy_ran.txt\n"),
			},
		},
	}
}

func fakeHostingPlugin() *plugin.Plugin {
	return &plugin.Plugin{
		Manifest: plugin.Manifest{Name: "docker-swarm-hetzner", Kind: plugin.KindHosting},
		FS: fstest.MapFS{
			"files/Caddyfile": &fstest.MapFile{
				Data: []byte("{\n\temail {{ADMIN_EMAIL}}\n}\n\n# kiln:insert\n"),
			},
			"fragments/docker-compose.fragment.yml": &fstest.MapFile{
				Data: []byte("services:\n  caddy:\n    image: caddy:2-alpine\n"),
			},
			"fragments/Caddyfile.fragment": &fstest.MapFile{
				Data: []byte("{{DOMAIN}} {\n\treverse_proxy api:{{PORT}}\n}\n"),
			},
		},
	}
}

func fakeGoModulePlugin(kind plugin.Kind, name, goWorkEntry string) *plugin.Plugin {
	return &plugin.Plugin{
		Manifest: plugin.Manifest{Name: name, Kind: kind, ContributesGoModule: true},
		FS: fstest.MapFS{
			"files/go.mod":               &fstest.MapFile{Data: []byte("module fake\n")},
			"fragments/go.work.fragment": &fstest.MapFile{Data: []byte(goWorkEntry)},
		},
	}
}

func TestTransformPath(t *testing.T) {
	values := Values{"APP_NAME": "demoapp"}

	cases := []struct{ in, want string }{
		{"main.go", "main.go"},
		{"go.mod.tmpl", "go.mod"},
		{"dot__env.example", ".env.example"},
		{"dot__github/workflows/deploy.yml.tmpl", ".github/workflows/deploy.yml"},
		{"{{APP_NAME}}-api/main.go", "demoapp-api/main.go"},
	}
	for _, tc := range cases {
		if got := transformPath(tc.in, values); got != tc.want {
			t.Errorf("transformPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGenerateEndToEnd(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "demoapp")

	values := Values{"APP_NAME": "demoapp", "ADMIN_EMAIL": "hello@example.com", "DOMAIN": "demo.example.com", "PORT": "8080", "DOMAIN_KEY": "unused"}

	result, err := Generate(Options{
		Plugins: []*plugin.Plugin{fakeBackendPlugin(), fakeHostingPlugin()},
		Values:  values,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := WriteToDisk(result, out); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}

	// Files copied and rendered.
	main, err := os.ReadFile(filepath.Join(out, "main.go"))
	if err != nil {
		t.Fatalf("reading generated main.go: %v", err)
	}
	if string(main) != "// demoapp entrypoint\n" {
		t.Errorf("main.go = %q, placeholder not substituted", main)
	}

	// Executable bit restored.
	info, err := os.Stat(filepath.Join(out, "deploy-helper.sh"))
	if err != nil {
		t.Fatalf("stat deploy-helper.sh: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("deploy-helper.sh should be executable after generation")
	}

	// "go.mod.tmpl" -> "go.mod" and "dot__env.example" -> ".env.example":
	// escapes for content go:embed can't hold directly (see transformPath).
	goMod, err := os.ReadFile(filepath.Join(out, "go.mod"))
	if err != nil {
		t.Fatalf("go.mod.tmpl should have been written out as go.mod: %v", err)
	}
	if string(goMod) != "module demoapp\n" {
		t.Errorf("go.mod = %q, placeholder not substituted", goMod)
	}
	if _, err := os.ReadFile(filepath.Join(out, ".env.example")); err != nil {
		t.Fatalf("dot__env.example should have been written out as .env.example: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "go.mod.tmpl")); !os.IsNotExist(err) {
		t.Error("go.mod.tmpl should not exist verbatim in the generated output")
	}

	// Compose fragments merged from both plugins.
	compose, err := os.ReadFile(filepath.Join(out, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading docker-compose.yml: %v", err)
	}
	if !strings.Contains(string(compose), "demoapp-api") || !strings.Contains(string(compose), "caddy:2-alpine") {
		t.Errorf("docker-compose.yml missing merged content:\n%s", compose)
	}

	// Caddyfile text-fragment merge.
	caddy, err := os.ReadFile(filepath.Join(out, "Caddyfile"))
	if err != nil {
		t.Fatalf("reading Caddyfile: %v", err)
	}
	if !strings.Contains(string(caddy), "demo.example.com") || !strings.Contains(string(caddy), "reverse_proxy api:8080") {
		t.Errorf("Caddyfile missing merged fragment:\n%s", caddy)
	}
	if strings.Contains(string(caddy), "kiln:insert") {
		t.Error("Caddyfile marker should have been replaced")
	}

	// No go.work with only one Go-module plugin selected.
	if _, err := os.Stat(filepath.Join(out, "go.work")); !os.IsNotExist(err) {
		t.Error("go.work should not be generated with a single Go-module plugin")
	}

	// Generate itself never runs hooks — see TestRunHooksPostCopy. Confirm
	// that directly here too, since it's easy to regress silently.
	if _, err := os.Stat(filepath.Join(out, "post_copy_ran.txt")); !os.IsNotExist(err) {
		t.Error("Generate should not run post_copy hooks; that's RunHooks' job")
	}
}

func TestRunHooksPostCopy(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "demoapp")

	values := Values{"APP_NAME": "demoapp", "ADMIN_EMAIL": "hello@example.com", "DOMAIN": "demo.example.com", "PORT": "8080"}
	plugins := []*plugin.Plugin{fakeBackendPlugin(), fakeHostingPlugin()}

	result, err := Generate(Options{Plugins: plugins, Values: values})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := WriteToDisk(result, out); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}
	if err := RunHooks(plugins, out, values); err != nil {
		t.Fatalf("RunHooks: %v", err)
	}

	// post_copy hook ran with the resolved values available as KILN_ env vars.
	ran, err := os.ReadFile(filepath.Join(out, "post_copy_ran.txt"))
	if err != nil {
		t.Fatalf("post_copy hook did not run: %v", err)
	}
	if strings.TrimSpace(string(ran)) != "domain=demo.example.com" {
		t.Errorf("post_copy hook saw wrong KILN_DOMAIN: %q", ran)
	}
}

func TestGenerateGoWorkNeedsTwoModules(t *testing.T) {
	result, err := Generate(Options{
		Plugins: []*plugin.Plugin{
			fakeGoModulePlugin(plugin.KindBackend, "go-api", "./demoapp-api"),
			fakeGoModulePlugin(plugin.KindPayments, "flutterwave", "./demoapp-payments-flutterwave"),
		},
		Values: Values{},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	goWork, ok := result.Files["go.work"]
	if !ok {
		t.Fatal("go.work should be generated with two Go-module plugins")
	}
	if !strings.Contains(string(goWork.Content), "./demoapp-api") || !strings.Contains(string(goWork.Content), "./demoapp-payments-flutterwave") {
		t.Errorf("go.work missing both module entries:\n%s", goWork.Content)
	}
}

// TestGenerateOptionalFragmentTargetSkippedWhenNoBase covers a text
// fragment aimed at a base file (e.g. buf.gen.yaml) that no selected
// plugin shipped — unlike Caddyfile, which always has an owner in
// practice, a target like this is genuinely optional (only present when a
// proto-aware backend is also selected) and must be silently skipped
// rather than treated as an error.
func TestGenerateOptionalFragmentTargetSkippedWhenNoBase(t *testing.T) {
	orphan := &plugin.Plugin{
		Manifest: plugin.Manifest{Name: "kmp-compose", Kind: plugin.KindMobile},
		FS: fstest.MapFS{
			"fragments/buf.gen.yaml.fragment": &fstest.MapFile{
				Data: []byte("  - remote: buf.build/community/timostamm-protobuf-ts\n"),
			},
		},
	}

	result, err := Generate(Options{
		Plugins: []*plugin.Plugin{fakeBackendPlugin(), orphan},
		Values:  Values{"APP_NAME": "demoapp"},
	})
	if err != nil {
		t.Fatalf("Generate: %v (an unmatched optional fragment should be skipped, not error)", err)
	}
	if _, ok := result.Files["buf.gen.yaml"]; ok {
		t.Error("buf.gen.yaml should not be synthesized when no plugin shipped it as a base file")
	}
}

func TestGenerateComposeConflict(t *testing.T) {
	conflicting := &plugin.Plugin{
		Manifest: plugin.Manifest{Name: "conflicting", Kind: plugin.KindGlue},
		FS: fstest.MapFS{
			"fragments/docker-compose.fragment.yml": &fstest.MapFile{
				Data: []byte("services:\n  api:\n    image: some-other-image\n"),
			},
		},
	}

	_, err := Generate(Options{
		Plugins: []*plugin.Plugin{fakeBackendPlugin(), conflicting},
		Values:  Values{"APP_NAME": "demoapp"},
	})
	if err == nil {
		t.Fatal("expected a merge conflict error, got nil")
	}
}

// TestGenerateBindingFragments covers per-binding fragments: one render per
// bound provider, with the provider's contract fields layered over the
// global values, into a named slot of a nested, escaped base path.
func TestGenerateBindingFragments(t *testing.T) {
	host := &plugin.Plugin{
		Manifest: plugin.Manifest{Name: "host", Kind: plugin.KindHosting},
		FS: fstest.MapFS{
			"files/dot__ci/deploy.yml": &fstest.MapFile{Data: []byte("steps:\n  # kiln:insert build\n  - run: deploy {{APP_NAME}}\n")},
			"bindings/http-service/dot__ci/deploy.yml@build.fragment": &fstest.MapFile{
				Data: []byte("- run: build {{SERVICE_NAME}} for {{APP_NAME}}\n"),
			},
		},
	}
	api := &plugin.Plugin{Manifest: plugin.Manifest{Name: "api", Kind: plugin.KindBackend}, FS: fstest.MapFS{}}
	worker := &plugin.Plugin{Manifest: plugin.Manifest{Name: "worker", Kind: plugin.KindGlue}, FS: fstest.MapFS{}}

	result, err := Generate(Options{
		Plugins: []*plugin.Plugin{api, worker, host},
		Bindings: []plugin.Binding{
			{Contract: "http-service@1", Consumer: host, Provider: api, Values: map[string]string{"SERVICE_NAME": "demo-api"}},
			{Contract: "http-service@1", Consumer: host, Provider: worker, Values: map[string]string{"SERVICE_NAME": "demo-worker"}},
		},
		Values: Values{"APP_NAME": "demo"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	want := "steps:\n" +
		"  # --- hosting/host for backend/api (http-service@1) ---\n" +
		"  - run: build demo-api for demo\n" +
		"  # --- hosting/host for glue/worker (http-service@1) ---\n" +
		"  - run: build demo-worker for demo\n" +
		"  - run: deploy demo\n"
	if got := string(result.Files[".ci/deploy.yml"].Content); got != want {
		t.Errorf(".ci/deploy.yml =\n%s\nwant\n%s", got, want)
	}
}
