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

func TestExplicitHTTPAddr_NotOKWhenNothingConfigured(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()

	if v, ok := explicitHTTPAddr(dir); ok {
		t.Errorf("explicitHTTPAddr = (%q, true), want ok=false with no env/.env", v)
	}
}

func TestExplicitHTTPAddr_NotOKWhenFileExistsWithoutKey(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SOME_OTHER_KEY=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if v, ok := explicitHTTPAddr(dir); ok {
		t.Errorf("explicitHTTPAddr = (%q, true), want ok=false when .env doesn't have the key", v)
	}
}

func TestExplicitHTTPAddr_OKFromFile(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PUMC_HTTP_ADDR=0.0.0.0:9999\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	v, ok := explicitHTTPAddr(dir)
	if !ok || v != "0.0.0.0:9999" {
		t.Errorf("explicitHTTPAddr = (%q, %v), want (%q, true)", v, ok, "0.0.0.0:9999")
	}
}

func TestExplicitHTTPAddr_EnvWinsOverFile(t *testing.T) {
	os.Setenv("PUMC_HTTP_ADDR", "127.0.0.1:1111")
	defer os.Unsetenv("PUMC_HTTP_ADDR")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PUMC_HTTP_ADDR=0.0.0.0:9999\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	v, ok := explicitHTTPAddr(dir)
	if !ok || v != "127.0.0.1:1111" {
		t.Errorf("explicitHTTPAddr = (%q, %v), want (%q, true)", v, ok, "127.0.0.1:1111")
	}
}

func newDemoDBFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("not a real sqlite file -- demoDBPresent only checks existence"), 0o600); err != nil {
		t.Fatalf("write demo.db fixture: %v", err)
	}
}

func TestDemoDBPresent_FalseWhenAbsent(t *testing.T) {
	os.Unsetenv("PUMC_DB_DSN")
	dir := t.TempDir()

	if demoDBPresent(dir) {
		t.Error("demoDBPresent = true, want false with no demo.db")
	}
}

func TestDemoDBPresent_TrueNextToDefaultDSN(t *testing.T) {
	os.Unsetenv("PUMC_DB_DSN")
	dir := t.TempDir()
	newDemoDBFile(t, filepath.Join(dir, "demo.db"))

	if !demoDBPresent(dir) {
		t.Error("demoDBPresent = false, want true with demo.db next to the default DSN")
	}
}

func TestDemoDBPresent_FollowsDBDSNOverrideToADifferentDirectory(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Setenv("PUMC_DB_DSN", filepath.Join(dbDir, "custom.db"))
	defer os.Unsetenv("PUMC_DB_DSN")

	if demoDBPresent(dir) {
		t.Error("demoDBPresent = true, want false before demo.db exists in the overridden directory")
	}

	newDemoDBFile(t, filepath.Join(dbDir, "demo.db"))
	if !demoDBPresent(dir) {
		t.Error("demoDBPresent = false, want true once demo.db exists next to the PUMC_DB_DSN-overridden path")
	}
	// The default-location demo.db must not be what's detected here.
	if _, err := os.Stat(filepath.Join(dir, "demo.db")); err == nil {
		t.Fatal("test setup error: a demo.db exists at the default location too")
	}
}

func TestApplyDemoDefaults_SeedsPortWhenDemoDBPresentAndNothingExplicit(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	os.Unsetenv("PUMC_DB_DSN")
	dir := t.TempDir()
	newDemoDBFile(t, filepath.Join(dir, "demo.db"))

	applyDemoDefaults(dir)

	got := currentHTTPAddr(dir)
	if got != demoModeHTTPAddr {
		t.Errorf("currentHTTPAddr after applyDemoDefaults = %q, want %q", got, demoModeHTTPAddr)
	}
}

func TestApplyDemoDefaults_DoesNothingWhenNoDemoDB(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	os.Unsetenv("PUMC_DB_DSN")
	dir := t.TempDir()

	applyDemoDefaults(dir)

	if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
		t.Error(".env was created even though no demo.db is present")
	}
}

func TestApplyDemoDefaults_NeverOverridesAnExplicitPort(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	os.Unsetenv("PUMC_DB_DSN")
	dir := t.TempDir()
	newDemoDBFile(t, filepath.Join(dir, "demo.db"))
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PUMC_HTTP_ADDR=0.0.0.0:7777\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	applyDemoDefaults(dir)

	got := currentHTTPAddr(dir)
	if got != "0.0.0.0:7777" {
		t.Errorf("currentHTTPAddr after applyDemoDefaults = %q, want the already-explicit %q untouched", got, "0.0.0.0:7777")
	}
}

func TestApplyDemoDefaults_IsAOneTimeSeedNotAPersistentOverride(t *testing.T) {
	os.Unsetenv("PUMC_HTTP_ADDR")
	os.Unsetenv("PUMC_DB_DSN")
	dir := t.TempDir()
	newDemoDBFile(t, filepath.Join(dir, "demo.db"))

	applyDemoDefaults(dir)
	if got := currentHTTPAddr(dir); got != demoModeHTTPAddr {
		t.Fatalf("first applyDemoDefaults: currentHTTPAddr = %q, want %q", got, demoModeHTTPAddr)
	}

	// An operator manually changes the port afterward (e.g. via the
	// Change Port dialog) -- a later run of applyDemoDefaults must not
	// revert that.
	if err := dotenv.SetValue(filepath.Join(dir, ".env"), "PUMC_HTTP_ADDR", "0.0.0.0:8443"); err != nil {
		t.Fatal(err)
	}

	applyDemoDefaults(dir)
	if got := currentHTTPAddr(dir); got != "0.0.0.0:8443" {
		t.Errorf("second applyDemoDefaults reverted an operator's manual port change: got %q, want %q", got, "0.0.0.0:8443")
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
