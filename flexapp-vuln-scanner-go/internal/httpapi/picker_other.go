//go:build !windows

// This build has no native file/folder dialog to show -- the picker is a
// Win32-only feature (see picker_windows.go). Both handlers exist purely
// so the route table is identical on every platform; ConfigHandler's
// pickerAvailable=false is what actually keeps the frontend from ever
// calling them here.
package httpapi

import "net/http"

const pickerAvailable = false

func PickFileHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "the file/folder picker is only available on Windows", http.StatusNotImplemented)
}

func PickFolderHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "the file/folder picker is only available on Windows", http.StatusNotImplemented)
}
