// Package buildinfo holds the build-time version string injected at link time
// by the Dockerfile via -ldflags -X. "dev" is the value for any build without
// that flag (local `go build`, `make build`, `go test`). It lives in its own
// package rather than in cmd/server because package main cannot be imported by
// the HTTP layer that renders this value into /status (Phase 18 D-13, STAT-01).
package buildinfo

// Version is the commit the binary was built from. The Dockerfile rewrites it
// at link time with `-X .../internal/buildinfo.Version=<sha>`; it must be a var,
// not a const, because the Go linker can only rewrite a package-level var. A
// wrong import path in that flag is silently ignored by the linker, so a
// production build that still reports "dev" means the injection is broken.
var Version = "dev"

// Short returns Version truncated to its first 12 characters when longer and
// unchanged otherwise — the /status about-block display form of a 40-character
// commit SHA. "dev" is returned as-is.
func Short() string {
	const shortLen = 12
	if len(Version) > shortLen {
		return Version[:shortLen]
	}
	return Version
}
