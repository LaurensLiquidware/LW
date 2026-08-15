package collector

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

const licenseInfoSuccessJSON = `{ "WebMessageType": 2, "Type": "success", "Message": "", "MessageKey": null, "Tag": [ {
	"RegisteredTo": "Liquidware Training EU", "LicenseMode": "NamedUser", "LicenseProduct": "ProU+FlexApp",
	"SupportEnds": "12/31/2026", "TotalLicenses": "5", "UsedLicenses": "1", "Evaluation": "Yes",
	"ConsoleVersion": "6.9.5.9678 3038806 2026-07-01", "IsTrialExpired": "false", "IsTrial": "false",
	"IsProUOnly": "false", "IsFlexOnly": "false"
} ] }`

func tenantForServer(t *testing.T, srv *httptest.Server, tlsSkipVerify bool) tenant.Tenant {
	t.Helper()
	u := strings.TrimPrefix(strings.TrimPrefix(srv.URL, "https://"), "http://")
	host, portStr, err := net.SplitHostPort(u)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	return tenant.Tenant{ID: "t1", DisplayName: "Test Tenant", Hostname: host, Port: port, TLSSkipVerify: tlsSkipVerify, Enabled: true}
}

func TestCollectOne_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(licenseInfoSuccessJSON))
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	s := CollectOne(context.Background(), tn, nil, now, time.UTC)

	if s.Status != snapshot.StatusSuccess {
		t.Fatalf("Status = %q, want success (error: %s)", s.Status, s.ErrorMessage)
	}
	if s.TotalLicenses == nil || *s.TotalLicenses != 5 {
		t.Errorf("TotalLicenses = %v, want 5", s.TotalLicenses)
	}
	if s.UsedLicenses == nil || *s.UsedLicenses != 1 {
		t.Errorf("UsedLicenses = %v, want 1", s.UsedLicenses)
	}
	if s.AuthPath != "unauthenticated" {
		t.Errorf("AuthPath = %q, want unauthenticated", s.AuthPath)
	}
	if s.CollectionDate != "2026-08-14" {
		t.Errorf("CollectionDate = %q, want 2026-08-14", s.CollectionDate)
	}
	if s.RawPayload == "" {
		t.Error("expected the raw payload to be retained")
	}
	if s.Evaluation == nil || !*s.Evaluation {
		t.Errorf("Evaluation = %v, want true", s.Evaluation)
	}
}

func TestCollectOne_Unreachable(t *testing.T) {
	// A closed listener gives a real, dead address.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().(*net.TCPAddr)
	l.Close()

	tn := tenant.Tenant{ID: "t1", Hostname: "127.0.0.1", Port: addr.Port, Enabled: true}
	s := CollectOne(context.Background(), tn, nil, time.Now(), time.UTC)

	if s.Status != snapshot.StatusUnreachable {
		t.Fatalf("Status = %q, want unreachable (message: %s)", s.Status, s.ErrorMessage)
	}
	if s.TotalLicenses != nil || s.UsedLicenses != nil {
		t.Error("a failed poll must not carry license figures")
	}
}

func TestCollectOne_MalformedResponse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("<html><body>500</body></html>"))
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	s := CollectOne(context.Background(), tn, nil, time.Now(), time.UTC)

	if s.Status != snapshot.StatusMalformed {
		t.Fatalf("Status = %q, want malformed", s.Status)
	}
}

func TestCollectOne_AuthRequiredWithoutCredentials(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	s := CollectOne(context.Background(), tn, nil, time.Now(), time.UTC)

	if s.Status != snapshot.StatusAuthRequired {
		t.Fatalf("Status = %q, want auth_required", s.Status)
	}
}

func TestCollectOne_FallsBackToAuthenticatedWithCredentials(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/licenseinfo", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("_ncfa")
		if err != nil || cookie.Value != "session" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(licenseInfoSuccessJSON))
	})
	mux.HandleFunc("/authenticate", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "_ncfa", Value: "session", Path: "/"})
		w.Write([]byte(`{"WebMessageType":2,"Type":"success","Message":"","MessageKey":null,"Tag":null}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	creds := &tenant.Credentials{Username: "admin", Password: "correct"}
	s := CollectOne(context.Background(), tn, creds, time.Now(), time.UTC)

	if s.Status != snapshot.StatusSuccess {
		t.Fatalf("Status = %q, want success (message: %s)", s.Status, s.ErrorMessage)
	}
	if s.AuthPath != "authenticated" {
		t.Errorf("AuthPath = %q, want authenticated", s.AuthPath)
	}
}

// TestCollectOne_RetriesTransientFailureThenSucceeds forces the first two
// requests to fail at the connection level (simulating a flaky console)
// and the third to succeed, and checks CollectOne still reports success —
// exercising the retry-with-backoff path for transient errors.
func TestCollectOne_RetriesTransientFailureThenSucceeds(t *testing.T) {
	var attempts int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("ResponseWriter does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatal(err)
			}
			conn.Close()
			return
		}
		w.Write([]byte(licenseInfoSuccessJSON))
	}))
	srv.StartTLS()
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s := CollectOne(ctx, tn, nil, time.Now(), time.UTC)

	if s.Status != snapshot.StatusSuccess {
		t.Fatalf("Status = %q, want success after retries (message: %s)", s.Status, s.ErrorMessage)
	}
	if atomic.LoadInt32(&attempts) < 3 {
		t.Errorf("attempts = %d, want at least 3", attempts)
	}
}

func TestCollectOne_DoesNotRetryAuthRejection(t *testing.T) {
	var attempts int32
	mux := http.NewServeMux()
	mux.HandleFunc("/licenseinfo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	mux.HandleFunc("/authenticate", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.Write([]byte(`{"WebMessageType":2,"Type":"error","Message":"Incorrect Username or Password","MessageKey":null,"Tag":null}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	creds := &tenant.Credentials{Username: "admin", Password: "wrong"}
	s := CollectOne(context.Background(), tn, creds, time.Now(), time.UTC)

	if s.Status != snapshot.StatusAuthRejected {
		t.Fatalf("Status = %q, want auth_rejected", s.Status)
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("authenticate was called %d times, want exactly 1 (rejection must not be retried)", attempts)
	}
}
