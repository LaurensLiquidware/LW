package httpapi

import "net/http"

// ConfigHandler is a small, public endpoint the frontend reads at startup
// to learn whether the native file/folder picker (see picker_windows.go /
// picker_other.go) is available -- the New Scan screen only renders its
// Browse buttons when it is. pickerAvailable is a build-time constant
// (true only in the Windows build), not something that can fail at
// runtime, so this never needs a health probe or a separate address the
// way an out-of-process picker would.
func ConfigHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"pickerAvailable": pickerAvailable})
}
