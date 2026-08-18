package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/breadface/kiln/internal/scaffold"
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
