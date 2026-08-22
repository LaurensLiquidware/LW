//go:build windows

package main

import (
	"fmt"
	"os"

	"github.com/jchv/go-webview2"

	"github.com/liquidware/profileunity-splashscreen-manager/internal/api"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/platform"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/static"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/version"
)

// iconResourceID must match the icon id goversioninfo writes into the binary's
// resources, so the window and taskbar show the Liquidware mark.
const iconResourceID = 1

func main() {
	plat := platform.New()
	o := parseFlags(plat)

	if o.showVersion {
		printVersion()
		return
	}
	api.SearchURLTemplate = o.searchURL

	// Writing to Program Files needs elevation. The shipped executable carries a
	// requireAdmin manifest so Windows has already prompted by this point; this
	// covers a developer build that has no manifest.
	if elevated, err := plat.IsElevated(); err == nil && !elevated {
		if o.noElevate {
			platform.ShowMessage(version.ProductName,
				"This tool needs administrator rights to write to the ProfileUnity Client.NET folder, "+
					"and -no-elevate was passed. Restart it as an administrator.", true)
			os.Exit(1)
		}
		if err := platform.RelaunchElevated(os.Args[1:]); err != nil {
			platform.ShowMessage(version.ProductName,
				"This tool needs administrator rights to write to the ProfileUnity Client.NET folder.\n\n"+
					err.Error(), true)
			os.Exit(1)
		}
		// The elevated instance takes over.
		return
	}

	// A missing WebView2 runtime would otherwise present as a blank window with no
	// explanation. It ships with Windows 11 and patched Windows 10, but a
	// locked-down VDI image -- exactly where this tool runs -- may not have it.
	if plat.WebViewRuntimeVersion() == "" {
		platform.ShowMessage(version.ProductName,
			"The Microsoft Edge WebView2 Runtime is required but does not appear to be installed.\n\n"+
				"It is included with Windows 11 and with fully patched Windows 10. On an image that does "+
				"not have it, install the Evergreen Standalone Installer from Microsoft "+
				"(search for \"WebView2 Runtime download\"), then start this tool again.\n\n"+
				"If you believe the runtime is present, it may be installed per-user for a different "+
				"account than the one this tool is elevated as.", true)
		os.Exit(1)
	}

	srv, origin, err := setup(plat, o)
	if err != nil {
		platform.ShowMessage(version.ProductName, err.Error(), true)
		os.Exit(1)
	}
	defer srv.Cleanup()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  fmt.Sprintf("%s %s", version.ProductName, version.AppVersion),
			Width:  1040,
			Height: 780,
			IconId: iconResourceID,
			Center: true,
		},
	})
	if w == nil {
		platform.ShowMessage(version.ProductName,
			"The WebView2 window could not be created. The Edge WebView2 Runtime may be present but "+
				"damaged; reinstalling it usually fixes this.", true)
		os.Exit(1)
	}
	defer w.Destroy()

	// The native file dialog runs a modal message loop and must be opened from the
	// UI thread, not from an HTTP handler's goroutine.
	platform.SetDispatcher(plat, w.Dispatch)

	// Hand the page its API token. Injecting it here rather than putting it in a
	// URL keeps it out of history, referrers and logs, and a page in the user's
	// browser has no way to obtain it.
	w.Init(fmt.Sprintf(
		"window.__PSM__ = Object.freeze({ token: %q, origin: %q, version: %q, placeholderUI: %v });",
		srv.Token(), origin, version.AppVersion, static.IsPlaceholder()))

	w.Navigate(origin)
	w.Run()
}
