package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"profileunity-msp-console/internal/auth"
	"profileunity-msp-console/internal/dashboard"
	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/scheduler"
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

type testDeps struct {
	auth       AuthDeps
	tenants    TenantDeps
	dashboard  DashboardDeps
	history    HistoryDeps
	reports    ReportDeps
	alerts     AlertDeps
	collection CollectionDeps
}

func newTestDeps(t *testing.T) testDeps {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	tenantRepo := tenant.NewRepo(sqlDB, nil)
	snapshotRepo := snapshot.NewRepo(sqlDB)
	repos := dashboard.Repos{Tenants: tenantRepo, Snapshots: snapshotRepo}
	sched := scheduler.New(tenantRepo, snapshotRepo, time.Hour, time.UTC, 5, 30*time.Second)

	return testDeps{
		auth: AuthDeps{
			Users:    auth.NewUserRepo(sqlDB),
			Sessions: auth.NewSessionRepo(sqlDB, 30*time.Minute, 12*time.Hour),
			Secure:   false,
		},
		tenants:   TenantDeps{Tenants: tenantRepo},
		dashboard: DashboardDeps{Repos: repos, Location: time.UTC},
		history:   HistoryDeps{Repos: repos},
		reports:   ReportDeps{Repos: repos},
		alerts:    AlertDeps{Repos: repos, Location: time.UTC},
		collection: CollectionDeps{
			Scheduler: sched,
			Status:    func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} },
		},
	}
}

func newTestRouter(t *testing.T) (http.Handler, testDeps) {
	t.Helper()
	deps := newTestDeps(t)
	router, err := NewRouter(func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} }, deps.auth, deps.tenants, deps.dashboard, deps.history, deps.reports, deps.alerts, deps.collection)
	if err != nil {
		t.Fatal(err)
	}
	return router, deps
}

// authenticatedSession creates a user and returns a session cookie for it.
func authenticatedSession(t *testing.T, deps testDeps) *http.Cookie {
	t.Helper()
	u, err := deps.auth.Users.CreateUser(t.Context(), "tester", "correct-horse-battery-staple", auth.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	token, err := deps.auth.Sessions.Create(t.Context(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: SessionCookieName, Value: token}
}

func TestNewRouter_ServesLegalFiles(t *testing.T) {
	router, _ := newTestRouter(t)

	for _, path := range []string{"/Spark_License.pdf", "/bom.cdx.json", "/THIRD-PARTY-NOTICES.txt"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, rec.Code)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("%s: empty body", path)
		}
	}
}

func TestNewRouter_VersionEndpoint(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// TestNewRouter_SPAFallback covers the bug this caught: a client-side
// Angular route like /login has no matching file in the embedded dist
// and must fall back to index.html, not 404 — otherwise a hard refresh
// or direct link to any route but "/" breaks.
func TestNewRouter_SPAFallback(t *testing.T) {
	router, _ := newTestRouter(t)

	for _, path := range []string{"/", "/login", "/dashboard", "/tenants/some-id"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, rec.Code)
			continue
		}
		if !strings.Contains(rec.Body.String(), "<html") {
			t.Errorf("%s: body does not look like index.html: %q", path, rec.Body.String())
		}
	}
}

func TestNewRouter_RealStaticFileIsServedAsIs(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/assets/i18n/en.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "\"app\"") {
		t.Errorf("expected real en.json content, got %q", rec.Body.String())
	}
}

func TestNewRouter_TenantsRequireSession(t *testing.T) {
	router, _ := newTestRouter(t)
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/tenants", nil),
		httptest.NewRequest(http.MethodGet, "/api/dashboard", nil),
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", req.Method, req.URL.Path, rec.Code)
		}
	}
}

func TestNewRouter_TenantCRUD(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	// Create requires CSRF.
	createReq := httptest.NewRequest(http.MethodPost, "/api/tenants", strings.NewReader(`{"displayName":"Acme","hostname":"acme.example.com","port":8000,"enabled":true}`))
	createReq.AddCookie(cookie)
	createReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, createReq)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create without CSRF: status = %d, want 403", rec.Code)
	}

	createReq2 := httptest.NewRequest(http.MethodPost, "/api/tenants", strings.NewReader(`{"displayName":"Acme","hostname":"acme.example.com","port":8000,"enabled":true}`))
	createReq2.AddCookie(cookie)
	createReq2.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "tok"})
	createReq2.Header.Set("X-Requested-With", "XMLHttpRequest")
	createReq2.Header.Set(auth.CSRFHeaderName, "tok")
	createReq2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, createReq2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %s", rec2.Code, rec2.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	listReq.AddCookie(cookie)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: status = %d", listRec.Code)
	}
	if !strings.Contains(listRec.Body.String(), "Acme") {
		t.Errorf("list body missing created tenant: %s", listRec.Body.String())
	}
}

func TestNewRouter_DashboardEndpoint(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	if _, err := deps.tenants.Tenants.Create(t.Context(), tenant.CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "never_collected") {
		t.Errorf("expected never_collected data status, got %s", rec.Body.String())
	}
}

func TestNewRouter_CollectNowEndpoint(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	// No CSRF token -- rejected, same as any other mutating endpoint.
	noCSRFReq := httptest.NewRequest(http.MethodPost, "/api/collect/run", nil)
	noCSRFReq.AddCookie(cookie)
	noCSRFRec := httptest.NewRecorder()
	router.ServeHTTP(noCSRFRec, noCSRFReq)
	if noCSRFRec.Code != http.StatusForbidden {
		t.Fatalf("without CSRF: status = %d, want 403", noCSRFRec.Code)
	}

	// No enabled tenants, so the run completes instantly with no real
	// network calls -- this test exercises routing/auth/CSRF, not the
	// collector itself (covered in internal/collector and
	// internal/scheduler).
	req := httptest.NewRequest(http.MethodPost, "/api/collect/run", nil)
	req.AddCookie(cookie)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "tok"})
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set(auth.CSRFHeaderName, "tok")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"not_implemented"`) {
		t.Errorf("expected the Status callback's response echoed back, got %s", rec.Body.String())
	}
}

func TestNewRouter_CollectNowEndpoint_RequiresSession(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/collect/run", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestNewRouter_ChangePasswordEndpoint(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	// No CSRF token -- rejected, same as any other mutating endpoint.
	noCSRFReq := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"currentPassword":"correct-horse-battery-staple","newPassword":"new-correct-horse-battery"}`))
	noCSRFReq.AddCookie(cookie)
	noCSRFRec := httptest.NewRecorder()
	router.ServeHTTP(noCSRFRec, noCSRFReq)
	if noCSRFRec.Code != http.StatusForbidden {
		t.Fatalf("without CSRF: status = %d, want 403", noCSRFRec.Code)
	}

	// Wrong current password -- rejected, and the account keeps its
	// original password.
	wrongReq := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"currentPassword":"totally-wrong","newPassword":"new-correct-horse-battery"}`))
	wrongReq.AddCookie(cookie)
	wrongReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "tok"})
	wrongReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	wrongReq.Header.Set(auth.CSRFHeaderName, "tok")
	wrongRec := httptest.NewRecorder()
	router.ServeHTTP(wrongRec, wrongReq)
	if wrongRec.Code != http.StatusForbidden {
		t.Fatalf("wrong current password: status = %d, want 403, body = %s", wrongRec.Code, wrongRec.Body.String())
	}

	// Correct current password -- succeeds.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"currentPassword":"correct-horse-battery-staple","newPassword":"new-correct-horse-battery"}`))
	req.AddCookie(cookie)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "tok"})
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set(auth.CSRFHeaderName, "tok")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}

	// The new password now works for a fresh login.
	if _, err := deps.auth.Users.Authenticate(t.Context(), "tester", "new-correct-horse-battery"); err != nil {
		t.Errorf("new password does not authenticate: %v", err)
	}
}

func TestNewRouter_ChangePasswordEndpoint_RequiresSession(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"currentPassword":"x","newPassword":"y"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestNewRouter_AlertsEndpoint(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	// Never collected -- alertable (data_not_ok).
	if _, err := deps.tenants.Tenants.Create(t.Context(), tenant.CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	// Healthy -- not alertable.
	healthy, err := deps.tenants.Tenants.Create(t.Context(), tenant.CreateInput{DisplayName: "Healthy Co", Hostname: "h2", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.dashboard.Repos.Snapshots.Upsert(t.Context(), snapshot.Snapshot{
		TenantID: healthy.ID, CollectionDate: time.Now().UTC().Format("2006-01-02"), CollectedAtUTC: time.Now().UTC(),
		Status: snapshot.StatusSuccess, TotalLicenses: intPtrTest(10), UsedLicenses: intPtrTest(1),
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Acme") {
		t.Errorf("expected the never-collected tenant in the alerts response, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Healthy Co") {
		t.Errorf("healthy tenant should not appear in alerts, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "data_not_ok") {
		t.Errorf("expected data_not_ok reason, got %s", rec.Body.String())
	}
}

func TestNewRouter_AlertsEndpoint_RequiresSession(t *testing.T) {
	router, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestNewRouter_TenantHistoryEndpoint(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	tn, err := deps.tenants.Tenants.Create(t.Context(), tenant.CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.dashboard.Repos.Snapshots.Upsert(t.Context(), snapshot.Snapshot{
		TenantID: tn.ID, CollectionDate: "2026-08-14", CollectedAtUTC: time.Now().UTC(),
		Status: snapshot.StatusSuccess, TotalLicenses: intPtrTest(5), UsedLicenses: intPtrTest(1),
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/"+tn.ID+"/history", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "2026-08-14") {
		t.Errorf("expected the seeded point in the response, got %s", rec.Body.String())
	}
}

func TestNewRouter_TenantHistoryEndpoint_UnknownTenant(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/does-not-exist/history", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestNewRouter_PortfolioHistoryEndpoint(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	tn, err := deps.tenants.Tenants.Create(t.Context(), tenant.CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.dashboard.Repos.Snapshots.Upsert(t.Context(), snapshot.Snapshot{
		TenantID: tn.ID, CollectionDate: "2026-08-14", CollectedAtUTC: time.Now().UTC(),
		Status: snapshot.StatusSuccess, TotalLicenses: intPtrTest(5), UsedLicenses: intPtrTest(1),
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/history/portfolio", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"totalUsed\":1") {
		t.Errorf("expected totalUsed=1 in the response, got %s", rec.Body.String())
	}
}

func TestNewRouter_TenantMonthlyReportEndpoint(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	tn, err := deps.tenants.Tenants.Create(t.Context(), tenant.CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.dashboard.Repos.Snapshots.Upsert(t.Context(), snapshot.Snapshot{
		TenantID: tn.ID, CollectionDate: "2026-08-14", CollectedAtUTC: time.Now().UTC(),
		Status: snapshot.StatusSuccess, TotalLicenses: intPtrTest(5), UsedLicenses: intPtrTest(1),
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/"+tn.ID+"/reports/monthly?year=2026&month=8", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"daysCollected":1`) {
		t.Errorf("expected daysCollected=1 in the response, got %s", rec.Body.String())
	}
}

func TestNewRouter_TenantMonthlyReportEndpoint_UnknownTenant(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/does-not-exist/reports/monthly?year=2026&month=8", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestNewRouter_TenantMonthlyReportEndpoint_MissingMonth(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	tn, err := deps.tenants.Tenants.Create(t.Context(), tenant.CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/"+tn.ID+"/reports/monthly?year=2026", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestNewRouter_TenantMonthlyReportPDFEndpoint(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	tn, err := deps.tenants.Tenants.Create(t.Context(), tenant.CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.dashboard.Repos.Snapshots.Upsert(t.Context(), snapshot.Snapshot{
		TenantID: tn.ID, CollectionDate: "2026-08-14", CollectedAtUTC: time.Now().UTC(),
		Status: snapshot.StatusSuccess, TotalLicenses: intPtrTest(5), UsedLicenses: intPtrTest(1),
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/"+tn.ID+"/reports/monthly.pdf?year=2026&month=8", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	if !strings.HasPrefix(rec.Body.String(), "%PDF-") {
		t.Errorf("body does not start with a PDF header")
	}
}

func TestNewRouter_PortfolioMonthlyReportEndpoint(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	tn, err := deps.tenants.Tenants.Create(t.Context(), tenant.CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.dashboard.Repos.Snapshots.Upsert(t.Context(), snapshot.Snapshot{
		TenantID: tn.ID, CollectionDate: "2026-08-14", CollectedAtUTC: time.Now().UTC(),
		Status: snapshot.StatusSuccess, TotalLicenses: intPtrTest(5), UsedLicenses: intPtrTest(1),
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/reports/portfolio/monthly?year=2026&month=8", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"tenantsRegistered":1`) {
		t.Errorf("expected tenantsRegistered=1 in the response, got %s", rec.Body.String())
	}
}

func TestNewRouter_PortfolioMonthlyReportPDFEndpoint(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/reports/portfolio/monthly.pdf?year=2026&month=8", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(rec.Body.String(), "%PDF-") {
		t.Errorf("body does not start with a PDF header")
	}
}

func TestNewRouter_ReportEndpoints_RequireSession(t *testing.T) {
	router, _ := newTestRouter(t)

	for _, path := range []string{
		"/api/tenants/x/reports/monthly?year=2026&month=8",
		"/api/tenants/x/reports/monthly.pdf?year=2026&month=8",
		"/api/reports/portfolio/monthly?year=2026&month=8",
		"/api/reports/portfolio/monthly.pdf?year=2026&month=8",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", path, rec.Code)
		}
	}
}

func intPtrTest(v int) *int { return &v }
