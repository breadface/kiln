package plugin

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func fixtureFS() fstest.MapFS {
	return fstest.MapFS{
		"backend/go-api/plugin.yaml": &fstest.MapFile{Data: []byte(`
name: go-api
kind: backend
version: 0.1.0
description: fake go-api plugin
`)},
		"backend/go-api/files/main.go": &fstest.MapFile{Data: []byte("package main\n")},
		"hosting/docker-swarm-hetzner/plugin.yaml": &fstest.MapFile{Data: []byte(`
name: docker-swarm-hetzner
kind: hosting
version: 0.1.0
description: fake hosting plugin
`)},
	}
}

func TestResolve(t *testing.T) {
	l := NewEmbedLoader(fixtureFS())

	p, err := l.Resolve(KindBackend, "go-api")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Manifest.Name != "go-api" || p.Manifest.Kind != KindBackend {
		t.Fatalf("unexpected manifest: %+v", p.Manifest)
	}
	if p.Source != "embedded" {
		t.Fatalf("Source = %q, want %q", p.Source, "embedded")
	}
	if _, err := fs.ReadFile(p.FS, "files/main.go"); err != nil {
		t.Fatalf("expected FS rooted at plugin dir, ReadFile files/main.go: %v", err)
	}
}

func TestResolveMissing(t *testing.T) {
	l := NewEmbedLoader(fixtureFS())
	if _, err := l.Resolve(KindBackend, "does-not-exist"); err == nil {
		t.Fatal("expected error for missing plugin, got nil")
	}
	if _, err := l.Resolve(KindFrontend, "go-api"); err == nil {
		t.Fatal("expected error for wrong kind, got nil")
	}
}

func TestList(t *testing.T) {
	l := NewEmbedLoader(fixtureFS())

	all, err := l.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List(\"\") returned %d plugins, want 2", len(all))
	}
	// Deterministic order: backend before hosting (Kind.Order()).
	if all[0].Manifest.Kind != KindBackend || all[1].Manifest.Kind != KindHosting {
		t.Fatalf("unexpected order: %v, %v", all[0].Manifest.Kind, all[1].Manifest.Kind)
	}

	backendOnly, err := l.List(KindBackend)
	if err != nil {
		t.Fatalf("List(backend): %v", err)
	}
	if len(backendOnly) != 1 || backendOnly[0].Manifest.Name != "go-api" {
		t.Fatalf("List(backend) = %+v, want just go-api", backendOnly)
	}
}
