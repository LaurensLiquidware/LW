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
func NewRouter(schedulerStatus func() SchedulerStatus, authDeps AuthDeps, tenantDeps TenantDeps, dashboardDeps DashboardDeps, historyDeps HistoryDeps, reportDeps ReportDeps, alertDeps AlertDeps, collectionDeps CollectionDeps, settingsDeps SettingsDeps, licenseDeps LicenseDeps) (http.Handler, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", HealthHandler(schedulerStatus))
	mux.HandleFunc("/api/version", VersionHandler)

	mux.HandleFunc("/api/csrf", CSRFHandler(authDeps))
	mux.Handle("/api/auth/login", auth.RequireCSRF(LoginHandler(authDeps)))
	mux.Handle("/api/auth/logout", auth.RequireCSRF(RequireSession(authDeps.Sessions, LogoutHandler(authDeps))))
	mux.Handle("/api/auth/me", RequireSession(authDeps.Sessions, MeHandler(authDeps)))
	mux.Handle("/api/auth/change-password", RequireSession(authDeps.Sessions, auth.RequireCSRF(ChangePasswordHandler(authDeps))))

	// Console login account management (the Users screen) -- lets a
	// signed-in operator add or remove other login accounts. There is no
	// separate admin role gate today (every screen is open to any
	// session, per the flat permission model this app already has); the
	// only hard restrictions are inside DeleteUserHandler itself
	// (can't delete yourself, can't delete the last remaining account).
	mux.Handle("GET /api/users", RequireSession(authDeps.Sessions, ListUsersHandler(authDeps)))
	mux.Handle("POST /api/users", RequireSession(authDeps.Sessions, auth.RequireCSRF(CreateUserHandler(authDeps))))
	mux.Handle("DELETE /api/users/{id}", RequireSession(authDeps.Sessions, auth.RequireCSRF(DeleteUserHandler(authDeps))))

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
	mux.Handle("POST /api/tenants/test", RequireSession(authDeps.Sessions, auth.RequireCSRF(DisallowInDemoMode(authDeps.DemoMode, TestConnectionHandler()))))

	mux.Handle("GET /api/dashboard", RequireSession(authDeps.Sessions, DashboardHandler(dashboardDeps)))

	// Manual "Collect Now" (project brief §7.2): runs the same collection
	// pass the scheduler's ticker runs, on demand, so a newly-added tenant
	// doesn't have to wait for the next scheduled interval.
	mux.Handle("POST /api/collect/run", RequireSession(authDeps.Sessions, auth.RequireCSRF(DisallowInDemoMode(collectionDeps.DemoMode, CollectNowHandler(collectionDeps)))))

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

	// Settings screen: everything not required to boot the process in
	// the first place (listen address, DB driver/DSN, the credential
	// encryption key, and the initial admin account stay env-var-only).
	// Changes here are pushed live into the running scheduler/report-mail/
	// session components and hot-swapped into the TLS listener — no
	// restart needed.
	mux.Handle("GET /api/settings", RequireSession(authDeps.Sessions, GetSettingsHandler(settingsDeps)))
	mux.Handle("PUT /api/settings", RequireSession(authDeps.Sessions, auth.RequireCSRF(UpdateSettingsHandler(settingsDeps))))
	mux.Handle("POST /api/settings/tls-cert", RequireSession(authDeps.Sessions, auth.RequireCSRF(UploadTLSCertHandler(settingsDeps))))
	mux.Handle("POST /api/settings/test-email", RequireSession(authDeps.Sessions, auth.RequireCSRF(TestEmailHandler(settingsDeps))))
	mux.Handle("POST /api/settings/send-report-now", RequireSession(authDeps.Sessions, auth.RequireCSRF(SendReportNowHandler(settingsDeps))))
	mux.Handle("GET /api/settings/logo", RequireSession(authDeps.Sessions, GetLogoHandler(settingsDeps)))
	mux.Handle("POST /api/settings/logo", RequireSession(authDeps.Sessions, auth.RequireCSRF(UploadLogoHandler(settingsDeps))))
	mux.Handle("DELETE /api/settings/logo", RequireSession(authDeps.Sessions, auth.RequireCSRF(ClearLogoHandler(settingsDeps))))

	// Licenses screen: pushing a signed license to a tenant's
	// ProfileUnity License Server (a distinct host/credential from the
	// tenant's own console). Preview is a local decode, no network call,
	// so it's available even in demo mode; Checkup and Push both reach a
	// real host and are blocked in demo mode the same way Test
	// Connection/Collect Now already are.
	mux.Handle("GET /api/tenants/{id}/license-server", RequireSession(authDeps.Sessions, GetLicenseServerHandler(licenseDeps)))
	mux.Handle("PUT /api/tenants/{id}/license-server", RequireSession(authDeps.Sessions, auth.RequireCSRF(UpdateLicenseServerHandler(licenseDeps))))
	mux.Handle("POST /api/tenants/{id}/license-server/checkup", RequireSession(authDeps.Sessions, auth.RequireCSRF(DisallowInDemoMode(licenseDeps.DemoMode, CheckupLicenseServerHandler()))))
	mux.Handle("POST /api/tenants/{id}/license/preview", RequireSession(authDeps.Sessions, auth.RequireCSRF(PreviewLicenseHandler())))
	mux.Handle("POST /api/tenants/{id}/license/push", RequireSession(authDeps.Sessions, auth.RequireCSRF(DisallowInDemoMode(licenseDeps.DemoMode, PushLicenseHandler(licenseDeps)))))
	mux.Handle("GET /api/tenants/{id}/license/history", RequireSession(authDeps.Sessions, LicenseHistoryHandler(licenseDeps)))

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
