// Package scaffold implements the compose engine that turns a set of
// selected plugins into a generated project on disk.
package scaffold

import "regexp"

// placeholderPattern matches Kiln's bare {{NAME}} placeholder syntax. It is
// deliberately disjoint from Compose/shell ${VAR} interpolation syntax
// (different delimiter pair entirely), so files like docker-compose.yml
// that already use ${VAR} are never mistaken for Kiln placeholders.
var placeholderPattern = regexp.MustCompile(`\{\{[A-Z0-9_]+\}\}`)

// Values maps placeholder names (without delimiters) to their resolved
// values.
type Values map[string]string

// Render replaces every {{NAME}} occurrence in content with values[NAME].
// A placeholder with no matching value is left in the output unchanged, on
// purpose — callers that need full resolution should check the result with
// HasUnresolved rather than relying on Render to fail.
func Render(content []byte, values Values) []byte {
	return placeholderPattern.ReplaceAllFunc(content, func(tok []byte) []byte {
		name := string(tok[2 : len(tok)-2])
		if v, ok := values[name]; ok {
			return []byte(v)
		}
		return tok
	})
}

// RenderString is Render for strings — used for file and directory path
// segments, which are rendered the same way file contents are.
func RenderString(s string, values Values) string {
	return string(Render([]byte(s), values))
}

// HasUnresolved reports whether content still contains an unresolved
// {{NAME}} placeholder.
func HasUnresolved(content []byte) bool {
	return placeholderPattern.Match(content)
}
