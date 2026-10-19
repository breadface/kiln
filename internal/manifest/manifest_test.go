package manifest

import (
	"reflect"
	"testing"
	"time"
)

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	want := Project{
		KilnVersion: "0.1.0",
		GeneratedAt: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
		Plugins: []PluginRecord{
			{Kind: "backend", Name: "go-api", Version: "0.1.0", Source: "embedded"},
			{Kind: "hosting", Name: "docker-swarm-hetzner", Version: "0.1.0", Source: "embedded"},
		},
		Bindings: []BindingRecord{
			{Contract: "http-service@1", Consumer: "hosting/docker-swarm-hetzner", Provider: "backend/go-api", Values: map[string]string{"PORT": "8080"}},
		},
		Values: map[string]string{"APP_NAME": "demoapp", "DOMAIN": "demo.example.com"},
	}

	if err := Write(dir, want); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if !got.GeneratedAt.Equal(want.GeneratedAt) {
		t.Errorf("GeneratedAt = %v, want %v", got.GeneratedAt, want.GeneratedAt)
	}
	got.GeneratedAt = want.GeneratedAt // normalize before the rest of the comparison

	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestReadMissingFile(t *testing.T) {
	if _, err := Read(t.TempDir()); err == nil {
		t.Fatal("expected an error reading kiln.yaml from a directory without one")
	}
}
