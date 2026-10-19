package fragment

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMergeYAMLDisjointServices(t *testing.T) {
	api := []byte(`
services:
  api:
    image: demoapp-api
    expose: ["8080"]
`)
	caddy := []byte(`
services:
  caddy:
    image: caddy:2-alpine
    ports: ["80:80", "443:443"]
`)

	out, err := MergeYAML([][]byte{api, caddy})
	if err != nil {
		t.Fatalf("MergeYAML: %v", err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal merged output: %v", err)
	}
	services, ok := doc["services"].(map[string]interface{})
	if !ok {
		t.Fatalf("services key missing or wrong type: %v", doc["services"])
	}
	if _, ok := services["api"]; !ok {
		t.Error("merged output missing services.api")
	}
	if _, ok := services["caddy"]; !ok {
		t.Error("merged output missing services.caddy")
	}
}

func TestMergeYAMLIdenticalDuplicateIsFine(t *testing.T) {
	a := []byte("services:\n  db:\n    image: postgres:15-alpine\n")
	b := []byte("services:\n  db:\n    image: postgres:15-alpine\n")

	if _, err := MergeYAML([][]byte{a, b}); err != nil {
		t.Fatalf("identical duplicate keys should not conflict: %v", err)
	}
}

func TestMergeYAMLScalarCollision(t *testing.T) {
	a := []byte("services:\n  api:\n    image: demoapp-api:v1\n")
	b := []byte("services:\n  api:\n    image: demoapp-api:v2\n")

	_, err := MergeYAML([][]byte{a, b})
	if err == nil {
		t.Fatal("expected a conflict error for differing values at the same key, got nil")
	}
	if !strings.Contains(err.Error(), "services.api.image") {
		t.Errorf("error should name the conflicting key path, got: %v", err)
	}
}

func TestMergeYAMLInvalidYAML(t *testing.T) {
	bad := []byte("services: [this is not: a map")
	if _, err := MergeYAML([][]byte{bad}); err == nil {
		t.Fatal("expected an error for invalid YAML fragment, got nil")
	}
}
