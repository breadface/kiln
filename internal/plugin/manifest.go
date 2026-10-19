// Package plugin defines the plugin manifest format and the interface used
// to resolve plugins by kind and name.
package plugin

import (
	"sort"
	"strings"
)

// Kind identifies what category of project concern a plugin addresses.
type Kind string

const (
	KindBackend  Kind = "backend"
	KindFrontend Kind = "frontend"
	KindMobile   Kind = "mobile"
	KindPayments Kind = "payments"
	KindHosting  Kind = "hosting"
	KindGlue     Kind = "glue"
)

// kindOrder controls the deterministic order plugins are applied and their
// fragments are merged in: application services first, infrastructure last.
var kindOrder = map[Kind]int{
	KindBackend:  0,
	KindFrontend: 1,
	KindMobile:   2,
	KindPayments: 3,
	KindHosting:  4,
	KindGlue:     5,
}

// Kinds returns every known Kind in application order.
func Kinds() []Kind {
	out := make([]Kind, 0, len(kindOrder))
	for k := range kindOrder {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Order() < out[j].Order() })
	return out
}

// Known reports whether k is one of the kinds Kiln knows how to order.
func (k Kind) Known() bool {
	_, ok := kindOrder[k]
	return ok
}

// Order returns k's position in the deterministic plugin-application order.
// Unknown kinds sort last.
func (k Kind) Order() int {
	if o, ok := kindOrder[k]; ok {
		return o
	}
	return len(kindOrder)
}

// Placeholder describes a template variable a plugin's files reference.
type Placeholder struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Default     string `yaml:"default,omitempty"`
	Required    bool   `yaml:"required,omitempty"`

	// Flag, if set, exposes this placeholder as a `kiln new --<flag>` CLI
	// option. Plugins sharing a placeholder name may share a flag.
	Flag string `yaml:"flag,omitempty"`

	// Derive computes this placeholder from another one when it isn't set
	// explicitly. Tried before Default.
	Derive *Derive `yaml:"derive,omitempty"`
}

// Derive computes a placeholder value from another placeholder's value,
// without giving plugins a template language. Exactly one of Match or
// Replace is used:
//
//   - Match + Template: if the regexp Match matches From's value, the value
//     is Template expanded with the match's capture groups ($1, ${name}).
//     No match leaves the placeholder unset.
//   - Replace: every occurrence of each key in From's value is replaced
//     with its value, e.g. {".": "/"} turns com.example.app into
//     com/example/app.
type Derive struct {
	From     string            `yaml:"from"`
	Match    string            `yaml:"match,omitempty"`
	Template string            `yaml:"template,omitempty"`
	Replace  map[string]string `yaml:"replace,omitempty"`
}

// Provide declares that a plugin implements a contract, with the values
// the contract's schema asks for (see contracts/). A plugin may provide the
// same contract more than once — e.g. one env@1 entry per env var.
type Provide struct {
	Contract string            `yaml:"contract"` // "<name>@<major>", e.g. "http-service@1"
	With     map[string]string `yaml:"with,omitempty"`
}

// Consume declares that a plugin works with every selected plugin
// providing a contract — e.g. a hosting plugin consumes http-service@1 to
// deploy each service, without knowing which backend provided it.
type Consume struct {
	Contract string `yaml:"contract"`
	// Min and Max bound how many providers must be bound. Max 0 means
	// unlimited.
	Min int `yaml:"min,omitempty"`
	Max int `yaml:"max,omitempty"`
	// As, if set, exports the single bound provider's fields as global
	// placeholders named <As>_<FIELD> (e.g. API_PORT), so this plugin's
	// files/ can reference them. Requires Max: 1.
	As string `yaml:"as,omitempty"`
}

// ContractName returns the name part of a "<name>@<major>" contract ID.
func ContractName(id string) string {
	name, _, _ := strings.Cut(id, "@")
	return name
}

// Manifest is the plugin.yaml contract every plugin ships.
type Manifest struct {
	Name        string `yaml:"name"`
	Kind        Kind   `yaml:"kind"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`

	// Requires and Conflicts entries are either a bare kind ("backend" means
	// "any plugin of this kind must/must not be selected") or a specific
	// "kind/name" reference (e.g. "hosting/docker-swarm-hetzner").
	Requires  []string `yaml:"requires,omitempty"`
	Conflicts []string `yaml:"conflicts,omitempty"`

	Placeholders []Placeholder `yaml:"placeholders,omitempty"`

	Provides []Provide `yaml:"provides,omitempty"`
	Consumes []Consume `yaml:"consumes,omitempty"`

	// NextSteps are shell lines printed after generation, rendered with the
	// resolved placeholder values and run relative to the project root.
	NextSteps []string `yaml:"next_steps,omitempty"`

	// PostCopy is a path, relative to the plugin's own directory, to a shell
	// script run after all selected plugins' files are copied and merged.
	PostCopy string `yaml:"post_copy,omitempty"`

	// ContributesGoModule marks a plugin that ships its own go.mod at the
	// project root. Used to decide whether a go.work file is needed — it
	// only makes sense once two or more selected plugins set this.
	ContributesGoModule bool `yaml:"contributes_go_module,omitempty"`

	// Executable lists glob patterns, relative to files/, whose executable
	// bit must be restored after copy (embedded filesystems strip it).
	Executable []string `yaml:"executable,omitempty"`
}
