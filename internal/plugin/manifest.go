// Package plugin defines the plugin manifest format and the interface used
// to resolve plugins by kind and name.
package plugin

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
