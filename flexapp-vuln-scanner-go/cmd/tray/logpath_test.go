package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLogFilePath_DefaultsWhenNoEnvFile(t *testing.T) {
	dir := t.TempDir()
	os.Unsetenv("FVS_LOG_FILE")

	got := resolveLogFilePath(dir)
	want := filepath.Join(dir, "flexapp-vuln-scanner.log")
	if got != want {
		t.Errorf("resolveLogFilePath = %q, want %q", got, want)
	}
}

func TestResolveLogFilePath_HonorsEnvFileOverride(t *testing.T) {
	os.Unsetenv("FVS_LOG_FILE")
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("FVS_LOG_FILE=custom.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := resolveLogFilePath(dir)
	want := filepath.Join(dir, "custom.log")
	if got != want {
		t.Errorf("resolveLogFilePath = %q, want %q", got, want)
	}
	os.Unsetenv("FVS_LOG_FILE")
}

func TestResolveLogFilePath_AbsoluteOverrideUsedAsIs(t *testing.T) {
	os.Unsetenv("FVS_LOG_FILE")
	dir := t.TempDir()
	abs := filepath.Join(t.TempDir(), "elsewhere.log")
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("FVS_LOG_FILE="+abs+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := resolveLogFilePath(dir)
	if got != abs {
		t.Errorf("resolveLogFilePath = %q, want %q", got, abs)
	}
	os.Unsetenv("FVS_LOG_FILE")
}
