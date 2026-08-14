package httpapi

import (
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"profileunity-msp-console/internal/auth"
	"profileunity-msp-console/internal/legal"
	"profileunity-msp-console/web"
)

// NewRouter assembles the server's HTTP handler: health, auth, and the
// embedded frontend. API endpoints are an explicit whitelist, per the
// reference project's proxy pattern — nothing beyond what is
// deliberately exposed here is reachable from the frontend. Tenant,
// collection, and reporting endpoints are added in later build phases
// behind the same pattern.
//
// schedulerStatus reports live scheduler state; pass a func that always
// returns SchedulerStatus{Status: "not_implemented"} where no scheduler
// exists yet.
func NewRouter(schedulerStatus func() SchedulerStatus, authDeps AuthDeps) (http.Handler, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", HealthHandler(schedulerStatus))
	mux.HandleFunc("/api/version", VersionHandler)

	mux.HandleFunc("/api/csrf", CSRFHandler(authDeps))
	mux.Handle("/api/auth/login", auth.RequireCSRF(LoginHandler(authDeps)))
	mux.Handle("/api/auth/logout", auth.RequireCSRF(RequireSession(authDeps.Sessions, LogoutHandler(authDeps))))
	mux.Handle("/api/auth/me", RequireSession(authDeps.Sessions, MeHandler(authDeps)))

	// Legal packaging (project brief §11.7): the license PDF and SBOM
	// ship inside the binary and are reachable at fixed top-level paths
	// so the About screen can link to them directly.
	legalServer := http.FileServer(http.FS(legal.FS))
	mux.Handle("/Spark_License.pdf", legalServer)
	mux.Handle("/bom.cdx.json", legalServer)
	mux.Handle("/THIRD-PARTY-NOTICES.txt", legalServer)

	distFS, err := fs.Sub(web.Dist, web.DistDir)
	if err != nil {
		return nil, err
	}
	mux.Handle("/", spaHandler(distFS))

	return mux, nil
}

// spaHandler serves the built Angular app, falling back to index.html for
// any path that isn't a real static file — the Angular router owns
// client-side routes like /login and /dashboard, which don't exist as
// files, and would otherwise 404 on a hard refresh or direct link.
func spaHandler(distFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(distFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(distFS, path); err != nil {
			r2 := new(http.Request)
			*r2 = *r
			r2.URL = new(url.URL)
			*r2.URL = *r.URL
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
