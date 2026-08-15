package config

import (
	"encoding/base64"
	"testing"
	"time"
)

func withEnv(t *testing.T, kv map[string]string, fn func()) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
	fn()
}

func TestLoad_HTTPAddrDefaultsWhenUnset(t *testing.T) {
	withEnv(t, map[string]string{envHTTPAddr: ""}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.HTTPAddr != "0.0.0.0:8443" {
			t.Errorf("HTTPAddr = %q, want 0.0.0.0:8443", cfg.HTTPAddr)
		}
	})
}

func TestLoad_HTTPAddrExplicitOverridesDefault(t *testing.T) {
	withEnv(t, map[string]string{envHTTPAddr: "127.0.0.1:9000"}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.HTTPAddr != "127.0.0.1:9000" {
			t.Errorf("HTTPAddr = %q, want 127.0.0.1:9000", cfg.HTTPAddr)
		}
	})
}

func TestLoad_Defaults(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:    "0.0.0.0:8443",
		envEnvironment: "",
		envDBDriver:    "",
		envDBDSN:       "",
		envLogLevel:    "",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Environment != "development" {
			t.Errorf("Environment = %q, want development", cfg.Environment)
		}
		if cfg.DBDriver != "sqlite" {
			t.Errorf("DBDriver = %q, want sqlite", cfg.DBDriver)
		}
		if cfg.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want debug (default environment is development)", cfg.LogLevel)
		}
		if cfg.LogFile != "./profileunity-msp-console.log" {
			t.Errorf("LogFile = %q, want ./profileunity-msp-console.log", cfg.LogFile)
		}
		if cfg.CredentialEncryptionKeyFile != "./credential-encryption.key" {
			t.Errorf("CredentialEncryptionKeyFile = %q, want ./credential-encryption.key", cfg.CredentialEncryptionKeyFile)
		}
	})
}

func TestLoad_LogLevelDefaultsToInfoInProduction(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:    "0.0.0.0:8443",
		envEnvironment: "production",
		envLogLevel:    "",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LogLevel != "info" {
			t.Errorf("LogLevel = %q, want info in production", cfg.LogLevel)
		}
	})
}

func TestLoad_LogLevelExplicitOverridesEnvironmentDefault(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:    "0.0.0.0:8443",
		envEnvironment: "production",
		envLogLevel:    "debug",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want debug (explicit override)", cfg.LogLevel)
		}
	})
}

func TestLoad_RejectsUnknownDBDriver(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr: "0.0.0.0:8443",
		envDBDriver: "mysql",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for unsupported DB driver, got nil")
		}
	})
}

func TestLoad_RejectsUnknownLogLevel(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr: "0.0.0.0:8443",
		envLogLevel: "verbose",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for unsupported log level, got nil")
		}
	})
}

func TestLoad_CollectionDefaults(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:                "0.0.0.0:8443",
		envCollectionInterval:      "",
		envCollectionTimezone:      "",
		envCollectionConcurrency:   "",
		envCollectionTenantTimeout: "",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.CollectionInterval != time.Hour {
			t.Errorf("CollectionInterval = %s, want 1h", cfg.CollectionInterval)
		}
		if cfg.CollectionTenantTimeout != 30*time.Second {
			t.Errorf("CollectionTenantTimeout = %s, want 30s", cfg.CollectionTenantTimeout)
		}
		if cfg.CollectionConcurrency != 5 {
			t.Errorf("CollectionConcurrency = %d, want 5", cfg.CollectionConcurrency)
		}
		if cfg.CollectionTimezone != "UTC" || cfg.CollectionLocation != time.UTC {
			t.Errorf("CollectionTimezone = %q, CollectionLocation = %v", cfg.CollectionTimezone, cfg.CollectionLocation)
		}
	})
}

func TestLoad_RejectsUnknownTimezone(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:           "0.0.0.0:8443",
		envCollectionTimezone: "Not/AZone",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for an unknown IANA timezone")
		}
	})
}

func TestLoad_RejectsNonPositiveConcurrency(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:              "0.0.0.0:8443",
		envCollectionConcurrency: "0",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for zero concurrency")
		}
	})
}

func TestLoad_CredentialEncryptionKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(key)

	withEnv(t, map[string]string{
		envHTTPAddr:                "0.0.0.0:8443",
		envCredentialEncryptionKey: encoded,
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.CredentialEncryptionKey) != 32 {
			t.Errorf("CredentialEncryptionKey length = %d, want 32", len(cfg.CredentialEncryptionKey))
		}
	})
}

func TestLoad_RejectsWrongSizeEncryptionKey(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("too-short"))
	withEnv(t, map[string]string{
		envHTTPAddr:                "0.0.0.0:8443",
		envCredentialEncryptionKey: encoded,
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for a wrong-size encryption key")
		}
	})
}

func TestLoad_NoEncryptionKeyByDefault(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:                "0.0.0.0:8443",
		envCredentialEncryptionKey: "",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.CredentialEncryptionKey != nil {
			t.Error("CredentialEncryptionKey should be nil when unset")
		}
	})
}

func TestLoad_SessionAndTLSDefaults(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:               "0.0.0.0:8443",
		envSessionIdleTimeout:     "",
		envSessionAbsoluteTimeout: "",
		envTLSCertFile:            "",
		envTLSKeyFile:             "",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.SessionIdleTimeout != 30*time.Minute {
			t.Errorf("SessionIdleTimeout = %s, want 30m", cfg.SessionIdleTimeout)
		}
		if cfg.SessionAbsoluteTimeout != 12*time.Hour {
			t.Errorf("SessionAbsoluteTimeout = %s, want 12h", cfg.SessionAbsoluteTimeout)
		}
		if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
			t.Error("TLSCertFile/TLSKeyFile should have non-empty defaults")
		}
	})
}

func TestLoad_RejectsMismatchedBootstrapAdminCredentials(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:               "0.0.0.0:8443",
		envBootstrapAdminUsername: "admin",
		envBootstrapAdminPassword: "",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for a username without a password")
		}
	})
}

func TestLoad_AcceptsMatchedBootstrapAdminCredentials(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:               "0.0.0.0:8443",
		envBootstrapAdminUsername: "admin",
		envBootstrapAdminPassword: "correct-horse-battery-staple",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.BootstrapAdminUsername != "admin" {
			t.Errorf("BootstrapAdminUsername = %q", cfg.BootstrapAdminUsername)
		}
	})
}

func TestLoad_BootstrapAdminDefaultsWhenBothUnset(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:               "0.0.0.0:8443",
		envBootstrapAdminUsername: "",
		envBootstrapAdminPassword: "",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.BootstrapAdminUsername != DefaultBootstrapAdminUsername {
			t.Errorf("BootstrapAdminUsername = %q, want %q", cfg.BootstrapAdminUsername, DefaultBootstrapAdminUsername)
		}
		if cfg.BootstrapAdminPassword != DefaultBootstrapAdminPassword {
			t.Errorf("BootstrapAdminPassword = %q, want %q", cfg.BootstrapAdminPassword, DefaultBootstrapAdminPassword)
		}
	})
}

func TestLoad_ReportEmailDisabledByDefault(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr: "0.0.0.0:8443",
		envSMTPHost: "",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ReportEmailEnabled() {
			t.Error("ReportEmailEnabled() should be false when PUMC_SMTP_HOST is unset")
		}
	})
}

func TestLoad_ReportEmailFullyConfigured(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:         "0.0.0.0:8443",
		envSMTPHost:         "smtp.example.com",
		envSMTPFrom:         "reports@example.com",
		envReportRecipients: "msp@liquidware.eu, ops@example.com ,",
		envReportEmailDay:   "1",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.ReportEmailEnabled() {
			t.Fatal("ReportEmailEnabled() should be true once PUMC_SMTP_HOST is set")
		}
		if cfg.SMTPPort != 587 {
			t.Errorf("SMTPPort = %d, want default 587", cfg.SMTPPort)
		}
		if cfg.SMTPSecurity != "starttls" {
			t.Errorf("SMTPSecurity = %q, want default starttls", cfg.SMTPSecurity)
		}
		want := []string{"msp@liquidware.eu", "ops@example.com"}
		if len(cfg.ReportRecipients) != len(want) || cfg.ReportRecipients[0] != want[0] || cfg.ReportRecipients[1] != want[1] {
			t.Errorf("ReportRecipients = %v, want %v", cfg.ReportRecipients, want)
		}
	})
}

func TestLoad_RejectsSMTPHostWithoutFrom(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:         "0.0.0.0:8443",
		envSMTPHost:         "smtp.example.com",
		envSMTPFrom:         "",
		envReportRecipients: "msp@liquidware.eu",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for SMTP host set without a from address")
		}
	})
}

func TestLoad_RejectsSMTPHostWithoutRecipients(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:         "0.0.0.0:8443",
		envSMTPHost:         "smtp.example.com",
		envSMTPFrom:         "reports@example.com",
		envReportRecipients: "",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for SMTP host set without report recipients")
		}
	})
}

func TestLoad_RejectsRecipientsWithoutSMTPHost(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:         "0.0.0.0:8443",
		envSMTPHost:         "",
		envReportRecipients: "msp@liquidware.eu",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for report recipients set without an SMTP host")
		}
	})
}

func TestLoad_RejectsUnknownSMTPSecurity(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:     "0.0.0.0:8443",
		envSMTPSecurity: "ssl",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for an unsupported SMTP security mode")
		}
	})
}

func TestLoad_RejectsOutOfRangeReportEmailDay(t *testing.T) {
	withEnv(t, map[string]string{
		envHTTPAddr:         "0.0.0.0:8443",
		envSMTPHost:         "smtp.example.com",
		envSMTPFrom:         "reports@example.com",
		envReportRecipients: "msp@liquidware.eu",
		envReportEmailDay:   "29",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for a report email day outside 1-28")
		}
	})
}
