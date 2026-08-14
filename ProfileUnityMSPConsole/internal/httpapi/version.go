package httpapi

import (
	"net/http"

	"profileunity-msp-console/internal/version"
)

// VersionHandler is a small, public endpoint the About screen (project
// brief §11.7) reads from, separate from /healthz so the frontend doesn't
// have to parse operational health just to show a version number.
func VersionHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": version.Version})
}
