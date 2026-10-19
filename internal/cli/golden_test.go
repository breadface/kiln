package cli

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/breadface/kiln/internal/generate"
	"github.com/breadface/kiln/internal/manifest"
)

var update = flag.Bool("update", false, "rewrite testdata/plans/*.golden from the current plugins")

// TestGoldenPlans snapshots every supported plugin combination: the plan
// summary, a content hash per generated file, and the full content of
// every file plugins merged fragments into. A plugin change shows up in
// review as a diff of exactly what it does to each combination.
//
// After an intended change, regenerate with:
//
//	go test ./internal/cli -run TestGoldenPlans -update
func TestGoldenPlans(t *testing.T) {
	for name := range scenarios {
		t.Run(name, func(t *testing.T) {
			got := renderGolden(planScenario(t, name))
			path := filepath.Join("testdata", "plans", name+".golden")

			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run with -update to create it)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("plan for %s changed; if intended, run: go test ./internal/cli -run TestGoldenPlans -update\n\n%s",
					name, firstDifference(want, got))
			}
		})
	}
}

func renderGolden(plan *generate.Plan) []byte {
	var b bytes.Buffer
	printPlan(&b, plan)

	b.WriteString("\nHashes\n")
	for _, path := range sortedKeys(plan.Result.Files) {
		if path == manifest.FileName {
			continue // records generated_at, so never stable
		}
		fmt.Fprintf(&b, "  %s  %s\n", generate.HashContent(plan.Result.Files[path].Content), path)
	}

	for _, path := range sortedKeys(plan.Result.Files) {
		f := plan.Result.Files[path]
		if !f.Merged {
			continue
		}
		fmt.Fprintf(&b, "\n==> %s <==\n", path)
		b.Write(f.Content)
		if !bytes.HasSuffix(f.Content, []byte("\n")) {
			b.WriteString("\n")
		}
	}
	return b.Bytes()
}

// firstDifference shows the first differing line, so a failure points at
// the change instead of dumping two whole snapshots.
func firstDifference(want, got []byte) string {
	w, g := bytes.Split(want, []byte("\n")), bytes.Split(got, []byte("\n"))
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl []byte
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if !bytes.Equal(wl, gl) {
			return fmt.Sprintf("line %d:\n  want: %s\n  got:  %s", i+1, wl, gl)
		}
	}
	return ""
}
