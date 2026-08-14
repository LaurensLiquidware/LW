package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

// CSRFCookieName is the non-HttpOnly cookie carrying the CSRF token, so
// frontend JavaScript can read it and echo it back on mutating requests
// (the classic double-submit pattern). A cookie alone is not sufficient
// against CSRF — project brief §6 (carried over from the reference
// project) — so RequireCSRF also demands X-Requested-With.
const CSRFCookieName = "pumc_csrf"

// CSRFHeaderName is the header a mutating request must echo the cookie's
// value into.
const CSRFHeaderName = "X-CSRF-Token"

// NewCSRFToken generates a fresh random CSRF token value.
func NewCSRFToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// SetCSRFCookie issues (or re-issues) the CSRF cookie on the response.
func SetCSRFCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false, // must be readable by frontend JS to echo back
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// RequireCSRF wraps handler, rejecting any state-changing request unless
// it carries X-Requested-With and an X-CSRF-Token header matching the
// CSRF cookie. GET/HEAD/OPTIONS pass through untouched — they must not
// have side effects, so there is nothing to protect.
func RequireCSRF(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			handler.ServeHTTP(w, r)
			return
		}

		if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			http.Error(w, "missing X-Requested-With header", http.StatusForbidden)
			return
		}

		cookie, err := r.Cookie(CSRFCookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, "missing CSRF cookie", http.StatusForbidden)
			return
		}
		header := r.Header.Get(CSRFHeaderName)
		if header == "" || header != cookie.Value {
			http.Error(w, "CSRF token mismatch", http.StatusForbidden)
			return
		}

		handler.ServeHTTP(w, r)
	})
}
