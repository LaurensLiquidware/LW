package dotenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	input := strings.Join([]string{
		"# a comment",
		"",
		"PUMC_HTTP_ADDR=0.0.0.0:8443",
		"  PUMC_ENVIRONMENT = production  ",
		`PUMC_DB_DSN="./profileunity-msp-console.db"`,
		"PUMC_BOOTSTRAP_ADMIN_PASSWORD='has a space'",
		"PUMC_LOG_LEVEL=", // explicitly empty is valid
	}, "\n")

	got, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"PUMC_HTTP_ADDR":                "0.0.0.0:8443",
		"PUMC_ENVIRONMENT":              "production",
		"PUMC_DB_DSN":                   "./profileunity-msp-console.db",
		"PUMC_BOOTSTRAP_ADMIN_PASSWORD": "has a space",
		"PUMC_LOG_LEVEL":                "",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d keys, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestParse_MissingEquals(t *testing.T) {
	if _, err := Parse(strings.NewReader("NOT_A_KV_LINE")); err == nil {
		t.Fatal("expected error for a line with no '=', got nil")
	}
}

func TestParse_EmptyKey(t *testing.T) {
	if _, err := Parse(strings.NewReader("=value")); err == nil {
		t.Fatal("expected error for an empty key, got nil")
	}
}

func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	if err := Load(filepath.Join(t.TempDir(), "does-not-exist.env")); err != nil {
		t.Fatalf("missing file should be a no-op, got: %v", err)
	}
}

func TestLoad_SetsUnsetVars(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("PUMC_TEST_DOTENV_A=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PUMC_TEST_DOTENV_A", "")
	os.Unsetenv("PUMC_TEST_DOTENV_A")

	if err := Load(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := os.Getenv("PUMC_TEST_DOTENV_A"); got != "from-file" {
		t.Errorf("PUMC_TEST_DOTENV_A = %q, want %q", got, "from-file")
	}
}

func TestLoad_RealEnvVarWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("PUMC_TEST_DOTENV_B=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PUMC_TEST_DOTENV_B", "from-real-env")

	if err := Load(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := os.Getenv("PUMC_TEST_DOTENV_B"); got != "from-real-env" {
		t.Errorf("PUMC_TEST_DOTENV_B = %q, want %q (real env must win)", got, "from-real-env")
	}
}
