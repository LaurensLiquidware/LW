package logo

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	// Decoders registered for image.DecodeConfig. The PowerShell version used
	// System.Drawing; these are pure Go, so no native imaging dependency and no
	// GDI+ file locking.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

// RecommendedWidth and RecommendedHeight are the splash logo dimensions the
// Liquidware KB recommends. Mismatches warn but never block.
const (
	RecommendedWidth  = 300
	RecommendedHeight = 86
)

// AllowedExtensions are the file types ProfileUnity will read. Fixed by
// ProfileUnity, not by us.
var AllowedExtensions = []string{".bmp", ".jpg", ".jpeg", ".gif", ".png", ".tif", ".tiff"}

// IsAllowedExtension reports whether ext (with leading dot, any case) is one
// ProfileUnity accepts.
func IsAllowedExtension(ext string) bool {
	ext = strings.ToLower(ext)
	for _, a := range AllowedExtensions {
		if a == ext {
			return true
		}
	}
	return false
}

// NormalizeExtension maps the extensions ProfileUnity does not recognise onto the
// ones it does. ProfileUnity looks for .jpg and .tif, never .jpeg or .tiff.
func NormalizeExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpeg":
		return ".jpg"
	case ".tiff":
		return ".tif"
	default:
		return strings.ToLower(ext)
	}
}

// ImageInfo describes a candidate logo file.
type ImageInfo struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
}

// MatchesRecommended reports whether the image is exactly the KB's recommended size.
func (i ImageInfo) MatchesRecommended() bool {
	return i.Width == RecommendedWidth && i.Height == RecommendedHeight
}

// Inspect decodes just enough of the file's header to read its real dimensions
// and format.
//
// This is also the content check: a file named .png that is not a PNG fails here
// rather than being copied into Client.NET, where ProfileUnity would render no
// logo at all. The PowerShell version validated only the extension until 0.2.0.
func Inspect(path string) (ImageInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return ImageInfo{}, fmt.Errorf("cannot open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return ImageInfo{}, fmt.Errorf("%s could not be read as an image: %w", filepath.Base(path), err)
	}
	return ImageInfo{Width: cfg.Width, Height: cfg.Height, Format: format}, nil
}
