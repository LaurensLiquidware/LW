package httpapi

import (
	"encoding/json"
	"net/http"

	"flexapp-vuln-scanner/internal/version"
)

// VersionHandler is a small, public endpoint the About screen reads from,
// separate from /healthz so the frontend doesn't have to parse
// operational health just to show a version number.
func VersionHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": version.Version})
}

// writeJSON writes v as a JSON response body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
