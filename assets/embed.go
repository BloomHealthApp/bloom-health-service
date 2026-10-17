// Package assets embeds the per-environment configuration files. They never contain secrets.
package assets

import "embed"

// Configs holds dev-config.json, stag-config.json and prod-config.json.
//
//go:embed *-config.json
var Configs embed.FS
