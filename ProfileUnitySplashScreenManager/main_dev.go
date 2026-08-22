//go:build !windows

package main

// Development entry point. The shipped product is Windows-only; this exists so
// the Go core and the HTTP API can be exercised on a development machine, which
// is also how the API integration tests drive a real server.
//
// The privileged and OS-specific operations -- the native file dialog, clipboard
// capture, launching the splash preview -- are stubbed here and return
// platform.ErrUnsupported.

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/liquidware/profileunity-splashscreen-manager/internal/api"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/platform"
)

func main() {
	plat := platform.New()
	o := parseFlags(plat)

	if o.showVersion {
		printVersion()
		return
	}
	api.SearchURLTemplate = o.searchURL

	srv, origin, err := setup(plat, o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer srv.Cleanup()

	fmt.Printf("development server: %s\n", origin)
	fmt.Printf("API token: %s\n", srv.Token())
	fmt.Printf("target dir: %s\n", o.targetDir)
	fmt.Println("this is not the shipped product; Windows-only features are stubbed")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}
