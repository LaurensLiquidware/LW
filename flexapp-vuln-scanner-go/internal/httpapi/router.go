package httpapi

import (
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"flexapp-vuln-scanner/internal/legal"
	"flexapp-vuln-scanner/web"
)

// NewRouter assembles the server's HTTP handler: health, version, the
// legal/SBOM artifacts, and the embedded frontend. This is deliberately
// minimal for now -- scan endpoints (new scan, progress, results,
// compare) are added in later build phases behind the same
// explicit-whitelist pattern the reference project uses.
func NewRouter() (http.Handler, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", HealthHandler())
	mux.HandleFunc("/api/version", VersionHandler)

	// Legal packaging (Sparks Tool checklist §7): the license PDF and SBOM
	// ship inside the binary and are reachable at fixed top-level paths so
	// the About screen can link to them directly.
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
// any path that isn't a real static file -- the Angular router owns
// client-side routes, which don't exist as files and would otherwise 404
// on a hard refresh or direct link.
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
