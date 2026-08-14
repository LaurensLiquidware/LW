package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"profileunity-msp-console/internal/tlscert"
)

// generateTestCertPEM generates a fresh self-signed cert/key pair (via
// tlscert.EnsureSelfSigned into a temp dir) and returns its PEM bytes —
// a real, valid, matching pair for exercising the upload handler.
func generateTestCertPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if _, err := tlscert.EnsureSelfSigned(certPath, keyPath, []string{"localhost"}); err != nil {
		t.Fatalf("EnsureSelfSigned: %v", err)
	}
	cert, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key
}

// TestGetSettingsHandler_ReturnsSeededDefaultsWithoutLeakingAPassword
// covers both the happy path and the one thing this endpoint must never
// do: echo back a stored SMTP password, even though none is set here.
func TestGetSettingsHandler_ReturnsSeededDefaultsWithoutLeakingAPassword(t *testing.T) {
	deps := newTestDeps(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()
	GetSettingsHandler(deps.settings)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var dto settingsDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if dto.CollectionIntervalSeconds != 3600 {
		t.Errorf("CollectionIntervalSeconds = %d, want 3600 (seeded 1h)", dto.CollectionIntervalSeconds)
	}
	if dto.SMTPPasswordSet {
		t.Error("SMTPPasswordSet = true when no password was ever set")
	}
	if rec.Body.String() != "" && bytes.Contains(rec.Body.Bytes(), []byte("smtpPassword\":\"")) {
		t.Error("response body contains a raw smtpPassword field")
	}
}

func settingsPutRequest(body any) *http.Request {
	b, _ := json.Marshal(body)
	return httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(b))
}

func TestUpdateSettingsHandler_PersistsAndAppliesLive(t *testing.T) {
	deps := newTestDeps(t)

	pw := "s3cret-app-password"
	body := settingsWriteRequest{
		SMTPHost:                       "smtp.example.com",
		SMTPPort:                       587,
		SMTPUsername:                   "svc",
		SMTPPassword:                   &pw,
		SMTPFrom:                       "reports@example.com",
		SMTPSecurity:                   "starttls",
		ReportRecipients:               []string{"msp@liquidware.eu"},
		ReportEmailDay:                 5,
		CollectionIntervalSeconds:      1800,
		CollectionTimezone:             "UTC",
		CollectionConcurrency:          8,
		CollectionTenantTimeoutSeconds: 20,
		SessionIdleTimeoutSeconds:      900,
		SessionAbsoluteTimeoutSeconds:  3600,
	}

	rec := httptest.NewRecorder()
	UpdateSettingsHandler(deps.settings)(rec, settingsPutRequest(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}

	var dto settingsDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !dto.SMTPPasswordSet {
		t.Error("SMTPPasswordSet = false right after setting a password")
	}
	if dto.CollectionConcurrency != 8 {
		t.Errorf("CollectionConcurrency = %d, want 8", dto.CollectionConcurrency)
	}

	// Persisted:
	stored, ok, err := deps.settings.Store.Load(context.Background())
	if err != nil || !ok {
		t.Fatalf("Load after update: ok=%v err=%v", ok, err)
	}
	if stored.SMTPHost != "smtp.example.com" || stored.CollectionConcurrency != 8 {
		t.Errorf("stored settings weren't persisted correctly: %+v", stored)
	}

	// Applied live -- the running scheduler must reflect the new
	// concurrency/interval without needing a restart. There's no public
	// getter, so this is verified indirectly via the session timeout
	// (which does have one) and the reportmail config (verified in its
	// own package's tests) -- here we confirm at least that the call
	// didn't panic and the session timeout propagated.
	if deps.settings.Sessions.IdleTimeout() != 900*time.Second {
		t.Errorf("SessionRepo.IdleTimeout() = %s after update, want 15m", deps.settings.Sessions.IdleTimeout())
	}
}

func TestUpdateSettingsHandler_UnchangedPasswordWhenOmitted(t *testing.T) {
	deps := newTestDeps(t)

	pw := "first-password"
	first := settingsWriteRequest{
		SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPPassword: &pw, SMTPFrom: "reports@example.com",
		SMTPSecurity: "starttls", ReportRecipients: []string{"msp@liquidware.eu"}, ReportEmailDay: 1,
		CollectionIntervalSeconds: 3600, CollectionTimezone: "UTC", CollectionConcurrency: 5, CollectionTenantTimeoutSeconds: 30,
		SessionIdleTimeoutSeconds: 1800, SessionAbsoluteTimeoutSeconds: 43200,
	}
	rec := httptest.NewRecorder()
	UpdateSettingsHandler(deps.settings)(rec, settingsPutRequest(first))
	if rec.Code != http.StatusOK {
		t.Fatalf("first update status = %d, body: %s", rec.Code, rec.Body.String())
	}

	// Second update omits SMTPPassword entirely (nil) -- the stored
	// password must survive untouched.
	second := first
	second.SMTPPassword = nil
	second.ReportEmailDay = 10
	rec2 := httptest.NewRecorder()
	UpdateSettingsHandler(deps.settings)(rec2, settingsPutRequest(second))
	if rec2.Code != http.StatusOK {
		t.Fatalf("second update status = %d, body: %s", rec2.Code, rec2.Body.String())
	}

	stored, ok, err := deps.settings.Store.Load(context.Background())
	if err != nil || !ok {
		t.Fatalf("Load: ok=%v err=%v", ok, err)
	}
	if stored.SMTPPassword != "first-password" {
		t.Errorf("SMTPPassword = %q, want it left untouched by the second update", stored.SMTPPassword)
	}
	if stored.ReportEmailDay != 10 {
		t.Errorf("ReportEmailDay = %d, want 10 from the second update", stored.ReportEmailDay)
	}
}

func TestUpdateSettingsHandler_RejectsInvalidSettings(t *testing.T) {
	deps := newTestDeps(t)

	body := settingsWriteRequest{
		SMTPHost:           "smtp.example.com", // host set...
		SMTPFrom:           "",                 // ...but from address missing: invalid.
		ReportRecipients:   []string{"msp@liquidware.eu"},
		CollectionTimezone: "UTC", CollectionConcurrency: 5, CollectionTenantTimeoutSeconds: 30,
		CollectionIntervalSeconds: 3600, SessionIdleTimeoutSeconds: 1800, SessionAbsoluteTimeoutSeconds: 43200,
	}
	rec := httptest.NewRecorder()
	UpdateSettingsHandler(deps.settings)(rec, settingsPutRequest(body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an SMTP host without a from address", rec.Code)
	}
}

func TestUploadTLSCertHandler_HotSwapsAndPersists(t *testing.T) {
	deps := newTestDeps(t)

	certPEM, keyPEM := generateTestCertPEM(t)
	body := tlsCertUploadRequest{CertPEM: string(certPEM), KeyPEM: string(keyPEM)}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/tls-cert", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	UploadTLSCertHandler(deps.settings)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}

	var dto settingsDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !dto.TLSCertConfigured {
		t.Error("TLSCertConfigured = false right after a successful upload")
	}

	stored, ok, err := deps.settings.Store.Load(context.Background())
	if err != nil || !ok {
		t.Fatalf("Load: ok=%v err=%v", ok, err)
	}
	if stored.TLSCertPEM == "" || stored.TLSKeyPEM == "" {
		t.Error("uploaded certificate/key were not persisted")
	}
}

func TestUploadTLSCertHandler_RejectsMismatchedPair(t *testing.T) {
	deps := newTestDeps(t)

	certA, _ := generateTestCertPEM(t)
	_, keyB := generateTestCertPEM(t)
	body := tlsCertUploadRequest{CertPEM: string(certA), KeyPEM: string(keyB)}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/tls-cert", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	UploadTLSCertHandler(deps.settings)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a mismatched cert/key pair", rec.Code)
	}
}
