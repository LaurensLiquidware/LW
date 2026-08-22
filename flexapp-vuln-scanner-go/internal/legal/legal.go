// Package legal embeds the Sparks Tool License, SBOM, and third-party
// notices so they ship inside the binary itself, regardless of the
// process's working directory — project brief §11.7 requires these to be
// visible to the end user, not just present somewhere in the source tree.
//
// go:embed cannot reach outside this package directory, so the three
// files here are copies synced from the repository root by
// scripts/sync-legal.sh ahead of every build (same pattern as
// internal/version/VERSION_EMBED — see README.md "Version"). They are
// git-ignored; the root copies remain the single source of truth.
package legal

import "embed"

//go:embed Spark_License.pdf bom.cdx.json THIRD-PARTY-NOTICES.txt
var FS embed.FS
