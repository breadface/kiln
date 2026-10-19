package plugin

import (
	"fmt"
	"io/fs"
	"path"
	"sort"

	"gopkg.in/yaml.v3"
)

// EmbedLoader resolves plugins from an fs.FS whose root directly contains
// one directory per Kind (e.g. "backend/", "hosting/"), each containing one
// directory per plugin name, each containing a plugin.yaml. It's named for
// its production use — backed by a //go:embed filesystem — but it accepts
// any fs.FS, including fstest.MapFS in tests, so it doesn't require real
// embedded plugin content to exist to be exercised.
type EmbedLoader struct {
	root fs.FS
}

// NewEmbedLoader returns a Loader backed by root.
func NewEmbedLoader(root fs.FS) *EmbedLoader {
	return &EmbedLoader{root: root}
}

func (l *EmbedLoader) Resolve(kind Kind, name string) (*Plugin, error) {
	dir := path.Join(string(kind), name)
	sub, err := fs.Sub(l.root, dir)
	if err != nil {
		return nil, fmt.Errorf("plugin %s/%s: %w", kind, name, err)
	}
	raw, err := fs.ReadFile(sub, "plugin.yaml")
	if err != nil {
		return nil, fmt.Errorf("plugin %s/%s: missing plugin.yaml: %w", kind, name, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("plugin %s/%s: invalid plugin.yaml: %w", kind, name, err)
	}
	digest, err := Digest(sub)
	if err != nil {
		return nil, fmt.Errorf("plugin %s/%s: %w", kind, name, err)
	}
	return &Plugin{Manifest: m, FS: sub, Source: "embedded", Digest: digest}, nil
}

func (l *EmbedLoader) List(kind Kind) ([]*Plugin, error) {
	var out []*Plugin

	kindDirs, err := fs.ReadDir(l.root, ".")
	if err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}
	for _, kd := range kindDirs {
		if !kd.IsDir() {
			continue
		}
		k := Kind(kd.Name())
		if kind != "" && k != kind {
			continue
		}

		nameDirs, err := fs.ReadDir(l.root, kd.Name())
		if err != nil {
			return nil, fmt.Errorf("list plugins under %s: %w", kd.Name(), err)
		}
		for _, nd := range nameDirs {
			if !nd.IsDir() {
				continue
			}
			p, err := l.Resolve(k, nd.Name())
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Manifest.Kind != out[j].Manifest.Kind {
			return out[i].Manifest.Kind.Order() < out[j].Manifest.Kind.Order()
		}
		return out[i].Manifest.Name < out[j].Manifest.Name
	})
	return out, nil
}
