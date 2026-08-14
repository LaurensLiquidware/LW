package collector

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTestConnection_UnauthenticatedSuccess(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(licenseInfoSuccessJSON))
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	outcome, msg := TestConnection(context.Background(), TestConnectionParams{
		Hostname: tn.Hostname, Port: tn.Port, TLSSkipVerify: true,
	})
	if outcome != ConnUnauthenticatedSuccess {
		t.Fatalf("outcome = %q, want unauthenticated_success (msg: %s)", outcome, msg)
	}
}

func TestTestConnection_AuthenticatedSuccess(t *testing.T) {
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
	outcome, _ := TestConnection(context.Background(), TestConnectionParams{
		Hostname: tn.Hostname, Port: tn.Port, TLSSkipVerify: true, Username: "admin", Password: "correct",
	})
	if outcome != ConnAuthenticatedSuccess {
		t.Fatalf("outcome = %q, want authenticated_success", outcome)
	}
}

func TestTestConnection_TLSFailureWithoutSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(licenseInfoSuccessJSON))
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, false)
	outcome, _ := TestConnection(context.Background(), TestConnectionParams{Hostname: tn.Hostname, Port: tn.Port})
	if outcome != ConnTLSFailure {
		t.Fatalf("outcome = %q, want tls_failure", outcome)
	}
}

func TestTestConnection_AuthRequiredWithoutCredentials(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	outcome, _ := TestConnection(context.Background(), TestConnectionParams{Hostname: tn.Hostname, Port: tn.Port, TLSSkipVerify: true})
	if outcome != ConnAuthRequired {
		t.Fatalf("outcome = %q, want auth_required", outcome)
	}
}

func TestTestConnection_AuthRejected(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/licenseinfo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	mux.HandleFunc("/authenticate", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"WebMessageType":2,"Type":"error","Message":"Incorrect Username or Password","MessageKey":null,"Tag":null}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	outcome, _ := TestConnection(context.Background(), TestConnectionParams{
		Hostname: tn.Hostname, Port: tn.Port, TLSSkipVerify: true, Username: "admin", Password: "wrong",
	})
	if outcome != ConnAuthRejected {
		t.Fatalf("outcome = %q, want auth_rejected", outcome)
	}
}

func TestTestConnection_Unreachable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().(*net.TCPAddr)
	l.Close()

	outcome, _ := TestConnection(context.Background(), TestConnectionParams{Hostname: "127.0.0.1", Port: addr.Port})
	if outcome != ConnUnreachable {
		t.Fatalf("outcome = %q, want unreachable", outcome)
	}
}

func TestTestConnection_MalformedResponse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("<html>error</html>"))
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	outcome, _ := TestConnection(context.Background(), TestConnectionParams{Hostname: tn.Hostname, Port: tn.Port, TLSSkipVerify: true})
	if outcome != ConnMalformedResponse {
		t.Fatalf("outcome = %q, want malformed_response", outcome)
	}
}
