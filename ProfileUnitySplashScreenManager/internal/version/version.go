// Package version is the single source of truth for the application version.
//
// Section 6 of the Sparks Tool Project Review Checklist requires one source of
// truth: the version shown to the user, the version in the SBOM metadata, the
// version in the executable's file metadata and the version in the release
// artifact's filename must all agree. Everything derives from AppVersion here.
//
//   - the UI reads it from GET /api/about
//   - build/build.sh and build/Build.ps1 read it with `go run ./internal/version`
//   - goversioninfo stamps it into the .exe's file metadata
//   - bom.cdx.json's metadata.component.version must match it
package version

// AppVersion is the application version, semantic versioning MAJOR.MINOR.PATCH.
// Bump it for every release, including fix-only releases, and add a CHANGELOG.md
// entry. Do not hardcode this string anywhere else.
const AppVersion = "0.3.0"

// ProductName is the user-facing product name.
const ProductName = "ProfileUnity SplashScreen Logo Manager"

// Company owns the copyright on this tool.
const Company = "Liquidware"
