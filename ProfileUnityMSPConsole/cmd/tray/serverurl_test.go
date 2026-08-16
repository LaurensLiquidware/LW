package main

import (
	"os"
	"path/filepath"
	"testing"

	"profileunity-msp-console/internal/dotenv"
)

func TestResolveServerURL_DefaultsToLocalhost(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()

	got := resolveServerURL(dir)
	want := "https://localhost:8443"
	if got != want {
		t.Errorf("resolveServerURL = %q, want %q", got, want)
	}
}

func TestResolveServerURL_HonorsEnvFileOverride(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("PUMC_HTTP_ADDR=192.168.1.50:9443\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := resolveServerURL(dir)
	want := "https://192.168.1.50:9443"
	if got != want {
		t.Errorf("resolveServerURL = %q, want %q", got, want)
	}
	os.Unsetenv("PUMC_HTTP_ADDR")
}

func TestResolveServerURL_AllInterfacesBecomesLocalhost(t *testing.T) {
	os.Setenv("PUMC_HTTP_ADDR", "0.0.0.0:8443")
	defer os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()

	got := resolveServerURL(dir)
	want := "https://localhost:8443"
	if got != want {
		t.Errorf("resolveServerURL = %q, want %q", got, want)
	}
}

func TestCurrentHTTPAddr_DefaultsWhenNoEnvOrFile(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()

	got := currentHTTPAddr(dir)
	if got != "0.0.0.0:8443" {
		t.Errorf("currentHTTPAddr = %q, want %q", got, "0.0.0.0:8443")
	}
}

func TestCurrentHTTPAddr_RealEnvVarWins(t *testing.T) {
	os.Setenv("PUMC_HTTP_ADDR", "127.0.0.1:9000")
	defer os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()

	got := currentHTTPAddr(dir)
	if got != "127.0.0.1:9000" {
		t.Errorf("currentHTTPAddr = %q, want %q", got, "127.0.0.1:9000")
	}
}

func TestCurrentHTTPAddr_ReadsFileDirectly(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("PUMC_HTTP_ADDR=0.0.0.0:8444\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := currentHTTPAddr(dir)
	if got != "0.0.0.0:8444" {
		t.Errorf("currentHTTPAddr = %q, want %q", got, "0.0.0.0:8444")
	}
}

// TestCurrentHTTPAddr_ReflectsFileChangesAcrossCalls is the regression
// test for the staleness bug the old dotenv.Load-based resolveServerURL
// had: calling it a second time after the .env file changed must return
// the new value, not the first call's cached env-var side effect.
func TestCurrentHTTPAddr_ReflectsFileChangesAcrossCalls(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("PUMC_HTTP_ADDR=0.0.0.0:8443\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	first := currentHTTPAddr(dir)
	if first != "0.0.0.0:8443" {
		t.Fatalf("first call = %q, want %q", first, "0.0.0.0:8443")
	}

	if err := dotenv.SetValue(envPath, "PUMC_HTTP_ADDR", "0.0.0.0:8444"); err != nil {
		t.Fatal(err)
	}

	second := currentHTTPAddr(dir)
	if second != "0.0.0.0:8444" {
		t.Errorf("second call (after file change) = %q, want %q -- currentHTTPAddr must not cache stale values", second, "0.0.0.0:8444")
	}
}

func TestValidatePort(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"8443", "8443", false},
		{"  8444  ", "8444", false},
		{"1", "1", false},
		{"65535", "65535", false},
		{"0", "", true},
		{"65536", "", true},
		{"-1", "", true},
		{"not-a-number", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := validatePort(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("validatePort(%q) = %q, nil, want an error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("validatePort(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("validatePort(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
