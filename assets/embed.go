// Package assets embeds the per-environment configuration files and the public privacy policy. They never contain secrets.
package assets

import "embed"

// Configs holds dev-config.json, stag-config.json and prod-config.json.
//
//go:embed *-config.json
var Configs embed.FS

// PrivacyPolicy is privacy-policy.md, a copy of the app's docs/privacy-policy.md, served at GET /privacy.
//
//go:embed privacy-policy.md
var PrivacyPolicy string
