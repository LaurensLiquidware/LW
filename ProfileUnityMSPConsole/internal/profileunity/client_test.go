package profileunity

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func serveFixture(t *testing.T, w http.ResponseWriter, filename string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("read fixture %s: %v", filename, err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func TestClient_GetLicenseInfoUnauthenticated_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/licenseinfo" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		serveFixture(t, w, "licenseinfo_success.json")
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	info, _, err := c.GetLicenseInfoUnauthenticated(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.TotalLicenses.Value != 5 || info.UsedLicenses.Value != 1 {
		t.Errorf("got TotalLicenses=%d UsedLicenses=%d, want 5 and 1", info.TotalLicenses.Value, info.UsedLicenses.Value)
	}
}

func TestClient_GetLicenseInfoUnauthenticated_ErrorTypeWithHTTP200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // the console's real defect: 200 on failure
		serveFixture(t, w, "licenseinfo_error_http200.json")
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = c.GetLicenseInfoUnauthenticated(context.Background())
	if err == nil {
		t.Fatal("expected an error despite HTTP 200")
	}
	if _, ok := err.(*APIError); !ok {
		t.Errorf("error type = %T, want *APIError", err)
	}
}

func TestClient_HTMLErrorPageIsMalformedPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		serveFixture(t, w, "html_error_page.html")
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = c.GetLicenseInfoUnauthenticated(context.Background())
	if _, ok := err.(*MalformedPayloadError); !ok {
		t.Fatalf("error type = %T (%v), want *MalformedPayloadError", err, err)
	}
}

func TestClient_ConnectionRefusedIsUnreachable(t *testing.T) {
	// Bind and immediately close a listener to get a real, but dead, address.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	c, err := New("http://" + addr)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = c.GetLicenseInfoUnauthenticated(context.Background())
	if _, ok := err.(*UnreachableError); !ok {
		t.Fatalf("error type = %T (%v), want *UnreachableError", err, err)
	}
}

func TestClient_TimeoutIsClassified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		serveFixture(t, w, "licenseinfo_success.json")
	}))
	defer srv.Close()

	c, err := New(srv.URL, WithTimeout(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = c.GetLicenseInfoUnauthenticated(context.Background())
	if _, ok := err.(*TimeoutError); !ok {
		t.Fatalf("error type = %T (%v), want *TimeoutError", err, err)
	}
}

func TestClient_TLSFailureWithoutSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "licenseinfo_success.json")
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = c.GetLicenseInfoUnauthenticated(context.Background())
	if _, ok := err.(*TLSError); !ok {
		t.Fatalf("error type = %T (%v), want *TLSError", err, err)
	}
}

func TestClient_TLSSucceedsWithSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "licenseinfo_success.json")
	}))
	defer srv.Close()

	c, err := New(srv.URL, WithInsecureSkipVerify(true))
	if err != nil {
		t.Fatal(err)
	}
	info, _, err := c.GetLicenseInfoUnauthenticated(context.Background())
	if err != nil {
		t.Fatalf("unexpected error with skip-verify: %v", err)
	}
	if info.TotalLicenses.Value != 5 {
		t.Errorf("TotalLicenses = %d, want 5", info.TotalLicenses.Value)
	}
}

func TestClient_AuthenticatedEndpointWithoutSessionIs401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetServerLicensing(context.Background())
	if _, ok := err.(*AuthRequiredError); !ok {
		t.Fatalf("error type = %T (%v), want *AuthRequiredError", err, err)
	}
}

func TestClient_Authenticate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/authenticate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", ct)
		}
		http.SetCookie(w, &http.Cookie{Name: "_ncfa", Value: "test-session", Path: "/"})
		serveFixture(t, w, "authenticate_success.json")
	}))
	defer srv.Close()

	c, err := New(srv.URL, WithCredentials("admin", "s3cr3t&pass=word"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Authenticate_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "authenticate_rejected.json")
	}))
	defer srv.Close()

	c, err := New(srv.URL, WithCredentials("admin", "wrong"))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Authenticate(context.Background())
	if _, ok := err.(*AuthRejectedError); !ok {
		t.Fatalf("error type = %T (%v), want *AuthRejectedError", err, err)
	}
}

// TestClient_CollectLicenseInfo_FallsBackToAuthenticated exercises the §4
// "attempt unauthenticated, fall back to authenticated" design: the first
// /licenseinfo call has no session and gets a 401; the client should
// authenticate and retry, and report AuthPathAuthenticated.
func TestClient_CollectLicenseInfo_FallsBackToAuthenticated(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/licenseinfo", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("_ncfa")
		if err != nil || cookie.Value != "test-session" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		serveFixture(t, w, "licenseinfo_success.json")
	})
	mux.HandleFunc("/authenticate", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "_ncfa", Value: "test-session", Path: "/"})
		serveFixture(t, w, "authenticate_success.json")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := New(srv.URL, WithCredentials("admin", "correct-password"))
	if err != nil {
		t.Fatal(err)
	}
	info, authPath, _, err := c.CollectLicenseInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authPath != AuthPathAuthenticated {
		t.Errorf("AuthPath = %q, want %q", authPath, AuthPathAuthenticated)
	}
	if info.TotalLicenses.Value != 5 {
		t.Errorf("TotalLicenses = %d, want 5", info.TotalLicenses.Value)
	}
}

func TestClient_CollectLicenseInfo_UnauthenticatedPathReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "licenseinfo_success.json")
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, authPath, _, err := c.CollectLicenseInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authPath != AuthPathUnauthenticated {
		t.Errorf("AuthPath = %q, want %q", authPath, AuthPathUnauthenticated)
	}
}

func TestClient_CollectLicenseInfo_NoCredentialsSurfacesOriginalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, err := New(srv.URL) // no WithCredentials
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = c.CollectLicenseInfo(context.Background())
	if _, ok := err.(*AuthRequiredError); !ok {
		t.Fatalf("error type = %T (%v), want *AuthRequiredError", err, err)
	}
}

func TestClient_GetServerLicensing_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/server/licensing" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		serveFixture(t, w, "server_licensing_success.json")
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	sl, err := c.GetServerLicensing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sl.Organization != "Liquidware Training EU" {
		t.Errorf("Organization = %q", sl.Organization)
	}
}

func TestClient_GetLicenseServers_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/licenseserver" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		serveFixture(t, w, "licenseserver_rows.json")
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := c.GetLicenseServers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0].ServerAddress != "10.0.0.5" {
		t.Errorf("got %+v", rows)
	}
}
