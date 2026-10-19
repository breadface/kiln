// Package contracts embeds the contract schemas plugins provide and consume.
// A contract is the interface between plugins: a provider fills in its
// fields, and a consumer is generated once per provider without knowing
// which plugin that is. See README.md for the format.
package contracts

import "embed"

//go:embed *.yaml
var FS embed.FS
