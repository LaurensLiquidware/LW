package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"profileunity-msp-console/internal/auth"
	"profileunity-msp-console/internal/db"
)

func newTestAuthDeps(t *testing.T) AuthDeps {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return AuthDeps{
		Users:    auth.NewUserRepo(sqlDB),
		Sessions: auth.NewSessionRepo(sqlDB, 30*60*1e9, 12*60*60*1e9),
		Secure:   false,
	}
}

func TestNewRouter_ServesLegalFiles(t *testing.T) {
	router, err := NewRouter(func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} }, newTestAuthDeps(t))
	if err != nil {
		t.Fatal(err)
	}

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
	router, err := NewRouter(func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} }, newTestAuthDeps(t))
	if err != nil {
		t.Fatal(err)
	}
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
	router, err := NewRouter(func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} }, newTestAuthDeps(t))
	if err != nil {
		t.Fatal(err)
	}

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
	router, err := NewRouter(func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} }, newTestAuthDeps(t))
	if err != nil {
		t.Fatal(err)
	}
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
