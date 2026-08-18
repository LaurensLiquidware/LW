package licensepush

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"profileunity-msp-console/internal/licenseserver"
)

const testLicenseBase64 = "PGxpY2Vuc2U+CiAgPG9yZ2FuaXphdGlvbj5BY21lPC9vcmdhbml6YXRpb24+CiAgPG1heFVzZXJzPjEwPC9tYXhVc2Vycz4KPC9saWNlbnNlPg=="

func TestPush_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"License": map[string]any{"Organization": "Acme"}})
	}))
	defer srv.Close()

	client, err := licenseserver.New(srv.URL, "prou_services", "secret")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Push(context.Background(), client, testLicenseBase64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != OutcomeSuccess {
		t.Errorf("Outcome = %q, want success", result.Outcome)
	}
	if result.Fields.Organization != "Acme" {
		t.Errorf("Fields.Organization = %q", result.Fields.Organization)
	}
}

func TestPush_AuthFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client, err := licenseserver.New(srv.URL, "prou_services", "wrong")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Push(context.Background(), client, testLicenseBase64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != OutcomeAuthFailed {
		t.Errorf("Outcome = %q, want auth_failed", result.Outcome)
	}
}

func TestPush_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		msg := "From Licenseserver:\r\nNEW: This license has already been added."
		_ = json.NewEncoder(w).Encode(map[string]any{"ErrorMsg": msg})
	}))
	defer srv.Close()

	client, err := licenseserver.New(srv.URL, "prou_services", "secret")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Push(context.Background(), client, testLicenseBase64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != OutcomeRejected {
		t.Errorf("Outcome = %q, want rejected", result.Outcome)
	}
}

func TestPush_InvalidLicenseCodeReturnsError(t *testing.T) {
	client, err := licenseserver.New("https://example.com", "prou_services", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Push(context.Background(), client, "not-valid-base64!!!"); err == nil {
		t.Fatal("expected an error for an undecodable license code")
	}
}
