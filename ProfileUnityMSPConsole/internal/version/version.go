// Package version exposes the single build version string.
//
// The root VERSION file is the only place a human edits the version.
// go:embed cannot read outside its own package directory, so
// scripts/sync-version.sh copies VERSION into VERSION_EMBED (generated,
// git-ignored) before every build. If that copy is missing or stale, this
// package fails to compile or reports it explicitly — it must never
// silently ship a wrong version. See README.md "Version".
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION_EMBED
var embedded string

// Version is the semantic version of this build, e.g. "0.1.0".
var Version = strings.TrimSpace(embedded)
