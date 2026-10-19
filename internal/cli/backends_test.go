package cli

import (
	"bytes"
	"io/fs"
	"testing"

	"github.com/breadface/kiln/plugins"
)

// TestGoBackendsShareBindings: both Go backends generate the same payments
// package (and pass env@1 into their container the same way), from their
// own copies of bindings/ — a plugin can't share files with another. This
// keeps the copies from drifting: change one, copy it to the other.
func TestGoBackendsShareBindings(t *testing.T) {
	read := func(root string) map[string][]byte {
		t.Helper()
		files := map[string][]byte{}
		sub, err := fs.Sub(plugins.FS, root)
		if err != nil {
			t.Fatal(err)
		}
		err = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			files[p], err = fs.ReadFile(sub, p)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	a, b := read("backend/go-api/bindings"), read("backend/go-grpc-gateway/bindings")
	if len(a) == 0 {
		t.Fatal("backend/go-api has no bindings/")
	}
	for p, content := range a {
		if other, ok := b[p]; !ok {
			t.Errorf("backend/go-grpc-gateway/bindings is missing %s", p)
		} else if !bytes.Equal(content, other) {
			t.Errorf("bindings/%s differs between go-api and go-grpc-gateway", p)
		}
	}
	for p := range b {
		if _, ok := a[p]; !ok {
			t.Errorf("backend/go-api/bindings is missing %s", p)
		}
	}
}
