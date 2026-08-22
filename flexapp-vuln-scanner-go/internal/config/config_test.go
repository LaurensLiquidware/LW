package config

import "testing"

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
		if cfg.HTTPAddr != DefaultHTTPAddr {
			t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, DefaultHTTPAddr)
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
		envHTTPAddr:    "",
		envEnvironment: "",
		envLogLevel:    "",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Environment != "development" {
			t.Errorf("Environment = %q, want development", cfg.Environment)
		}
		if cfg.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want debug (default environment is development)", cfg.LogLevel)
		}
		if cfg.LogFile != "./flexapp-vuln-scanner.log" {
			t.Errorf("LogFile = %q, want ./flexapp-vuln-scanner.log", cfg.LogFile)
		}
		if cfg.DefaultOutputDir != "./scan-out" {
			t.Errorf("DefaultOutputDir = %q, want ./scan-out", cfg.DefaultOutputDir)
		}
		if cfg.CacheDir != "./cache" {
			t.Errorf("CacheDir = %q, want ./cache", cfg.CacheDir)
		}
		if cfg.ScanHistoryFile != "./scan-history.json" {
			t.Errorf("ScanHistoryFile = %q, want ./scan-history.json", cfg.ScanHistoryFile)
		}
	})
}

func TestLoad_LogLevelDefaultsToInfoInProduction(t *testing.T) {
	withEnv(t, map[string]string{
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

func TestLoad_RejectsUnknownLogLevel(t *testing.T) {
	withEnv(t, map[string]string{
		envLogLevel: "verbose",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("expected error for unsupported log level, got nil")
		}
	})
}

func TestLoad_NVDAPIKeyDefaultsEmpty(t *testing.T) {
	withEnv(t, map[string]string{envNVDAPIKey: ""}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.NVDAPIKey != "" {
			t.Errorf("NVDAPIKey = %q, want empty", cfg.NVDAPIKey)
		}
	})
}

func TestLoad_NVDAPIKeyExplicit(t *testing.T) {
	withEnv(t, map[string]string{envNVDAPIKey: "  some-key  "}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.NVDAPIKey != "some-key" {
			t.Errorf("NVDAPIKey = %q, want %q", cfg.NVDAPIKey, "some-key")
		}
	})
}

func TestLoad_SkipDefenderScanDefaultsFalse(t *testing.T) {
	withEnv(t, map[string]string{envSkipDefenderScan: ""}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.SkipDefenderScan {
			t.Error("SkipDefenderScan = true, want false")
		}
	})
}

func TestLoad_SkipDefenderScanExplicit(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes"} {
		withEnv(t, map[string]string{envSkipDefenderScan: v}, func() {
			cfg, err := Load()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !cfg.SkipDefenderScan {
				t.Errorf("SkipDefenderScan = false for %q, want true", v)
			}
		})
	}
}

func TestLoad_SkipDefenderScanRejectsUnrecognizedValue(t *testing.T) {
	withEnv(t, map[string]string{envSkipDefenderScan: "banana"}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.SkipDefenderScan {
			t.Error("SkipDefenderScan = true for an unrecognized value, want false")
		}
	})
}
