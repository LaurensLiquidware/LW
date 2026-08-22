//go:build !windows

package main

import "fmt"

// This tray launcher wraps flexapp-vuln-scanner-server.exe with a
// Windows GUI (window, buttons, system tray icon, log viewer) — it has
// no reason to exist on any other OS. This stub only exists so
// `go build ./...`/`go vet ./...`/`go test ./...` keep working
// uniformly on non-Windows dev/CI machines; the real implementation is
// in main_windows.go.
func main() {
	fmt.Println("flexapp-vuln-scanner tray launcher is Windows-only; run flexapp-vuln-scanner-server directly on this platform.")
}
