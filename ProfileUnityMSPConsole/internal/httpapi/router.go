package httpapi

import (
	"io/fs"
	"net/http"

	"profileunity-msp-console/web"
)

// NewRouter assembles the server's HTTP handler: the health endpoint and
// the embedded frontend. API endpoints for tenants, collection, and
// reporting are added in later build phases behind an explicit whitelist,
// per the reference project's proxy pattern — nothing beyond what is
// deliberately exposed here is reachable from the frontend.
func NewRouter() (http.Handler, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", HealthHandler(func() SchedulerStatus {
		return SchedulerStatus{Status: "not_implemented"}
	}))

	distFS, err := fs.Sub(web.Dist, web.DistDir)
	if err != nil {
		return nil, err
	}
	mux.Handle("/", http.FileServer(http.FS(distFS)))

	return mux, nil
}
