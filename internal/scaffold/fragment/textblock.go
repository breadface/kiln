package fragment

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// DefaultMarker is the line a base file uses to mark where fragments for
// the unnamed slot get inserted.
const DefaultMarker = "# kiln:insert"

// markerPattern matches a marker line: optional indentation, a comment
// token, "kiln:insert", and an optional slot name, e.g.
//
//	# kiln:insert
//	  # kiln:insert routes
//	// kiln:insert wiring
//	<!-- kiln:insert secrets -->
//
// The indentation and comment token are reused for everything inserted at
// the marker, so fragments can be written unindented and provenance
// comments stay valid in whatever language the base file is in.
var markerPattern = regexp.MustCompile(`^([ \t]*)(#|//|--|;|<!--)[ \t]*kiln:insert(?:[ \t]+([A-Za-z0-9_-]+))?[ \t]*(?:-->)?[ \t]*$`)

// NamedFragment is a text fragment with a label identifying which plugin
// contributed it, so the merged output stays legible about where each block
// came from.
type NamedFragment struct {
	PluginRef string // e.g. "hosting/docker-swarm-hetzner"
	Content   string
}

// MergeTextBlocks inserts fragments at the first marker line in base for
// slot ("" is the unnamed slot), each preceded by a provenance comment.
// Every inserted line is prefixed with the marker's own indentation. This
// one mechanism is reused for every text-based shared file Kiln merges
// (Caddyfile, go.work, a CI workflow) — it has no knowledge of which file
// it's operating on.
func MergeTextBlocks(base []byte, slot string, fragments []NamedFragment) ([]byte, error) {
	lines := strings.Split(string(base), "\n")

	idx := -1
	var indent, comment string
	for i, l := range lines {
		m := markerPattern.FindStringSubmatch(l)
		if m != nil && m[3] == slot {
			idx, indent, comment = i, m[1], m[2]
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("base file has no %q marker line", markerText(slot))
	}

	var block []string
	for _, f := range fragments {
		provenance := fmt.Sprintf("%s%s --- %s ---", indent, comment, f.PluginRef)
		if comment == "<!--" {
			provenance += " -->"
		}
		block = append(block, provenance)
		for _, l := range strings.Split(strings.TrimRight(f.Content, "\n"), "\n") {
			if l != "" {
				l = indent + l
			}
			block = append(block, l)
		}
	}

	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:idx]...)
	out = append(out, block...)
	out = append(out, lines[idx:]...) // keep the marker: more fragments may target it
	return []byte(strings.Join(out, "\n")), nil
}

// StripMarkers removes every marker line from content, once all fragments
// have been merged — a slot nobody contributed to simply disappears.
func StripMarkers(content []byte) []byte {
	if !bytes.Contains(content, []byte("kiln:insert")) {
		return content
	}
	lines := strings.Split(string(content), "\n")
	out := lines[:0]
	for _, l := range lines {
		if !markerPattern.MatchString(l) {
			out = append(out, l)
		}
	}
	return []byte(strings.Join(out, "\n"))
}

func markerText(slot string) string {
	if slot == "" {
		return DefaultMarker
	}
	return DefaultMarker + " " + slot
}
