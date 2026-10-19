package cli

import (
	"bytes"
	"crypto/sha256"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/manifest"
	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/internal/scaffold"
	"github.com/breadface/kiln/plugins"
)

// TestNewEndToEnd drives the real `kiln new` command against the actual
// embedded plugins (not fakes) and checks the result the way a person
// would: does it build, is the compose file well-formed, did the
// executable bit survive. It shells out to `go build`, which downloads the
// plugin's pinned modules over the network.
func TestNewEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("go build downloads the pinned modules over the network; skipped with -short")
	}

	dir := t.TempDir()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{
		"new", "e2eapp",
		"--backend", "go-api",
		"--module", "github.com/example/e2eapp-api",
		"--domain", "e2eapp.example.com",
		"--admin-email", "admin@example.com",
		"--dir", dir,
		"--yes",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("kiln new: %v\noutput:\n%s", err, buf.String())
	}

	projectDir := filepath.Join(dir, "e2eapp")

	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if scaffold.HasUnresolved(content) {
			t.Errorf("%s still contains an unresolved {{PLACEHOLDER}}", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking generated project: %v", err)
	}

	compose, err := os.ReadFile(filepath.Join(projectDir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading docker-compose.yml: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(compose, &doc); err != nil {
		t.Fatalf("docker-compose.yml is not valid YAML: %v", err)
	}
	services, _ := doc["services"].(map[string]any)
	for _, want := range []string{"e2eapp-api", "caddy"} {
		if _, ok := services[want]; !ok {
			t.Errorf("docker-compose.yml missing services.%s", want)
		}
	}

	apiDir := filepath.Join(projectDir, "e2eapp-api")
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = apiDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated module failed to build: %v\n%s", err, out)
	}

	info, err := os.Stat(filepath.Join(projectDir, "deploy.sh"))
	if err != nil {
		t.Fatalf("stat deploy.sh: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("deploy.sh should be executable after generation")
	}
}

// TestNewEndToEndGRPCGateway is TestNewEndToEnd's counterpart for
// backend/go-grpc-gateway: proto/buf.yaml/buf.gen.yaml land at the project
// root (not inside the app directory — see plugins/README.md), the
// buf.gen.yaml marker got replaced with this plugin's own Go/grpc/gateway
// plugin entries, and the generated module builds using the pre-generated
// stubs committed into the plugin (no buf/network access needed here).
func TestNewEndToEndGRPCGateway(t *testing.T) {
	if testing.Short() {
		t.Skip("go build downloads the pinned modules over the network; skipped with -short")
	}

	dir := t.TempDir()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{
		"new", "e2eapp",
		"--backend", "go-grpc-gateway",
		"--module", "github.com/example/e2eapp-api",
		"--domain", "e2eapp.example.com",
		"--admin-email", "admin@example.com",
		"--dir", dir,
		"--yes",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("kiln new: %v\noutput:\n%s", err, buf.String())
	}

	projectDir := filepath.Join(dir, "e2eapp")

	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if scaffold.HasUnresolved(content) {
			t.Errorf("%s still contains an unresolved {{PLACEHOLDER}}", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking generated project: %v", err)
	}

	for _, want := range []string{"buf.yaml", "buf.gen.yaml", filepath.Join("proto", "greeter", "greeter.proto")} {
		if _, err := os.Stat(filepath.Join(projectDir, want)); err != nil {
			t.Errorf("expected root-level %s: %v", want, err)
		}
	}

	genYAML, err := os.ReadFile(filepath.Join(projectDir, "buf.gen.yaml"))
	if err != nil {
		t.Fatalf("reading buf.gen.yaml: %v", err)
	}
	if !strings.Contains(string(genYAML), "buf.build/protocolbuffers/go") {
		t.Error("buf.gen.yaml should contain go-grpc-gateway's fragment-contributed Go plugin entries")
	}
	if strings.Contains(string(genYAML), "kiln:insert") {
		t.Error("buf.gen.yaml still contains the unmerged fragment marker")
	}

	compose, err := os.ReadFile(filepath.Join(projectDir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading docker-compose.yml: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(compose, &doc); err != nil {
		t.Fatalf("docker-compose.yml is not valid YAML: %v", err)
	}
	services, _ := doc["services"].(map[string]any)
	for _, want := range []string{"e2eapp-api", "caddy"} {
		if _, ok := services[want]; !ok {
			t.Errorf("docker-compose.yml missing services.%s", want)
		}
	}

	apiDir := filepath.Join(projectDir, "e2eapp-api")
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = apiDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated module failed to build: %v\n%s", err, out)
	}
}

// TestNewEndToEndMobile is TestNewEndToEnd's mobile-plugin counterpart. It
// doesn't shell out to any Kotlin/Gradle/Xcode toolchain (that's a manual,
// best-effort check — see CONTRIBUTING.md) — it verifies the structural
// things a Go test can catch cheaply: no leftover placeholders across 40+
// files, the Kotlin package directory was actually built from
// MOBILE_PACKAGE_PATH (not just substituted in file contents), the
// executable bit survived, and the embedded gradle-wrapper.jar was copied
// out byte-for-byte (see plugins/mobile/kmp-compose/plugin.yaml).
func TestNewEndToEndMobile(t *testing.T) {
	dir := t.TempDir()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{
		"new", "e2eapp",
		"--backend", "go-api",
		"--module", "github.com/example/e2eapp-api",
		"--domain", "e2eapp.example.com",
		"--admin-email", "admin@example.com",
		"--mobile", "kmp-compose",
		"--mobile-package", "com.example.e2eapp",
		"--dir", dir,
		"--yes",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("kiln new: %v\noutput:\n%s", err, buf.String())
	}

	mobileDir := filepath.Join(dir, "e2eapp", "e2eapp-mobile")

	err := filepath.Walk(mobileDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if scaffold.HasUnresolved(content) {
			t.Errorf("%s still contains an unresolved {{PLACEHOLDER}}", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking generated mobile project: %v", err)
	}

	appKt := filepath.Join(mobileDir, "composeApp", "src", "commonMain", "kotlin", "com", "example", "e2eapp", "App.kt")
	if _, err := os.Stat(appKt); err != nil {
		t.Errorf("App.kt not found at the MOBILE_PACKAGE_PATH-derived path %s: %v", appKt, err)
	}

	gradlew := filepath.Join(mobileDir, "gradlew")
	info, err := os.Stat(gradlew)
	if err != nil {
		t.Fatalf("stat gradlew: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("gradlew should be executable after generation")
	}

	gotJar, err := os.ReadFile(filepath.Join(mobileDir, "gradle", "wrapper", "gradle-wrapper.jar"))
	if err != nil {
		t.Fatalf("reading generated gradle-wrapper.jar: %v", err)
	}
	wantJar, err := fs.ReadFile(plugins.FS, "mobile/kmp-compose/files/{{APP_NAME}}-mobile/gradle/wrapper/gradle-wrapper.jar")
	if err != nil {
		t.Fatalf("reading source gradle-wrapper.jar from the embedded plugin: %v", err)
	}
	gotSum, wantSum := sha256.Sum256(gotJar), sha256.Sum256(wantJar)
	if gotSum != wantSum {
		t.Error("generated gradle-wrapper.jar is not byte-identical to the plugin's source jar")
	}
}

// TestNewWithoutMobileFlagHasNoMobileDir guards the fix in
// internal/generate.SelectPlugins: mobile must never be auto-selected, so a
// plain `kiln new` (no --mobile) must not produce a *-mobile directory at
// all, even though exactly one mobile plugin exists.
func TestNewWithoutMobileFlagHasNoMobileDir(t *testing.T) {
	dir := t.TempDir()
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{
		"new", "e2eapp",
		"--backend", "go-api",
		"--module", "github.com/example/e2eapp-api",
		"--domain", "e2eapp.example.com",
		"--admin-email", "admin@example.com",
		"--dir", dir,
		"--yes",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("kiln new: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "e2eapp", "e2eapp-mobile")); !os.IsNotExist(err) {
		t.Error("e2eapp-mobile should not exist when --mobile wasn't passed")
	}

	// Confirm the loader really does have exactly one mobile plugin
	// available — otherwise this test would pass for the wrong reason.
	available, err := plugin.NewEmbedLoader(plugins.FS).List(plugin.KindMobile)
	if err != nil {
		t.Fatalf("listing mobile plugins: %v", err)
	}
	if len(available) != 1 {
		t.Fatalf("expected exactly one mobile plugin to be available, got %d", len(available))
	}
}

// TestApplyEndToEnd drives `kiln new`, then `kiln apply` on the generated
// project's own kiln.yaml into a second directory, through the real
// commands, and checks that both projects on disk match the hashes
// kiln.yaml recorded exactly. That holds because no plugin rewrites files
// after generation (dependencies are pinned in the plugins rather than
// resolved by a `go mod tidy` hook), so kiln.yaml describes the disk.
func TestApplyEndToEnd(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	run := func(args ...string) {
		t.Helper()
		root := newRootCmd()
		buf := &bytes.Buffer{}
		root.SetOut(buf)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("kiln %v: %v\n%s", args, err, buf.String())
		}
	}

	run("new", "e2eapp",
		"--backend", "go-api",
		"--module", "github.com/example/e2eapp-api",
		"--domain", "e2eapp.example.com",
		"--admin-email", "admin@example.com",
		"--dir", first, "--yes")
	run("apply", filepath.Join(first, "e2eapp", "kiln.yaml"), "--dir", second)

	m, err := manifest.Read(filepath.Join(first, "e2eapp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) == 0 {
		t.Fatal("kiln.yaml records no file hashes")
	}
	for _, dir := range []string{first, second} {
		for path, hash := range m.Files {
			content, err := os.ReadFile(filepath.Join(dir, "e2eapp", filepath.FromSlash(path)))
			if err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			if got := generate.HashContent(content); got != hash {
				t.Errorf("%s on disk hashes to %s, kiln.yaml records %s", path, got, hash)
			}
		}
	}
}
