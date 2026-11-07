// Package assets embeds the per-environment configuration files and the public policy pages. They never contain secrets.
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

// DeleteAccount is delete-account.md, the account-deletion page served at GET /delete-account (Google Play's
// Data safety "delete account URL"). Keep it in line with the deletion section of privacy-policy.md.
//
//go:embed delete-account.md
var DeleteAccount string
