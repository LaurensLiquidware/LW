package main

import (
	"os"
	"path/filepath"
	"testing"
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
