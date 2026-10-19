package plugin

import "io/fs"

// Plugin is a resolved plugin: its manifest, plus a filesystem rooted at the
// plugin's own directory (so FS contains plugin.yaml, files/, fragments/).
type Plugin struct {
	Manifest Manifest
	FS       fs.FS
	// Source identifies where the plugin came from (e.g. "embedded", or in
	// future a git remote URL). Recorded into the generated project's
	// kiln.yaml so `kiln upgrade` knows where to re-fetch from.
	Source string
}

// Ref returns the plugin's "kind/name" reference.
func (p *Plugin) Ref() string {
	return string(p.Manifest.Kind) + "/" + p.Manifest.Name
}

// Binding connects a plugin consuming a contract to one selected plugin
// providing it. Values holds the provider's contract fields, validated
// against the contract schema and rendered with the resolved placeholder
// values — they're what the consumer's bindings/<contract>/ fragments see.
type Binding struct {
	Contract string
	Consumer *Plugin
	Provider *Plugin
	Values   map[string]string
}

// Loader resolves plugins by kind and name. EmbedLoader is the only
// implementation today; a future git-backed loader (resolving plugins
// through a remote index) can implement the same interface without any
// changes to callers, since callers only depend on Loader and Plugin.FS.
type Loader interface {
	// Resolve returns the single plugin matching kind and name.
	Resolve(kind Kind, name string) (*Plugin, error)
	// List returns every plugin of the given kind, or every plugin if kind
	// is empty.
	List(kind Kind) ([]*Plugin, error)
}
