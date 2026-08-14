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
func NewRouter(schedulerStatus func() SchedulerStatus, authDeps AuthDeps, tenantDeps TenantDeps, dashboardDeps DashboardDeps, historyDeps HistoryDeps, reportDeps ReportDeps, alertDeps AlertDeps) (http.Handler, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", HealthHandler(schedulerStatus))
	mux.HandleFunc("/api/version", VersionHandler)

	mux.HandleFunc("/api/csrf", CSRFHandler(authDeps))
	mux.Handle("/api/auth/login", auth.RequireCSRF(LoginHandler(authDeps)))
	mux.Handle("/api/auth/logout", auth.RequireCSRF(RequireSession(authDeps.Sessions, LogoutHandler(authDeps))))
	mux.Handle("/api/auth/me", RequireSession(authDeps.Sessions, MeHandler(authDeps)))

	// Tenant management (project brief §7.1). All of it requires a
	// session; the mutating routes additionally require CSRF. Test
	// Connection makes outbound requests to whatever host:port is in the
	// request body, so it must never be reachable anonymously — an
	// unauthenticated version of this endpoint would be an open SSRF/
	// port-scanning proxy through this server.
	mux.Handle("GET /api/tenants", RequireSession(authDeps.Sessions, ListTenantsHandler(tenantDeps)))
	mux.Handle("GET /api/tenants/{id}", RequireSession(authDeps.Sessions, GetTenantHandler(tenantDeps)))
	mux.Handle("POST /api/tenants", RequireSession(authDeps.Sessions, auth.RequireCSRF(CreateTenantHandler(tenantDeps))))
	mux.Handle("PUT /api/tenants/{id}", RequireSession(authDeps.Sessions, auth.RequireCSRF(UpdateTenantHandler(tenantDeps))))
	mux.Handle("DELETE /api/tenants/{id}", RequireSession(authDeps.Sessions, auth.RequireCSRF(DeleteTenantHandler(tenantDeps))))
	mux.Handle("POST /api/tenants/test", RequireSession(authDeps.Sessions, auth.RequireCSRF(TestConnectionHandler())))

	mux.Handle("GET /api/dashboard", RequireSession(authDeps.Sessions, DashboardHandler(dashboardDeps)))

	// Alerting (project brief §7.6): in-app only, no email/SMTP.
	mux.Handle("GET /api/alerts", RequireSession(authDeps.Sessions, AlertsHandler(alertDeps)))

	// History and graphs (project brief §7.4).
	mux.Handle("GET /api/tenants/{id}/history", RequireSession(authDeps.Sessions, TenantHistoryHandler(historyDeps)))
	mux.Handle("GET /api/history/portfolio", RequireSession(authDeps.Sessions, PortfolioHistoryHandler(historyDeps)))

	// Monthly reporting (project brief §7.5).
	mux.Handle("GET /api/tenants/{id}/reports/monthly", RequireSession(authDeps.Sessions, TenantMonthlyReportHandler(reportDeps)))
	mux.Handle("GET /api/tenants/{id}/reports/monthly.pdf", RequireSession(authDeps.Sessions, TenantMonthlyReportPDFHandler(reportDeps)))
	mux.Handle("GET /api/reports/portfolio/monthly", RequireSession(authDeps.Sessions, PortfolioMonthlyReportHandler(reportDeps)))
	mux.Handle("GET /api/reports/portfolio/monthly.pdf", RequireSession(authDeps.Sessions, PortfolioMonthlyReportPDFHandler(reportDeps)))

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
