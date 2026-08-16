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
	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/dotenv"
)

// lookupEnvFile returns key's configured value for the install at
// installDir and whether one was actually set -- a real environment
// variable wins (matching internal/dotenv.Load's own "env always wins
// over file" rule), otherwise installDir/.env's value if present.
// Unlike dotenv.Load, this never mutates process environment, so it's
// safe to call again after changing a value (e.g. from cmd/tray's
// "Change Port" dialog) and always reflects the file's current contents
// -- calling dotenv.Load a second time would not, since it skips any key
// already set from the first call at startup. The "was it actually set"
// distinction (as opposed to a caller's own fallback default) is what
// lets applyDemoDefaults tell "nothing configured yet" apart from "an
// operator explicitly chose this value".
func lookupEnvFile(installDir, key string) (string, bool) {
	if v := os.Getenv(key); v != "" {
		return v, true
	}
	f, err := os.Open(filepath.Join(installDir, ".env"))
	if err != nil {
		return "", false
	}
	defer f.Close()
	kv, err := dotenv.Parse(f)
	if err != nil {
		return "", false
	}
	v, ok := kv[key]
	return v, ok && v != ""
}

// explicitHTTPAddr returns PUMC_HTTP_ADDR's configured value for
// installDir and whether one was actually set. See lookupEnvFile.
func explicitHTTPAddr(installDir string) (string, bool) {
	return lookupEnvFile(installDir, "PUMC_HTTP_ADDR")
}

// currentHTTPAddr returns the host:port the server at installDir is
// configured to listen on, falling back to config.DefaultHTTPAddr if
// nothing is explicitly configured.
func currentHTTPAddr(installDir string) string {
	if v, ok := explicitHTTPAddr(installDir); ok {
		return v
	}
	return config.DefaultHTTPAddr
}

// currentDBDSN returns the database DSN the server at installDir is
// configured to use, falling back to config.DefaultDBDSN if nothing is
// explicitly configured -- mirrors currentHTTPAddr's own resolution
// order for PUMC_DB_DSN.
func currentDBDSN(installDir string) string {
	if v, ok := lookupEnvFile(installDir, "PUMC_DB_DSN"); ok {
		return v
	}
	return config.DefaultDBDSN
}

// demoDBPath returns the absolute path where a demo.db sidecar would
// live for this install -- db.DemoSidecarPath's directory rule, resolved
// relative to installDir (not the tray's own working directory), since
// the server this launcher spawns always runs with cmd.Dir set to
// installDir (see main_windows.go), so a relative PUMC_DB_DSN is
// relative to that directory.
func demoDBPath(installDir string) string {
	dsn := currentDBDSN(installDir)
	if !filepath.IsAbs(dsn) {
		dsn = filepath.Join(installDir, dsn)
	}
	return db.DemoSidecarPath(dsn)
}

// demoDBPresent reports whether a demo.db sidecar exists for this
// install's configured database.
func demoDBPresent(installDir string) bool {
	_, err := os.Stat(demoDBPath(installDir))
	return err == nil
}

// demoModeHTTPAddr is the port a demo.db install defaults to -- distinct
// from the normal 8443 default, so a demo copy and a production copy can
// run side by side out of the box (see the tray's "Change Port" feature,
// built for exactly this, before this existed to set it automatically).
const demoModeHTTPAddr = "0.0.0.0:8444"

// applyDemoDefaults seeds PUMC_HTTP_ADDR to demoModeHTTPAddr the first
// time a demo.db sidecar is detected for this install, but only if no
// port has been explicitly configured yet -- so this never clobbers a
// port an operator already chose, whether manually or from an earlier
// run of this same logic (once written, the value is "explicit" from
// then on, so it's a one-time seed, not a persistent override). No-ops
// if demo.db isn't present. Call once, at startup, before the first
// resolveServerURL/title computation.
func applyDemoDefaults(installDir string) {
	if !demoDBPresent(installDir) {
		return
	}
	if _, explicit := explicitHTTPAddr(installDir); explicit {
		return
	}
	_ = dotenv.SetValue(filepath.Join(installDir, ".env"), "PUMC_HTTP_ADDR", demoModeHTTPAddr)
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
