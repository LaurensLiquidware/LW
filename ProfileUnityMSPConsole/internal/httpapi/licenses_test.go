package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"profileunity-msp-console/internal/auth"
	"profileunity-msp-console/internal/tenant"
)

const testLicenseBase64 = "PGxpY2Vuc2U+CiAgPG9yZ2FuaXphdGlvbj5BY21lPC9vcmdhbml6YXRpb24+CiAgPG1heFVzZXJzPjEwPC9tYXhVc2Vycz4KPC9saWNlbnNlPg=="

func createTestTenant(t *testing.T, deps testDeps) tenant.Tenant {
	t.Helper()
	tn, err := deps.tenants.Tenants.Create(context.Background(), tenant.CreateInput{
		DisplayName: "Acme", Hostname: "acme.example.com", Port: 8000, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tn
}

func TestGetLicenseServerHandler_EmptyByDefault(t *testing.T) {
	deps := newTestDeps(t)
	tn := createTestTenant(t, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/"+tn.ID+"/license-server", nil)
	req.SetPathValue("id", tn.ID)
	rec := httptest.NewRecorder()
	GetLicenseServerHandler(deps.licenses)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var dto licenseServerDTO
	if err := json.NewDecoder(rec.Body).Decode(&dto); err != nil {
		t.Fatal(err)
	}
	if dto.Hostname != "" || dto.HasPassword {
		t.Errorf("got %+v, want empty connection", dto)
	}
}

func TestUpdateLicenseServerHandler_SavesAndDoesNotLeakPassword(t *testing.T) {
	deps := newTestDeps(t)
	tn := createTestTenant(t, deps)

	body, _ := json.Marshal(licenseServerWriteRequest{
		Hostname: "ld-lw01.example.com", Port: 443, Username: "prou_services",
		Password: strPtr("secret"), TLSSkipVerify: true,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/tenants/"+tn.ID+"/license-server", bytes.NewReader(body))
	req.SetPathValue("id", tn.ID)
	rec := httptest.NewRecorder()
	UpdateLicenseServerHandler(deps.licenses)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("response leaked the password: %s", rec.Body.String())
	}

	var dto licenseServerDTO
	if err := json.NewDecoder(rec.Body).Decode(&dto); err != nil {
		t.Fatal(err)
	}
	if dto.Hostname != "ld-lw01.example.com" || !dto.HasPassword || !dto.TLSSkipVerify {
		t.Errorf("got %+v", dto)
	}
}

func TestPreviewLicenseHandler_DecodesLocally(t *testing.T) {
	body, _ := json.Marshal(licensePreviewRequest{LicenseBase64: testLicenseBase64})
	req := httptest.NewRequest(http.MethodPost, "/api/tenants/x/license/preview", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	PreviewLicenseHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var dto decodedLicenseDTO
	if err := json.NewDecoder(rec.Body).Decode(&dto); err != nil {
		t.Fatal(err)
	}
	if dto.Organization != "Acme" || dto.MaxUsers != 10 {
		t.Errorf("got %+v", dto)
	}
}

func TestPreviewLicenseHandler_RejectsInvalidBase64(t *testing.T) {
	body, _ := json.Marshal(licensePreviewRequest{LicenseBase64: "not-valid!!!"})
	req := httptest.NewRequest(http.MethodPost, "/api/tenants/x/license/preview", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	PreviewLicenseHandler()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestPushLicenseHandler_RequiresConfirm(t *testing.T) {
	deps := newTestDeps(t)
	tn := createTestTenant(t, deps)

	body, _ := json.Marshal(licensePushRequest{LicenseBase64: testLicenseBase64, Confirm: false})
	req := httptest.NewRequest(http.MethodPost, "/api/tenants/"+tn.ID+"/license/push", bytes.NewReader(body)).WithContext(withUser(context.Background(), "op"))
	req.SetPathValue("id", tn.ID)
	rec := httptest.NewRecorder()
	PushLicenseHandler(deps.licenses)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when confirm is false", rec.Code)
	}
}

func TestPushLicenseHandler_NotConfiguredYet(t *testing.T) {
	deps := newTestDeps(t)
	tn := createTestTenant(t, deps)

	body, _ := json.Marshal(licensePushRequest{LicenseBase64: testLicenseBase64, Confirm: true})
	req := httptest.NewRequest(http.MethodPost, "/api/tenants/"+tn.ID+"/license/push", bytes.NewReader(body)).WithContext(withUser(context.Background(), "op"))
	req.SetPathValue("id", tn.ID)
	rec := httptest.NewRecorder()
	PushLicenseHandler(deps.licenses)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when no License Server connection is configured, body: %s", rec.Code, rec.Body.String())
	}
}

func TestPushLicenseHandler_SuccessIsRecordedInHistory(t *testing.T) {
	deps := newTestDeps(t)
	tn := createTestTenant(t, deps)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"License": map[string]any{"Organization": "Acme"}})
	}))
	defer srv.Close()

	host, portStr := splitHostPort(t, srv.URL)
	saveLicenseServer(t, deps, tn.ID, host, portStr)

	body, _ := json.Marshal(licensePushRequest{LicenseBase64: testLicenseBase64, Confirm: true})
	req := httptest.NewRequest(http.MethodPost, "/api/tenants/"+tn.ID+"/license/push", bytes.NewReader(body)).WithContext(withUser(context.Background(), "op-id"))
	req.SetPathValue("id", tn.ID)
	rec := httptest.NewRecorder()
	PushLicenseHandler(deps.licenses)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var resp licensePushResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Outcome != "success" {
		t.Errorf("Outcome = %q, want success", resp.Outcome)
	}

	histReq := httptest.NewRequest(http.MethodGet, "/api/tenants/"+tn.ID+"/license/history", nil)
	histReq.SetPathValue("id", tn.ID)
	histRec := httptest.NewRecorder()
	LicenseHistoryHandler(deps.licenses)(histRec, histReq)

	var history []licensePushRecordDTO
	if err := json.NewDecoder(histRec.Body).Decode(&history); err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Outcome != "success" || history[0].LicenseCodeBase64 != testLicenseBase64 {
		t.Errorf("got %+v", history)
	}
}

func TestCheckupLicenseServerHandler_RequiresHostname(t *testing.T) {
	body, _ := json.Marshal(licenseServerCheckupRequest{Port: 443})
	req := httptest.NewRequest(http.MethodPost, "/api/tenants/x/license-server/checkup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	CheckupLicenseServerHandler()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body: %s", rec.Code, rec.Body.String())
	}
}

// TestCheckupLicenseServerHandler_WorksWithoutSavingFirst is the
// regression case for a real bug report: Checkup was disabled until a
// connection had been saved, even though the server's own /api/checkup
// is unauthenticated and needs no saved credential -- so it should work
// straight from the live form, same as Test Connection on the Tenants
// screen.
func TestCheckupLicenseServerHandler_WorksWithoutSavingFirst(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"IsWorking": true, "Message": "ok"})
	}))
	defer srv.Close()
	host, portStr := splitHostPort(t, srv.URL)
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(licenseServerCheckupRequest{Hostname: host, Port: port, TLSSkipVerify: true})
	req := httptest.NewRequest(http.MethodPost, "/api/tenants/x/license-server/checkup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	CheckupLicenseServerHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var resp licenseServerCheckupResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Ok || resp.Message != "ok" {
		t.Errorf("got %+v", resp)
	}
}

// --- demo-mode blocking, exercised through the full router (matches how
// Test Connection/Collect Now's demo-mode blocking is already tested) ---

func TestNewRouter_DemoMode_BlocksLicenseServerCheckup(t *testing.T) {
	deps := newTestDeps(t)
	deps.licenses.DemoMode = true
	router, err := NewRouter(func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} },
		deps.auth, deps.tenants, deps.dashboard, deps.history, deps.reports, deps.alerts, deps.collection, deps.settings, deps.licenses)
	if err != nil {
		t.Fatal(err)
	}
	tn := createTestTenant(t, deps)
	cookie := authenticatedSession(t, deps)

	req := httptest.NewRequest(http.MethodPost, "/api/tenants/"+tn.ID+"/license-server/checkup", nil)
	req.AddCookie(cookie)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "tok"})
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set(auth.CSRFHeaderName, "tok")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 in demo mode, body: %s", rec.Code, rec.Body.String())
	}
}

func TestNewRouter_DemoMode_BlocksLicensePush(t *testing.T) {
	deps := newTestDeps(t)
	deps.licenses.DemoMode = true
	router, err := NewRouter(func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} },
		deps.auth, deps.tenants, deps.dashboard, deps.history, deps.reports, deps.alerts, deps.collection, deps.settings, deps.licenses)
	if err != nil {
		t.Fatal(err)
	}
	tn := createTestTenant(t, deps)
	cookie := authenticatedSession(t, deps)

	body, _ := json.Marshal(licensePushRequest{LicenseBase64: testLicenseBase64, Confirm: true})
	req := httptest.NewRequest(http.MethodPost, "/api/tenants/"+tn.ID+"/license/push", bytes.NewReader(body))
	req.AddCookie(cookie)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "tok"})
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set(auth.CSRFHeaderName, "tok")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 in demo mode, body: %s", rec.Code, rec.Body.String())
	}
}

func TestNewRouter_DemoMode_PreviewStillWorks(t *testing.T) {
	deps := newTestDeps(t)
	deps.licenses.DemoMode = true
	router, err := NewRouter(func() SchedulerStatus { return SchedulerStatus{Status: "not_implemented"} },
		deps.auth, deps.tenants, deps.dashboard, deps.history, deps.reports, deps.alerts, deps.collection, deps.settings, deps.licenses)
	if err != nil {
		t.Fatal(err)
	}
	cookie := authenticatedSession(t, deps)

	body, _ := json.Marshal(licensePreviewRequest{LicenseBase64: testLicenseBase64})
	req := httptest.NewRequest(http.MethodPost, "/api/tenants/x/license/preview", bytes.NewReader(body))
	req.AddCookie(cookie)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "tok"})
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set(auth.CSRFHeaderName, "tok")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 -- preview never touches a real server, so demo mode must not block it, body: %s", rec.Code, rec.Body.String())
	}
}

func strPtr(s string) *string { return &s }

func saveLicenseServer(t *testing.T, deps testDeps, tenantID, host, port string) {
	t.Helper()
	p, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	pw := "secret"
	// tlsSkipVerify=true: the fake License Server in these tests uses
	// httptest's self-signed cert.
	if err := deps.licenses.Tenants.UpdateLicenseServer(context.Background(), tenantID, host, p, "prou_services", &pw, true); err != nil {
		t.Fatal(err)
	}
}

func splitHostPort(t *testing.T, rawURL string) (string, string) {
	t.Helper()
	u := strings.TrimPrefix(rawURL, "https://")
	parts := strings.SplitN(u, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected test server URL: %s", rawURL)
	}
	return parts[0], parts[1]
}
