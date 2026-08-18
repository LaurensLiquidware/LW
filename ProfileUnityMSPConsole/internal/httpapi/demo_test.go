package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"profileunity-msp-console/internal/auth"
)

func TestDisallowInDemoMode_BlocksWhenTrue(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	handler := DisallowInDemoMode(true, inner)
	req := httptest.NewRequest(http.MethodPost, "/whatever", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("inner handler was called, want it blocked in demo mode")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestDisallowInDemoMode_PassesThroughWhenFalse(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	handler := DisallowInDemoMode(false, inner)
	req := httptest.NewRequest(http.MethodPost, "/whatever", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("inner handler was not called, want it to run when not in demo mode")
	}
}

// newTestRouterDemo mirrors newTestRouter but wires demoMode into the
// AuthDeps/CollectionDeps fields the way cmd/server/main.go's openDatabase
// result would, so router-level demo-mode wiring (DisallowInDemoMode
// composed onto /api/collect/run and /api/tenants/test) is exercised
// end-to-end rather than just unit-tested in isolation above.
func newTestRouterDemo(t *testing.T) (http.Handler, testDeps) {
	t.Helper()
	deps := newTestDeps(t)
	deps.auth.DemoMode = true
	deps.collection.DemoMode = true
	router, err := NewRouter(func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} }, deps.auth, deps.tenants, deps.dashboard, deps.history, deps.reports, deps.alerts, deps.collection, deps.settings, deps.licenses)
	if err != nil {
		t.Fatal(err)
	}
	return router, deps
}

func TestNewRouter_DemoMode_BlocksCollectNow(t *testing.T) {
	router, deps := newTestRouterDemo(t)
	cookie := authenticatedSession(t, deps)

	req := httptest.NewRequest(http.MethodPost, "/api/collect/run", nil)
	req.AddCookie(cookie)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "tok"})
	req.Header.Set(auth.CSRFHeaderName, "tok")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (demo mode blocks Collect Now)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "demo mode") {
		t.Errorf("body = %q, want it to mention demo mode", rec.Body.String())
	}
}

func TestNewRouter_DemoMode_BlocksTestConnection(t *testing.T) {
	router, deps := newTestRouterDemo(t)
	cookie := authenticatedSession(t, deps)

	req := httptest.NewRequest(http.MethodPost, "/api/tenants/test", strings.NewReader(`{"hostname":"noordkaap.demo.example.com","port":8000}`))
	req.AddCookie(cookie)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "tok"})
	req.Header.Set(auth.CSRFHeaderName, "tok")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (demo mode blocks Test Connection) -- must never attempt a network call to a demo tenant's host", rec.Code)
	}
}

func TestNewRouter_DemoMode_SurfacedOnMe(t *testing.T) {
	router, deps := newTestRouterDemo(t)
	cookie := authenticatedSession(t, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"demoMode":true`) {
		t.Errorf("body = %q, want demoMode:true so the frontend can show the DEMO DATA badge", rec.Body.String())
	}
}

func TestNewRouter_NotDemoMode_CollectNowNotBlocked(t *testing.T) {
	router, deps := newTestRouter(t)
	cookie := authenticatedSession(t, deps)

	req := httptest.NewRequest(http.MethodPost, "/api/collect/run", nil)
	req.AddCookie(cookie)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "tok"})
	req.Header.Set(auth.CSRFHeaderName, "tok")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Fatalf("status = 403 outside demo mode -- Collect Now must not be blocked when not running against demo.db")
	}
}
