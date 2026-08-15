package collector

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if strings.Contains(msg, "Warning") {
		t.Errorf("message = %q, want no warning for a normal healthy license fixture", msg)
	}
}

// licenseInfoJSONWith returns a copy of the success fixture with one
// field's raw string value replaced, for exercising licenseWarnings'
// individual checks without a separate fixture file per case.
func licenseInfoJSONWith(field, value string) string {
	replacements := map[string]string{
		"TotalLicenses":  `"TotalLicenses": "5"`,
		"LicenseProduct": `"LicenseProduct": "ProU+FlexApp"`,
		"IsTrialExpired": `"IsTrialExpired": "false"`,
	}
	replacements[field] = fmt.Sprintf(`"%s": %s`, field, value)
	return fmt.Sprintf(`{ "WebMessageType": 2, "Type": "success", "Message": "", "MessageKey": null, "Tag": [ {
		"RegisteredTo": "Liquidware Training EU", "LicenseMode": "NamedUser", %s,
		"SupportEnds": "12/31/2026", %s, "UsedLicenses": "1", "Evaluation": "Yes",
		"ConsoleVersion": "6.9.5.9678 3038806 2026-07-01", %s, "IsTrial": "false",
		"IsProUOnly": "false", "IsFlexOnly": "false"
	} ] }`, replacements["LicenseProduct"], replacements["TotalLicenses"], replacements["IsTrialExpired"])
}

func TestTestConnection_WarnsWhenNoLicensedSeats(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(licenseInfoJSONWith("TotalLicenses", `"0"`)))
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	outcome, msg := TestConnection(context.Background(), TestConnectionParams{Hostname: tn.Hostname, Port: tn.Port, TLSSkipVerify: true})
	if outcome != ConnUnauthenticatedSuccess {
		t.Fatalf("outcome = %q, want unauthenticated_success (still a success -- this is advisory)", outcome)
	}
	if !strings.Contains(msg, "no licensed seats") {
		t.Errorf("message = %q, want it to mention no licensed seats", msg)
	}
}

func TestTestConnection_WarnsWhenTotalLicensesUnparseable(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(licenseInfoJSONWith("TotalLicenses", `"not-a-number"`)))
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	_, msg := TestConnection(context.Background(), TestConnectionParams{Hostname: tn.Hostname, Port: tn.Port, TLSSkipVerify: true})
	if !strings.Contains(msg, "no licensed seats") {
		t.Errorf("message = %q, want it to mention no licensed seats when TotalLicenses doesn't parse", msg)
	}
}

func TestTestConnection_WarnsWhenNoLicenseProduct(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(licenseInfoJSONWith("LicenseProduct", `""`)))
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	_, msg := TestConnection(context.Background(), TestConnectionParams{Hostname: tn.Hostname, Port: tn.Port, TLSSkipVerify: true})
	if !strings.Contains(msg, "did not report a license product") {
		t.Errorf("message = %q, want it to mention the missing license product", msg)
	}
}

func TestTestConnection_WarnsWhenTrialExpired(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(licenseInfoJSONWith("IsTrialExpired", `"true"`)))
	}))
	defer srv.Close()

	tn := tenantForServer(t, srv, true)
	_, msg := TestConnection(context.Background(), TestConnectionParams{Hostname: tn.Hostname, Port: tn.Port, TLSSkipVerify: true})
	if !strings.Contains(msg, "trial has already expired") {
		t.Errorf("message = %q, want it to mention the expired trial", msg)
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
