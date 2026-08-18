package licenseserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddLicenseEncoded_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "prou_services" || pass != "secret" {
			t.Errorf("unexpected credentials: user=%q pass=%q ok=%v", user, pass, ok)
		}
		_ = json.NewEncoder(w).Encode(AddLicenseResponse{License: &LicenseInfo{Organization: "Acme"}})
	}))
	defer srv.Close()

	c, err := New(srv.URL, "prou_services", "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out, err := c.AddLicenseEncoded(context.Background(), "dGVzdA==")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.License == nil || out.License.Organization != "Acme" {
		t.Errorf("unexpected response: %+v", out)
	}
}

func TestAddLicenseEncoded_ErrorMsgIsFailureEvenOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		msg := "From Licenseserver:\r\nNEW: This license has already been added."
		_ = json.NewEncoder(w).Encode(AddLicenseResponse{ErrorMsg: &msg})
	}))
	defer srv.Close()

	c, err := New(srv.URL, "prou_services", "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.AddLicenseEncoded(context.Background(), "dGVzdA=="); err == nil {
		t.Fatal("expected an error for a populated ErrorMsg despite HTTP 200")
	}
}

func TestAddLicenseEncoded_401IsAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, err := New(srv.URL, "prou_services", "wrong")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.AddLicenseEncoded(context.Background(), "dGVzdA=="); err == nil {
		t.Fatal("expected an error on 401")
	}
}

func TestAddLicenseEncoded_RefusesEmptyCredentials(t *testing.T) {
	c, err := New("https://example.com", "", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.AddLicenseEncoded(context.Background(), "dGVzdA=="); err == nil {
		t.Fatal("expected an error for empty credentials")
	}
}

func TestCheckup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"IsWorking": true, "Message": "ok"})
	}))
	defer srv.Close()

	c, err := New(srv.URL, "", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ok, msg, err := c.Checkup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || msg != "ok" {
		t.Errorf("Checkup = (%v, %q)", ok, msg)
	}
}
