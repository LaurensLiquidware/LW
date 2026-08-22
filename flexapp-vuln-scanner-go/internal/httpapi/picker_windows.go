//go:build windows

package httpapi

import (
	"net/http"
	"runtime"

	"github.com/lxn/walk"
)

// pickerAvailable is read by ConfigHandler; see appconfig.go.
const pickerAvailable = true

// PickFileHandler and PickFolderHandler show a real Win32 common dialog
// (via lxn/walk's FileDialog, the same wrapper cmd/tray already uses for
// its Change Port dialog) and return the chosen path as JSON. Unlike an
// earlier version of this feature, these run directly in the server
// process with no separate picker server and no dependency on cmd/tray
// being the process that's running: GetOpenFileName/GetSaveFileName and
// SHBrowseForFolder are blocking Win32 calls that pump their own message
// loop internally, so no walk.MainWindow or Synchronize call is needed --
// passing a nil owner is valid (FileDialog treats nil the same as "no
// owner window"). Locking the OS thread for the call's duration keeps the
// COM apartment ShowBrowseFolder initializes on the same thread it's torn
// down on, which OleInitialize/OleUninitialize require.
//
// This only works when the process has an interactive desktop to show UI
// on (true whether it's launched by cmd/tray or double-clicked directly;
// false if ever run as a Windows Service in Session 0, same limitation
// every Win32 GUI call has there).
func PickFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	title := r.URL.Query().Get("title")
	if title == "" {
		title = "Select a File"
	}
	filter := r.URL.Query().Get("filter")

	var (
		accepted bool
		path     string
		dlgErr   error
	)
	func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		dlg := &walk.FileDialog{Title: title, Filter: filter}
		accepted, dlgErr = dlg.ShowOpen(nil)
		path = dlg.FilePath
	}()

	writePickResult(w, accepted, path, dlgErr)
}

func PickFolderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	title := r.URL.Query().Get("title")
	if title == "" {
		title = "Select a Folder"
	}

	var (
		accepted bool
		path     string
		dlgErr   error
	)
	func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		dlg := &walk.FileDialog{Title: title}
		accepted, dlgErr = dlg.ShowBrowseFolder(nil)
		path = dlg.FilePath
	}()

	writePickResult(w, accepted, path, dlgErr)
}

func writePickResult(w http.ResponseWriter, accepted bool, path string, dlgErr error) {
	if dlgErr != nil {
		http.Error(w, dlgErr.Error(), http.StatusInternalServerError)
		return
	}
	if !accepted {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}
