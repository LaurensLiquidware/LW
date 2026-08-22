// Command ProfileUnitySplashScreenManager sets the ProfileUnity client splash
// screen logo, per Liquidware KB 12914471137293.
//
// The user interface is an Angular application embedded in this executable and
// rendered in a native WebView2 window; all privileged work happens in Go. The
// two halves talk over a loopback-only HTTP API guarded by a per-run token --
// see internal/api for the reasoning.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/liquidware/profileunity-splashscreen-manager/internal/api"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/logo"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/platform"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/static"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/version"
)

// dataDirName is where history and the manifest live. Deliberately outside
// Client.NET so they survive a ProfileUnity client reinstall or upgrade.
var dataDirRelative = filepath.Join("Liquidware", "ProfileUnitySplashScreenLogoManager")

type options struct {
	targetDir   string
	showVersion bool
	searchURL   string
	noElevate   bool
}

func parseFlags(plat platform.Platform) options {
	var o options
	flag.StringVar(&o.targetDir, "target-dir", plat.DefaultTargetDir(),
		"ProfileUnity Client.NET folder to write the logo into. Point this elsewhere to stage a logo before deployment.")
	flag.BoolVar(&o.showVersion, "version", false, "print the version and exit")
	flag.StringVar(&o.searchURL, "search-url", api.SearchURLTemplate,
		"image-search URL template; %s is the encoded query. Pass an empty string to disable image search entirely, for air-gapped or policy-restricted sites.")
	flag.BoolVar(&o.noElevate, "no-elevate", false,
		"do not attempt to relaunch elevated; fail instead if not already elevated")
	flag.Parse()
	return o
}

// setup builds the store, the server and the listener, and returns the origin.
func setup(plat platform.Platform, o options) (*api.Server, string, error) {
	store := &logo.Store{
		TargetDir: o.targetDir,
		DataDir:   filepath.Join(plat.ProgramData(), dataDirRelative),
	}
	if err := store.EnsureDirs(); err != nil {
		return nil, "", err
	}

	ui, err := static.UI()
	if err != nil {
		return nil, "", fmt.Errorf("the embedded user interface could not be opened: %w", err)
	}

	exeDir := "."
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	docs := api.Docs{
		LicensePDF: filepath.Join(exeDir, "Spark_License.pdf"),
		SBOM:       filepath.Join(exeDir, "bom.cdx.json"),
		Notices:    filepath.Join(exeDir, "THIRD-PARTY-NOTICES.txt"),
	}

	srv, err := api.New(store, plat, ui, docs)
	if err != nil {
		return nil, "", err
	}

	ln, err := srv.Listen()
	if err != nil {
		return nil, "", fmt.Errorf("could not open the local listener: %w", err)
	}
	origin := fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
	go srv.Serve(ln, origin)
	return srv, origin, nil
}

func printVersion() {
	fmt.Printf("%s %s\n", version.ProductName, version.AppVersion)
}
