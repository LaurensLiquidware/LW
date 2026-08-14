package config

import "testing"

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
