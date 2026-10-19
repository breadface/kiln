package fragment

import (
	"strings"
	"testing"
)

func TestMergeTextBlocksCaddyfile(t *testing.T) {
	base := []byte("{\n\temail {{ADMIN_EMAIL}}\n}\n\n# kiln:insert\n")
	fragments := []NamedFragment{
		{PluginRef: "hosting/docker-swarm-hetzner", Content: "{{DOMAIN}} {\n\treverse_proxy {{SERVICE_NAME}}:{{PORT}}\n}\n"},
	}

	out, err := MergeTextBlocks(base, "", fragments)
	if err != nil {
		t.Fatalf("MergeTextBlocks: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "# --- hosting/docker-swarm-hetzner ---") {
		t.Error("missing provenance comment for the contributing plugin")
	}
	if !strings.Contains(s, "reverse_proxy {{SERVICE_NAME}}:{{PORT}}") {
		t.Error("fragment content was not inserted")
	}
	if strings.Contains(string(StripMarkers(out)), DefaultMarker) {
		t.Error("StripMarkers should remove the marker line once merging is done")
	}
}

func TestMergeTextBlocksMissingMarker(t *testing.T) {
	base := []byte("no marker in here\n")
	_, err := MergeTextBlocks(base, "", []NamedFragment{{PluginRef: "x", Content: "y"}})
	if err == nil {
		t.Fatal("expected an error when the base file has no marker line, got nil")
	}
}

func TestMergeTextBlocksOrdering(t *testing.T) {
	base := []byte("# kiln:insert\n")
	fragments := []NamedFragment{
		{PluginRef: "backend/go-api", Content: "./demoapp-api"},
		{PluginRef: "payments/go-api/flutterwave", Content: "./payments-flutterwave"},
	}

	out, err := MergeTextBlocks(base, "", fragments)
	if err != nil {
		t.Fatalf("MergeTextBlocks: %v", err)
	}
	s := string(out)
	backendIdx := strings.Index(s, "backend/go-api")
	paymentsIdx := strings.Index(s, "payments/go-api/flutterwave")
	if backendIdx == -1 || paymentsIdx == -1 {
		t.Fatalf("both fragments should appear in output, got: %s", s)
	}
	if backendIdx > paymentsIdx {
		t.Error("fragments should appear in the order they were passed in (caller controls Kind order)")
	}
}

// TestMergeTextBlocksGoWork exercises the same generic mechanism against a
// go.work-shaped base, standing in for what a future second Go-module
// plugin (e.g. payments) would contribute alongside backend/go-api. No
// go.work is emitted in v0 by the scaffold engine itself (see internal/
// scaffold/engine.go), but the merge mechanism it would use is identical to
// Caddyfile merging and is proven here against two fake modules.
func TestMergeTextBlocksGoWork(t *testing.T) {
	base := []byte("go 1.23\n\nuse (\n\t# kiln:insert\n)\n")
	fragments := []NamedFragment{
		{PluginRef: "backend/go-api", Content: "./demoapp-api"},
		{PluginRef: "payments/go-api/flutterwave", Content: "./demoapp-payments-flutterwave"},
	}

	out, err := MergeTextBlocks(base, "", fragments)
	if err != nil {
		t.Fatalf("MergeTextBlocks: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "./demoapp-api") || !strings.Contains(s, "./demoapp-payments-flutterwave") {
		t.Fatalf("both module entries should appear in the merged go.work, got: %s", s)
	}
	if !strings.HasPrefix(s, "go 1.23") {
		t.Error("content before the marker should be preserved")
	}
	if !strings.Contains(s, ")\n") {
		t.Error("content after the marker should be preserved")
	}
}

// TestMergeTextBlocksNamedSlotIndentsAndComments covers a marker nested
// inside an indented YAML block with a named slot: inserted lines take the
// marker's indentation, the provenance comment uses its comment token, and
// fragments for a different slot don't land there.
func TestMergeTextBlocksNamedSlotIndentsAndComments(t *testing.T) {
	base := []byte("steps:\n  # kiln:insert build\n  // kiln:insert other\n")
	out, err := MergeTextBlocks(base, "build", []NamedFragment{
		{PluginRef: "hosting/x", Content: "- run: make\n  name: build\n"},
	})
	if err != nil {
		t.Fatalf("MergeTextBlocks: %v", err)
	}
	want := "steps:\n  # --- hosting/x ---\n  - run: make\n    name: build\n"
	if got := string(StripMarkers(out)); got != want {
		t.Errorf("merged =\n%q\nwant\n%q", got, want)
	}

	if _, err := MergeTextBlocks(base, "missing", nil); err == nil {
		t.Error("expected an error for a slot with no marker line")
	}
}

func TestMergeTextBlocksSlashComment(t *testing.T) {
	base := []byte("func wire() {\n\t// kiln:insert\n}\n")
	out, err := MergeTextBlocks(base, "", []NamedFragment{{PluginRef: "payments/stripe", Content: "register(stripe.New())"}})
	if err != nil {
		t.Fatalf("MergeTextBlocks: %v", err)
	}
	want := "func wire() {\n\t// --- payments/stripe ---\n\tregister(stripe.New())\n}\n"
	if got := string(StripMarkers(out)); got != want {
		t.Errorf("merged =\n%q\nwant\n%q", got, want)
	}
}
