package fragment

import (
	"fmt"
	"strings"
)

// DefaultMarker is the line a base file uses to mark where fragments get
// inserted.
const DefaultMarker = "# kiln:insert"

// NamedFragment is a text fragment with a label identifying which plugin
// contributed it, so the merged output stays legible about where each block
// came from.
type NamedFragment struct {
	PluginRef string // e.g. "hosting/docker-swarm-hetzner"
	Content   string
}

// MergeTextBlocks inserts fragments, each preceded by a small provenance
// comment, at the first line in base that (after trimming whitespace)
// exactly equals marker. This one mechanism is reused for every text-based
// shared file Kiln merges (Caddyfile, go.work) — it has no knowledge of
// which file it's operating on.
func MergeTextBlocks(base []byte, marker string, fragments []NamedFragment) ([]byte, error) {
	lines := strings.Split(string(base), "\n")

	idx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == marker {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("base file has no %q marker line", marker)
	}

	var block []string
	for _, f := range fragments {
		block = append(block, fmt.Sprintf("# --- %s ---", f.PluginRef))
		block = append(block, strings.Split(strings.TrimRight(f.Content, "\n"), "\n")...)
		block = append(block, "")
	}

	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:idx]...)
	out = append(out, block...)
	out = append(out, lines[idx+1:]...)
	return []byte(strings.Join(out, "\n")), nil
}
