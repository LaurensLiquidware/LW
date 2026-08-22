// Package static embeds the built Angular application into the executable.
//
// Section 3 of the Sparks Tool Project Review Checklist forbids CDN-loaded
// runtime dependencies. Embedding the whole UI -- scripts, styles and fonts -- in
// the binary means there is nothing to fetch at runtime and the tool works in a
// fully air-gapped environment.
//
// build/build.sh and build/Build.ps1 populate ui/ from the Angular build output
// before compiling. The placeholder committed here lets `go build` and the Go
// tests work in a checkout where the UI has not been built yet; the build scripts
// refuse to package a binary that still contains it.
package static

import (
	"embed"
	"io/fs"
)

//go:embed all:ui
var embedded embed.FS

// PlaceholderMarker appears in the committed placeholder index.html. The build
// scripts fail if it survives into a packaged binary.
const PlaceholderMarker = "PSM_UI_PLACEHOLDER"

// UI returns the built Angular application rooted at its index.html.
func UI() (fs.FS, error) {
	return fs.Sub(embedded, "ui")
}

// IsPlaceholder reports whether the embedded UI is still the committed
// placeholder rather than a real Angular build.
func IsPlaceholder() bool {
	b, err := embedded.ReadFile("ui/index.html")
	if err != nil {
		return true
	}
	for i := 0; i+len(PlaceholderMarker) <= len(b); i++ {
		if string(b[i:i+len(PlaceholderMarker)]) == PlaceholderMarker {
			return true
		}
	}
	return false
}
