package generate

import (
	"fmt"

	"github.com/breadface/kiln/internal/contract"
	"github.com/breadface/kiln/internal/manifest"
	"github.com/breadface/kiln/internal/plugin"
	"github.com/breadface/kiln/internal/scaffold"
)

// Plan is everything generating a project would produce, computed entirely
// in memory: no files written, no hooks run. `kiln plan` prints it, and
// `kiln new` / `kiln apply` write it to disk.
type Plan struct {
	Plugins  []*plugin.Plugin
	Bindings []plugin.Binding
	Values   scaffold.Values
	// Result holds every generated file, including kiln.yaml.
	Result   scaffold.Result
	Manifest manifest.Project
}

// MakePlan binds contracts between the selected plugins, generates every
// file, and records the result in kiln.yaml. values must already be
// resolved (see ResolveValues).
func MakePlan(kilnVersion string, selected []*plugin.Plugin, registry *contract.Registry, values scaffold.Values) (*Plan, error) {
	bindings, values, err := ResolveBindings(selected, registry, values)
	if err != nil {
		return nil, err
	}
	result, err := scaffold.Generate(scaffold.Options{Plugins: selected, Bindings: bindings, Values: values})
	if err != nil {
		return nil, err
	}

	m := BuildManifest(kilnVersion, selected, bindings, values, result.Files)
	data, err := manifest.Marshal(m)
	if err != nil {
		return nil, err
	}
	result.Files[manifest.FileName] = scaffold.GeneratedFile{Content: data}

	return &Plan{Plugins: selected, Bindings: bindings, Values: values, Result: result, Manifest: m}, nil
}

// FromManifest turns a recorded kiln.yaml back into a plugin selection and
// a request that reproduces it. Every recorded plugin must still resolve
// to the same version and content digest — regenerating from plugins that
// changed underneath a plan would silently produce a different project,
// which is an upgrade, not an apply.
func FromManifest(loader plugin.Loader, m manifest.Project) ([]*plugin.Plugin, Request, string, error) {
	appName := m.Values["APP_NAME"]
	if appName == "" {
		return nil, Request{}, "", fmt.Errorf("%s has no APP_NAME value", manifest.FileName)
	}

	selected := make([]*plugin.Plugin, 0, len(m.Plugins))
	for _, rec := range m.Plugins {
		p, err := loader.Resolve(plugin.Kind(rec.Kind), rec.Name)
		if err != nil {
			return nil, Request{}, "", err
		}
		if rec.Version != "" && rec.Version != p.Manifest.Version {
			return nil, Request{}, "", fmt.Errorf("plugin %s is version %s, but this plan was made with %s", p.Ref(), p.Manifest.Version, rec.Version)
		}
		if rec.Digest != "" && rec.Digest != p.Digest {
			return nil, Request{}, "", fmt.Errorf("plugin %s %s has changed since this plan was made (digest %s, plan has %s)", p.Ref(), p.Manifest.Version, p.Digest, rec.Digest)
		}
		selected = append(selected, p)
	}

	set := make(map[string]string, len(m.Values))
	for k, v := range m.Values {
		set[k] = v
	}
	return selected, Request{ProjectName: appName, Set: set}, appName, nil
}
