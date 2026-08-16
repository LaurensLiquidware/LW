// This file has no build tag: plain, cross-platform logic, split out
// from main_windows.go so it can be unit-tested without a Windows GUI
// (same reasoning as logpath.go).
package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"profileunity-msp-console/internal/config"
	"profileunity-msp-console/internal/dotenv"
)

// currentHTTPAddr returns the host:port the server at installDir is
// configured to listen on -- a real PUMC_HTTP_ADDR environment variable
// wins (matching internal/dotenv.Load's own "env always wins over file"
// rule), otherwise installDir/.env's value, otherwise
// config.DefaultHTTPAddr. Unlike dotenv.Load, this never mutates process
// environment, so it's safe to call again after changing the port (e.g.
// from cmd/tray's "Change Port" dialog) and always reflects the file's
// current contents -- calling dotenv.Load a second time would not, since
// it skips any key already set from the first call at startup.
func currentHTTPAddr(installDir string) string {
	if v := os.Getenv("PUMC_HTTP_ADDR"); v != "" {
		return v
	}
	f, err := os.Open(filepath.Join(installDir, ".env"))
	if err != nil {
		return config.DefaultHTTPAddr
	}
	defer f.Close()
	kv, err := dotenv.Parse(f)
	if err != nil {
		return config.DefaultHTTPAddr
	}
	if v, ok := kv["PUMC_HTTP_ADDR"]; ok && v != "" {
		return v
	}
	return config.DefaultHTTPAddr
}

// resolveServerURL builds the URL the server (running with working
// directory installDir) actually listens on, for the clickable link in
// the tray window. PUMC_HTTP_ADDR's host is often "0.0.0.0" (all
// interfaces, the default) or empty, neither of which is a browsable
// address -- both are treated as "localhost", which is always correct
// for opening the console from the same machine the launcher runs on.
func resolveServerURL(installDir string) string {
	addr := currentHTTPAddr(installDir)

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "https://localhost"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "https://" + net.JoinHostPort(host, port)
}

// validatePort parses raw as a TCP port number (1-65535), trimming
// whitespace, and returns it re-formatted as a plain decimal string. The
// error message is meant to be shown directly in the "Change Port"
// dialog.
func validatePort(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("enter a port number between 1 and 65535")
	}
	return strconv.Itoa(n), nil
}
