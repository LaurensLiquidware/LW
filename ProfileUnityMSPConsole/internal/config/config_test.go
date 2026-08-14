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

func TestLoad_RequiresHTTPAddr(t *testing.T) {
	withEnv(t, map[string]string{envHTTPAddr: ""}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error when PUMC_HTTP_ADDR is unset, got nil")
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
		if cfg.LogLevel != "info" {
			t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
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
