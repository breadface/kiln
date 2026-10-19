package contract

import (
	"testing"
	"testing/fstest"

	"github.com/breadface/kiln/contracts"
)

func TestLoadBundledContracts(t *testing.T) {
	r, err := Load(contracts.FS)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, id := range []string{"http-service@1", "migration@1", "env@1"} {
		if _, ok := r.Get(id); !ok {
			t.Errorf("bundled contract %s not found", id)
		}
	}
}

func TestLoadRejectsMisnamedFile(t *testing.T) {
	fsys := fstest.MapFS{"svc.yaml": &fstest.MapFile{Data: []byte("name: svc\nversion: 1\n")}}
	if _, err := Load(fsys); err == nil {
		t.Fatal("expected an error for a contract file not named <name>@<version>.yaml")
	}
}

func TestApply(t *testing.T) {
	c := Contract{Name: "svc", Version: 1, Fields: []Field{
		{Name: "NAME", Required: true},
		{Name: "PREFIX", Default: "/"},
	}}

	got, err := c.Apply(map[string]string{"NAME": "api"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got["NAME"] != "api" || got["PREFIX"] != "/" {
		t.Errorf("Apply = %v, want NAME=api and the PREFIX default", got)
	}

	if _, err := c.Apply(map[string]string{}); err == nil {
		t.Error("expected an error for a missing required field")
	}
	if _, err := c.Apply(map[string]string{"NAME": "api", "TYPO": "x"}); err == nil {
		t.Error("expected an error for a field outside the schema")
	}
}
