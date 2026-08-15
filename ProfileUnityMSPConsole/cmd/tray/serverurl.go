// This file has no build tag: plain, cross-platform logic, split out
// from main_windows.go so it can be unit-tested without a Windows GUI
// (same reasoning as logpath.go).
package main

import (
	"net"
	"os"
	"path/filepath"

	"profileunity-msp-console/internal/config"
	"profileunity-msp-console/internal/dotenv"
)

// resolveServerURL builds the URL the server (running with working
// directory installDir) actually listens on, for the clickable link in
// the tray window. PUMC_HTTP_ADDR's host is often "0.0.0.0" (all
// interfaces, the default) or empty, neither of which is a browsable
// address -- both are treated as "localhost", which is always correct
// for opening the console from the same machine the launcher runs on.
func resolveServerURL(installDir string) string {
	dotenv.Load(filepath.Join(installDir, ".env"))

	addr := os.Getenv("PUMC_HTTP_ADDR")
	if addr == "" {
		addr = config.DefaultHTTPAddr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "https://localhost"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "https://" + net.JoinHostPort(host, port)
}
