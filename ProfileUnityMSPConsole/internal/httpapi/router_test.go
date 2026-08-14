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
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

type testDeps struct {
	auth      AuthDeps
	tenants   TenantDeps
	dashboard DashboardDeps
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

	return testDeps{
		auth: AuthDeps{
			Users:    auth.NewUserRepo(sqlDB),
			Sessions: auth.NewSessionRepo(sqlDB, 30*time.Minute, 12*time.Hour),
			Secure:   false,
		},
		tenants: TenantDeps{Tenants: tenantRepo},
		dashboard: DashboardDeps{
			Repos:    dashboard.Repos{Tenants: tenantRepo, Snapshots: snapshotRepo},
			Location: time.UTC,
		},
	}
}

func newTestRouter(t *testing.T) (http.Handler, testDeps) {
	t.Helper()
	deps := newTestDeps(t)
	router, err := NewRouter(func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} }, deps.auth, deps.tenants, deps.dashboard)
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
