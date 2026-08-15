package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireCSRF_AllowsGetWithoutToken(t *testing.T) {
	handler := RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET status = %d, want 200", rec.Code)
	}
}

func TestRequireCSRF_RejectsPostWithoutHeaders(t *testing.T) {
	handler := RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestRequireCSRF_RejectsMismatchedToken(t *testing.T) {
	handler := RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set(CSRFHeaderName, "attacker-guessed-token")
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "real-token"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestRequireCSRF_AllowsMatchingToken(t *testing.T) {
	handler := RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set(CSRFHeaderName, "matching-token")
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "matching-token"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestNewCSRFToken_IsRandomAndNonEmpty(t *testing.T) {
	a, err := NewCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == "" || b == "" || a == b {
		t.Errorf("expected distinct non-empty tokens, got %q and %q", a, b)
	}
}
