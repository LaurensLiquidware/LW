//go:build !windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// stub is the non-Windows implementation. It exists so the core logic and the
// HTTP API can be built, tested and driven on a development machine; the shipped
// product is Windows-only.
type stub struct{}

// New returns the platform implementation for the current OS.
func New() Platform { return stub{} }

// IsElevated reports true off Windows: there is no Program Files to guard, and
// requiring a stub developer build to run as root would be worse.
func (stub) IsElevated() (bool, error) { return true, nil }

func (stub) ProgramData() string {
	if d := os.Getenv("PSM_PROGRAMDATA"); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), "psm-programdata")
}

func (s stub) DefaultTargetDir() string {
	if d := os.Getenv("PSM_TARGETDIR"); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), "psm-clientnet")
}

// OpenFileDialog has no native picker to show off Windows. For development and
// for the end-to-end tests it returns PSM_BROWSE_PATH when that is set, which
// lets the whole browse-preview-apply flow be exercised on a development machine.
//
// This affordance exists only in the non-Windows stub. The file carries a
// !windows build tag, so it is not compiled into the shipped executable at all
// and cannot be used to feed a path into the real tool.
func (stub) OpenFileDialog(string, FileFilter) (string, error) {
	if p := os.Getenv("PSM_BROWSE_PATH"); p != "" {
		return p, nil
	}
	return "", ErrUnsupported
}

// ClipboardImagePNG returns the file named by PSM_CLIPBOARD_PATH, standing in for
// a real clipboard bitmap. Same reasoning and same build-tag limits as
// OpenFileDialog above.
func (stub) ClipboardImagePNG() ([]byte, error) {
	if p := os.Getenv("PSM_CLIPBOARD_PATH"); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, ErrNoClipboardImage
		}
		return b, nil
	}
	return nil, ErrUnsupported
}
func (stub) OpenInBrowser(string) error    { return ErrUnsupported }
func (stub) LaunchDetached(string) error   { return ErrUnsupported }
func (stub) WebViewRuntimeVersion() string { return "" }

// ShowMessage writes to stderr off Windows; there is no native dialog to use.
func ShowMessage(title, text string, _ bool) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, text)
}

// RelaunchElevated is unsupported off Windows.
func RelaunchElevated([]string) error { return ErrUnsupported }
