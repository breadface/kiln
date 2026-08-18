// Package plugins embeds Kiln's bundled plugin content into the compiled
// binary, so a distributed `kiln` binary works standalone without the
// source repository present on disk. See README.md for the plugin
// authoring format.
package plugins

import "embed"

//go:embed backend hosting
var FS embed.FS
