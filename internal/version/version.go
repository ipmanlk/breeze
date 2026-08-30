// Package version holds the build version, injected via ldflags at build time.
// This lets operators determine the running version and validate upgrades.
package version

// Version is set at link time via -ldflags "-X ipmanlk/breeze/internal/version.Version=...".
// Defaults to "dev" when not set (e.g. `go run` / `go build` without ldflags).
var Version = "dev"
