// Package web embeds the built frontend into the server binary.
//
// dist/ is a placeholder until Phase 4 (Frontend shell) replaces it with
// the real Angular build output. The embed wiring itself — a single
// static filesystem the HTTP server can mount — does not change when
// that happens.
package web

import "embed"

//go:embed dist
var Dist embed.FS

// DistDir is the subdirectory inside Dist that holds the site root, for
// use with http.FS via fs.Sub.
const DistDir = "dist"
