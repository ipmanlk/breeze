// Package ui embeds the built SPA into the Go binary.
// Build with: make build-ui
package ui

import "embed"

// dist always contains the committed .gitkeep placeholder, so `go build`,
// `go vet`, tests, and linters never depend on the UI toolchain having run.
// `all:` is required for the embed to count dotfiles like .gitkeep; the real
// build output (make build-ui) populates the rest of the directory.
//
//go:embed all:dist
var Dist embed.FS
