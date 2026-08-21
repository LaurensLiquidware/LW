package httpapi

import "net/http"

// ConfigHandler is a small, public endpoint the frontend reads at startup
// to discover where the native file/folder picker (hosted by cmd/tray on
// Windows, see cmd/tray/picker_windows.go) is listening -- the server
// itself never starts the picker, since showing a native dialog needs a
// Windows GUI thread the headless server doesn't have. The frontend
// probes pickerAddr's health endpoint itself and hides its Browse
// buttons if it's unreachable, so an empty or wrong address just
// degrades gracefully rather than erroring.
func ConfigHandler(pickerAddr string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"pickerAddr": pickerAddr})
	}
}
