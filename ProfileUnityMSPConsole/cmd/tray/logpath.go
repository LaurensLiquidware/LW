// Package main is the tray launcher. This file has no build tag: it's
// plain, cross-platform logic (no Win32 calls), split out from
// main_windows.go so it can be unit-tested without a Windows GUI.
package main

import (
	"os"
	"path/filepath"

	"profileunity-msp-console/internal/config"
	"profileunity-msp-console/internal/dotenv"
)

// resolveLogFilePath figures out which file the server (running with
// working directory installDir) is logging to, so the "Show Log" viewer
// tails the right file even if an operator customized PUMC_LOG_FILE.
// It loads installDir's .env the same way cmd/server does (dotenv.Load
// only sets process env vars -- harmless to call from the tray process,
// which doesn't otherwise use these), then applies the same default the
// server itself would.
func resolveLogFilePath(installDir string) string {
	dotenv.Load(filepath.Join(installDir, ".env"))

	logFile := os.Getenv("PUMC_LOG_FILE")
	if logFile == "" {
		logFile = config.DefaultLogFile
	}
	if filepath.IsAbs(logFile) {
		return logFile
	}
	return filepath.Join(installDir, logFile)
}
