package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
)

// Digest returns a content hash over every file in fsys — paths and bytes,
// in fs.WalkDir's lexical order — as "sha256:<hex>". Two plugins with the
// same digest generate the same output from the same values, whatever
// their version says, which is what lets `kiln apply` refuse to regenerate
// a project from plugins that changed since it was planned.
func Digest(fsys fs.FS) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		// Length-prefix both fields so no two different trees can produce
		// the same byte stream.
		fmt.Fprintf(h, "%d:%s\x00%d:", len(p), p, len(data))
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("hashing plugin: %w", err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
