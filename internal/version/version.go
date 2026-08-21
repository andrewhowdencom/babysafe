// Package version is a single source of truth for the binary's
// version string. The `Version` variable is intended to be overridden
// at build time via -ldflags (see Taskfile.yml).
package version

// Version is the semver tag of the build. Replace at build time with:
//
//	go build -ldflags "-X github.com/andrewhowdencom/babysafe/internal/version.Version=v1.2.3"
var Version = "v0.0.0-dev"

// String returns the version in a human-friendly form.
func String() string { return Version }
