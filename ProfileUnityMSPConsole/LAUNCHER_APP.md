# Windows Tray Launcher — Reusable Pattern

A pure-Go Windows system-tray launcher that wraps a headless server binary — gives double-click users a GUI (Start/Stop/Restart, live log viewer, tray icon) instead of a bare console. Zero cgo, cross-compiles from Linux with plain `go build`.

Built for the ProfileUnity MSP Licensing Console (`cmd/tray/`); this doc summarizes it for reuse in another project.

## Architecture: two binaries

- **`<app>.exe`** (tray launcher, `cmd/tray`) — GUI, `-ldflags "-H=windowsgui"` (no console window)
- **`<app>-server.exe`** (the real headless server, unchanged) — what the launcher spawns as a child process, and what anyone running it from PowerShell/a Scheduled Task/a Windows Service should call directly

This split matters: the launcher never contains business logic, it's purely a process-control shell.

## Key library

`github.com/lxn/walk` — pure-Go native Win32 UI (`walk.MainWindow`, `PushButton`, `TextLabel`, `TextEdit`, `NotifyIcon`, `ImageView`, `LinkLabel`, `Icon`, `Bitmap`). No cgo, confirmed buildable with `CGO_ENABLED=0 GOOS=windows GOARCH=amd64` from a Linux sandbox with no mingw.

## Files to copy (all in `cmd/tray/`)

| File | Purpose |
|---|---|
| `main_windows.go` | The whole app — see below |
| `main_other.go` (`//go:build !windows`) | Stub so `go build ./...`/`go vet`/`go test` still work on non-Windows dev machines/CI |
| `logpath.go` | Resolves the log file path (no build tag — testable without Windows) |
| `serverurl.go` | Resolves the URL the server listens on, for the clickable link (no build tag) |
| `app.manifest` | comctl32 v6 + per-monitor DPI awareness |
| `icon.ico` | App/window/tray icon |
| `logo.png` | Optional wordmark, must have **true alpha transparency** if embedded (see gotcha below) |

Plus `scripts/build-windows.sh` for the cross-compile + resource embedding step.

## `main_windows.go` — the parts worth reusing verbatim

1. **Panic/error visibility is the single most important pattern.** A `-H=windowsgui` binary has no console — `fmt.Println` and default panic output are completely invisible if double-clicked. Every failure path must go through a native `windows.MessageBox` dialog instead:
   ```go
   func fatal(context string, err error) { showMessageBox(...); os.Exit(1) }
   func warnDialog(context string, err error) { showMessageBox(...) } // non-fatal
   ```
   Wrap `main()` in `defer recover()` → `fatal(...)` too. Skipping this means a startup bug just looks like "nothing happens."

2. **Explicit window size.** `walk.MainWindow` has no size-to-content default — without `mw.SetSize(...)`, it opens huge with `VBoxLayout` spreading gaps between every child. Always set both `SetMinMaxSize` and `SetSize`.

3. **Process control of the child server**, via `os/exec` + `syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW}` — spawns without a visible console but still signalable. Stop via `windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, pid)` with a hard-kill fallback after a timeout. This is the standard technique for a GUI-subsystem parent controlling a console-subsystem child.

4. **Minimize-to-tray**: intercept `mw.Closing()`, cancel it, and `mw.Hide()` instead — only the tray menu's "Exit" actually quits.

5. **Live log tailing**: poll `os.Stat` on the log file every 500ms, read only the new bytes (seek from last offset), append into a read-only `walk.TextEdit`, trim the on-screen buffer past a rune cap so it doesn't grow unbounded. Marshal all UI updates via `mw.Synchronize(func(){...})` since this runs on a background goroutine.

6. **Icon/logo loading**: `walk` has no "load icon from bytes" API — write to a temp file, load, delete. A `Bitmap` *can* be built directly from a decoded `image.Image` via `walk.NewBitmapFromImage`.

## The one gotcha: transparent PNGs

If you rasterize a logo from SVG via headless Chromium, you **must** pass `omitBackground: true` to `Page.captureScreenshot` (or `Emulation.setDefaultBackgroundColorOverride` with alpha 0) — otherwise Chromium bakes an opaque white background into the PNG even with CSS `background:transparent`. `walk`'s rendering (`Bitmap.drawStretched` → `alphaBlend`, real Win32 `AlphaBlend`) handles true alpha correctly, so this is purely an asset-generation issue, not a code one.

## Build script pattern (`scripts/build-windows.sh`)

Uses `github.com/josephspurrier/goversioninfo` to embed an icon + version metadata (+ the manifest, only for the tray binary) as a `.syso` resource file per package directory, which `go build` picks up automatically for `GOOS=windows GOARCH=amd64`. One parameterized `generate_syso()` bash function handles both binaries.

For reuse: swap `appTitle`, `serverExeName`, the icon/logo assets, and the env-var names in `logpath.go`/`serverurl.go` for whatever config convention the new project uses — everything else transfers as-is.
