// Package fragment merges per-plugin contributions into the shared root
// files a generated project needs (docker-compose.yml, Caddyfile, go.work),
// since more than one plugin can contribute to the same file.
package fragment

import (
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"
)

// MergeYAML deep-merges a set of YAML fragment documents (e.g. Compose
// files) into one. Two fragments defining the same key with the same value
// is fine (plugins are allowed to agree); two fragments defining the same
// key with different values is a conflict, since neither plugin knows about
// the other and Kiln can't guess which one should win.
func MergeYAML(fragments [][]byte) ([]byte, error) {
	merged := map[string]interface{}{}
	for _, f := range fragments {
		var doc map[string]interface{}
		if err := yaml.Unmarshal(f, &doc); err != nil {
			return nil, fmt.Errorf("parsing YAML fragment: %w", err)
		}
		if err := deepMerge(merged, doc, ""); err != nil {
			return nil, err
		}
	}
	out, err := yaml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged YAML: %w", err)
	}
	return out, nil
}

func deepMerge(dst, src map[string]interface{}, pathPrefix string) error {
	for k, v := range src {
		fullKey := k
		if pathPrefix != "" {
			fullKey = pathPrefix + "." + k
		}

		existing, ok := dst[k]
		if !ok {
			dst[k] = v
			continue
		}

		em, eok := existing.(map[string]interface{})
		sm, sok := v.(map[string]interface{})
		if eok && sok {
			if err := deepMerge(em, sm, fullKey); err != nil {
				return err
			}
			continue
		}

		if !reflect.DeepEqual(existing, v) {
			return fmt.Errorf("fragment merge conflict at %q: two plugins define different values", fullKey)
		}
	}
	return nil
}
