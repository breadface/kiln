// Package contract loads contract schemas and validates the values a
// plugin provides against them.
package contract

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Field is one value a contract's providers supply.
type Field struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required,omitempty"`
	Default     string `yaml:"default,omitempty"`
}

// Contract is a contracts/<name>@<version>.yaml schema.
type Contract struct {
	Name        string  `yaml:"name"`
	Version     int     `yaml:"version"`
	Description string  `yaml:"description"`
	Fields      []Field `yaml:"fields"`
}

// ID returns the contract's "<name>@<version>" identifier.
func (c Contract) ID() string {
	return c.Name + "@" + strconv.Itoa(c.Version)
}

// Apply validates a provider's values against the schema and fills in
// defaults: every required field must be present and non-empty, and no
// field outside the schema is allowed.
func (c Contract) Apply(with map[string]string) (map[string]string, error) {
	known := map[string]bool{}
	out := map[string]string{}
	for _, f := range c.Fields {
		known[f.Name] = true
		v, ok := with[f.Name]
		switch {
		case ok && v != "":
			out[f.Name] = v
		case f.Required:
			return nil, fmt.Errorf("%s: missing required field %s", c.ID(), f.Name)
		default:
			out[f.Name] = f.Default
		}
	}
	var unknown []string
	for k := range with {
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("%s: unknown field(s) %s", c.ID(), strings.Join(unknown, ", "))
	}
	return out, nil
}

// Registry holds every known contract, keyed by ID.
type Registry struct {
	byID map[string]Contract
}

// Load reads every *.yaml contract schema at the root of fsys. Each file
// must be named after its own ID, e.g. http-service@1.yaml.
func Load(fsys fs.FS) (*Registry, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("listing contracts: %w", err)
	}
	r := &Registry{byID: map[string]Contract{}}
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".yaml" {
			continue
		}
		raw, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("reading contract %s: %w", e.Name(), err)
		}
		var c Contract
		if err := yaml.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("parsing contract %s: %w", e.Name(), err)
		}
		if want := c.ID() + ".yaml"; e.Name() != want {
			return nil, fmt.Errorf("contract %s declares %s; file must be named %s", e.Name(), c.ID(), want)
		}
		r.byID[c.ID()] = c
	}
	return r, nil
}

// Get returns the contract with the given "<name>@<version>" ID.
func (r *Registry) Get(id string) (Contract, bool) {
	c, ok := r.byID[id]
	return c, ok
}

// List returns every contract, sorted by ID.
func (r *Registry) List() []Contract {
	out := make([]Contract, 0, len(r.byID))
	for _, c := range r.byID {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}
