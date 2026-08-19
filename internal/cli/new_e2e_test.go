package cli

import (
	"bytes"
	"crypto/sha256"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/internal/scaffold"
	"github.com/breadface/kiln/plugins"
)

// TestNewEndToEnd drives the real `kiln new` command against the actual
// embedded plugins (not fakes) and checks the result the way a person
// would: does it build, is the compose file well-formed, did the
// executable bit survive. It shells out to `go build`, which runs `go mod
// tidy` over the network via the go-api plugin's post_copy hook.
func TestNewEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to go build/go mod tidy over the network; skipped with -short")
	}

	dir := t.TempDir()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{
		"new", "e2eapp",
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
