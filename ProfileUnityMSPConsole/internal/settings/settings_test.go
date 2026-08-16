package settings

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"profileunity-msp-console/internal/db"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewStore(sqlDB)
}

func exampleSettings() Settings {
	return Settings{
		SMTPHost:                "smtp.example.com",
		SMTPPort:                587,
		SMTPFrom:                "reports@example.com",
		SMTPSecurity:            "starttls",
		ReportRecipients:        []string{"msp@liquidware.eu", "ops@example.com"},
		ReportEmailDay:          1,
		CollectionInterval:      time.Hour,
		CollectionTimezone:      "UTC",
		CollectionConcurrency:   5,
		CollectionTenantTimeout: 30 * time.Second,
		SessionIdleTimeout:      30 * time.Minute,
		SessionAbsoluteTimeout:  12 * time.Hour,
	}
}

func TestEnsureSeeded_InsertsWhenEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	seed := exampleSettings()
	got, err := store.EnsureSeeded(ctx, seed)
	if err != nil {
		t.Fatalf("EnsureSeeded: %v", err)
	}
	if got.SMTPHost != seed.SMTPHost || got.CollectionInterval != seed.CollectionInterval {
		t.Errorf("EnsureSeeded returned %+v, want the seed values", got)
	}

	loaded, ok, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !ok {
		t.Fatal("Load ok = false after EnsureSeeded")
	}
	if len(loaded.ReportRecipients) != 2 || loaded.ReportRecipients[0] != "msp@liquidware.eu" {
		t.Errorf("ReportRecipients = %v, want [msp@liquidware.eu ops@example.com]", loaded.ReportRecipients)
	}
	if loaded.CollectionInterval != time.Hour {
		t.Errorf("CollectionInterval = %s, want 1h", loaded.CollectionInterval)
	}
}

func TestEnsureSeeded_DoesNotOverwriteExisting(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	first := exampleSettings()
	if _, err := store.EnsureSeeded(ctx, first); err != nil {
		t.Fatalf("first EnsureSeeded: %v", err)
	}

	second := exampleSettings()
	second.SMTPHost = "should-not-be-used.example.com"
	got, err := store.EnsureSeeded(ctx, second)
	if err != nil {
		t.Fatalf("second EnsureSeeded: %v", err)
	}
	if got.SMTPHost != first.SMTPHost {
		t.Errorf("EnsureSeeded overwrote an existing row: SMTPHost = %q, want %q", got.SMTPHost, first.SMTPHost)
	}
}

func TestLoad_NotOKWhenNeverSeeded(t *testing.T) {
	store := newTestStore(t)
	_, ok, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ok {
		t.Error("Load ok = true on a fresh database with no seeded row")
	}
}

func TestUpdate_PersistsAndRoundTrips(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if _, err := store.EnsureSeeded(ctx, exampleSettings()); err != nil {
		t.Fatalf("EnsureSeeded: %v", err)
	}

	updated := exampleSettings()
	updated.CollectionInterval = 2 * time.Hour
	updated.ReportEmailDay = 15
	updated.ReportRecipients = []string{"new@example.com"}
	if err := store.Update(ctx, updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, ok, err := store.Load(ctx)
	if err != nil || !ok {
		t.Fatalf("Load after Update: ok=%v err=%v", ok, err)
	}
	if got.CollectionInterval != 2*time.Hour {
		t.Errorf("CollectionInterval = %s, want 2h", got.CollectionInterval)
	}
	if got.ReportEmailDay != 15 {
		t.Errorf("ReportEmailDay = %d, want 15", got.ReportEmailDay)
	}
	if len(got.ReportRecipients) != 1 || got.ReportRecipients[0] != "new@example.com" {
		t.Errorf("ReportRecipients = %v, want [new@example.com]", got.ReportRecipients)
	}
}

func TestUpdateTLSCert_LeavesOtherFieldsUntouched(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	seed := exampleSettings()
	if _, err := store.EnsureSeeded(ctx, seed); err != nil {
		t.Fatalf("EnsureSeeded: %v", err)
	}

	if err := store.UpdateTLSCert(ctx, "-----BEGIN CERTIFICATE-----\n...", "-----BEGIN EC PRIVATE KEY-----\n..."); err != nil {
		t.Fatalf("UpdateTLSCert: %v", err)
	}

	got, ok, err := store.Load(ctx)
	if err != nil || !ok {
		t.Fatalf("Load after UpdateTLSCert: ok=%v err=%v", ok, err)
	}
	if got.TLSCertPEM == "" || got.TLSKeyPEM == "" {
		t.Error("TLSCertPEM/TLSKeyPEM were not persisted")
	}
	if got.SMTPHost != seed.SMTPHost {
		t.Errorf("UpdateTLSCert changed an unrelated field: SMTPHost = %q, want %q", got.SMTPHost, seed.SMTPHost)
	}
}

func TestUpdate_PersistsCompanyName(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if _, err := store.EnsureSeeded(ctx, exampleSettings()); err != nil {
		t.Fatalf("EnsureSeeded: %v", err)
	}

	updated := exampleSettings()
	updated.CompanyName = "Acme MSP"
	if err := store.Update(ctx, updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, ok, err := store.Load(ctx)
	if err != nil || !ok {
		t.Fatalf("Load after Update: ok=%v err=%v", ok, err)
	}
	if got.CompanyName != "Acme MSP" {
		t.Errorf("CompanyName = %q, want %q", got.CompanyName, "Acme MSP")
	}
}

func TestUpdateBranding_SetsAndClearsLogoWithoutTouchingOtherFields(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	seed := exampleSettings()
	if _, err := store.EnsureSeeded(ctx, seed); err != nil {
		t.Fatalf("EnsureSeeded: %v", err)
	}

	logo := []byte{0x89, 0x50, 0x4E, 0x47} // not a full PNG, just distinct bytes to round-trip
	if err := store.UpdateBranding(ctx, "Acme MSP", logo, "png"); err != nil {
		t.Fatalf("UpdateBranding (set): %v", err)
	}

	got, ok, err := store.Load(ctx)
	if err != nil || !ok {
		t.Fatalf("Load after UpdateBranding: ok=%v err=%v", ok, err)
	}
	if got.CompanyName != "Acme MSP" {
		t.Errorf("CompanyName = %q, want %q", got.CompanyName, "Acme MSP")
	}
	if string(got.CompanyLogoImage) != string(logo) || got.CompanyLogoImageType != "png" {
		t.Errorf("CompanyLogoImage/Type = %v/%q, want %v/png", got.CompanyLogoImage, got.CompanyLogoImageType, logo)
	}
	if got.SMTPHost != seed.SMTPHost {
		t.Errorf("UpdateBranding changed an unrelated field: SMTPHost = %q, want %q", got.SMTPHost, seed.SMTPHost)
	}

	if err := store.UpdateBranding(ctx, "Acme MSP", nil, ""); err != nil {
		t.Fatalf("UpdateBranding (clear): %v", err)
	}
	cleared, ok, err := store.Load(ctx)
	if err != nil || !ok {
		t.Fatalf("Load after clearing branding: ok=%v err=%v", ok, err)
	}
	if len(cleared.CompanyLogoImage) != 0 || cleared.CompanyLogoImageType != "" {
		t.Errorf("logo not cleared: image=%v type=%q", cleared.CompanyLogoImage, cleared.CompanyLogoImageType)
	}
	if cleared.CompanyName != "Acme MSP" {
		t.Errorf("clearing the logo changed CompanyName: got %q, want %q", cleared.CompanyName, "Acme MSP")
	}
}

func TestValidate_RejectsSMTPHostWithoutFrom(t *testing.T) {
	s := exampleSettings()
	s.SMTPFrom = ""
	if err := s.Validate(); err == nil {
		t.Fatal("expected an error for an SMTP host without a from address")
	}
}

func TestValidate_RejectsSMTPHostWithoutRecipients(t *testing.T) {
	s := exampleSettings()
	s.ReportRecipients = nil
	if err := s.Validate(); err == nil {
		t.Fatal("expected an error for an SMTP host without report recipients")
	}
}

func TestValidate_AllowsSMTPFullyDisabled(t *testing.T) {
	s := exampleSettings()
	s.SMTPHost, s.SMTPFrom, s.ReportRecipients = "", "", nil
	if err := s.Validate(); err != nil {
		t.Errorf("unexpected error for a fully-disabled SMTP config: %v", err)
	}
}

func TestValidate_RejectsUnknownTimezone(t *testing.T) {
	s := exampleSettings()
	s.CollectionTimezone = "Not/AZone"
	if err := s.Validate(); err == nil {
		t.Fatal("expected an error for an unknown IANA timezone")
	}
}

func TestValidate_RejectsOutOfRangeReportEmailDay(t *testing.T) {
	s := exampleSettings()
	s.ReportEmailDay = 30
	if err := s.Validate(); err == nil {
		t.Fatal("expected an error for a report email day outside 1-28")
	}
}
