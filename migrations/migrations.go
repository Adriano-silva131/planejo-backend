package migrations

import "embed"

// The pattern also embeds this file so the package compiles before the first
// migration exists; goose ignores files without a version prefix.
//
//go:embed *
var FS embed.FS
