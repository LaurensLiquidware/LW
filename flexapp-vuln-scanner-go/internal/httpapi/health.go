package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"flexapp-vuln-scanner/internal/version"
)

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	TimeUTC string `json:"timeUtc"`
}

// HealthHandler returns the health endpoint handler, used by the tray
// launcher to detect when the server process is ready to serve requests.
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := healthResponse{
			Status:  "ok",
			Version: version.Version,
			TimeUTC: time.Now().UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(resp)
	}
}
