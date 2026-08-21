package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flexapp-vuln-scanner/internal/config"
)

func TestNew_WritesToFileAtConfiguredLevel(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	cfg := config.Config{LogLevel: "info", LogFile: logFile}

	logger, closer, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer closer.Close()

	logger.Debug("this debug line should be filtered out")
	logger.Info("this info line should appear", "key", "value")

	contents, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if strings.Contains(string(contents), "this debug line") {
		t.Errorf("expected debug line to be filtered at info level, got: %s", contents)
	}
	if !strings.Contains(string(contents), "this info line should appear") {
		t.Errorf("expected info line in log file, got: %s", contents)
	}
}

func TestNew_DebugLevelIncludesDebugLines(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	cfg := config.Config{LogLevel: "debug", LogFile: logFile}

	logger, closer, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer closer.Close()

	logger.Debug("a debug line")

	contents, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(contents), "a debug line") {
		t.Errorf("expected debug line to appear at debug level, got: %s", contents)
	}
}

func TestNew_RejectsUnknownLevel(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	cfg := config.Config{LogLevel: "verbose", LogFile: logFile}

	if _, _, err := New(cfg); err == nil {
		t.Fatal("expected error for unknown log level, got nil")
	}
}
