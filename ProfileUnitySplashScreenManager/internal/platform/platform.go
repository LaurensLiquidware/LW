// Package platform isolates the operating-system-specific pieces: the native
// file dialog, clipboard image capture, elevation checks, launching the splash
// preview, and locating ProgramData.
//
// The Windows implementation is in platform_windows.go; everything else gets the
// stub in platform_other.go. Keeping this behind an interface means the core
// logic and the HTTP API are testable and runnable on a non-Windows machine even
// though the shipped product is Windows-only.
package platform

import "errors"

// ErrUnsupported is returned by the stub implementation.
var ErrUnsupported = errors.New("this operation is only supported on Windows")

// ErrNoClipboardImage is returned when the clipboard holds no bitmap.
var ErrNoClipboardImage = errors.New("no image found on the clipboard")

// Platform is the set of OS services the application needs.
type Platform interface {
	// IsElevated reports whether the process can write to Program Files.
	IsElevated() (bool, error)

	// ProgramData returns the machine-wide application data directory.
	ProgramData() string

	// DefaultTargetDir returns the conventional ProfileUnity Client.NET path.
	DefaultTargetDir() string

	// OpenFileDialog shows the native file picker and returns the chosen path.
	// An empty path with a nil error means the user cancelled.
	OpenFileDialog(title string, filter FileFilter) (string, error)

	// ClipboardImagePNG returns the clipboard's image encoded as PNG.
	ClipboardImagePNG() ([]byte, error)

	// OpenInBrowser opens a URL in the user's default browser.
	OpenInBrowser(url string) error

	// LaunchDetached starts an executable without waiting for it.
	LaunchDetached(path string) error

	// WebViewRuntimeVersion returns the installed WebView2 runtime version, or an
	// empty string when it is not installed.
	WebViewRuntimeVersion() string
}

// FileFilter describes one entry in the file dialog's type dropdown.
type FileFilter struct {
	// Label is shown to the user, e.g. "Image files".
	Label string
	// Patterns are the globs, e.g. []string{"*.png", "*.jpg"}.
	Patterns []string
}
