// Package version holds build metadata injected at link time by
// goreleaser (or the Makefile's snapshot build) via -ldflags -X. See
// docs/adr/0017-build-and-release-goreleaser.md.
package version

// Version, Commit, and Date are overwritten at build time via
// -ldflags "-X purser/internal/version.Version=...". Left at these
// defaults for unstamped `go build`/`go run` during development.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns the version, or "version (commit, date)" once build
// metadata has actually been injected.
func String() string {
	if Commit == "none" && Date == "unknown" {
		return Version
	}
	return Version + " (" + Commit + ", " + Date + ")"
}
