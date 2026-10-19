// Package manifest reads and writes kiln.yaml, the record a generated
// project keeps of which plugins and values produced it — the basis for a
// future `kiln upgrade`.
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// FileName is the manifest's filename within a generated project's root.
const FileName = "kiln.yaml"

// PluginRecord records one plugin that contributed to a generated project.
type PluginRecord struct {
	Kind    string `yaml:"kind"`
	Name    string `yaml:"name"`
	Version string `yaml:"version,omitempty"`
	// Source identifies where the plugin was resolved from at generation
	// time (e.g. "embedded"; in future, a git remote URL).
	Source string `yaml:"source"`
}

// BindingRecord records one contract binding between two plugins.
type BindingRecord struct {
	Contract string            `yaml:"contract"`
	Consumer string            `yaml:"consumer"`
	Provider string            `yaml:"provider"`
	Values   map[string]string `yaml:"values,omitempty"`
}

// Project is the kiln.yaml contract.
type Project struct {
	KilnVersion string            `yaml:"kiln_version"`
	GeneratedAt time.Time         `yaml:"generated_at"`
	Plugins     []PluginRecord    `yaml:"plugins"`
	Bindings    []BindingRecord   `yaml:"bindings,omitempty"`
	Values      map[string]string `yaml:"values,omitempty"`
}

// Marshal serializes p as YAML. It does no I/O, so it's safe to call from
// contexts without a real filesystem (e.g. compiled to WebAssembly).
func Marshal(p Project) ([]byte, error) {
	data, err := yaml.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshaling %s: %w", FileName, err)
	}
	return data, nil
}

// Write serializes p as YAML into dir/kiln.yaml.
func Write(dir string, p Project) error {
	data, err := Marshal(p)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", FileName, err)
	}
	return nil
}

// Read parses dir/kiln.yaml.
func Read(dir string) (Project, error) {
	var p Project
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		return p, fmt.Errorf("reading %s: %w", FileName, err)
	}
	if err := yaml.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("parsing %s: %w", FileName, err)
	}
	return p, nil
}
