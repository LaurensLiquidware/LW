# FlexApp Vulnerability Scanner (Go + Angular)

A local, single-user, no-auth Windows tool that scans FlexApp packages
(classic VHDX or Package Manager ZIP) for known-vulnerable third-party
components: it inventories the package's files (Stage 1, PowerShell),
matches them against OSV.dev and NVD 2.0 (Stage 2, this Go backend), and
produces a coverage report, a findings table, a per-scan SBOM, and a PDF
report.

Go backend + Angular 21/PrimeNG 21 frontend (embedded via `go:embed`),
packaged as native Windows `.exe` files. Rebuilt from an original Python
implementation (Flask web UI + PySide6 desktop app), which has been fully
retired and deleted now that this rewrite is validated end-to-end on a
real Windows machine — see `GO_ANGULAR_REWRITE_PLAN.md` for the full
rewrite history, the Python→Go component mapping, and the build order
this project followed.

**Status:** feature-complete and validated on Windows. Real scans against
real VHDX packages, real UAC-elevated Stage 1 (VHDX mounting), real
native file/folder picker, live SSE-streamed progress, scan history that
survives restarts, cancellation, a Windows Defender malware scan of the
mounted package, and a scripted/CI-friendly CLI all work end-to-end.

## Running it

Three binaries, three jobs:

- **`flexapp-vuln-scanner.exe`** — the tray launcher. Double-click this
  one. Starts/stops/restarts the server, shows a live log, hosts the
  system tray icon, and exposes the native file/folder picker the New
  Scan screen's Browse buttons use.
- **`flexapp-vuln-scanner-server.exe`** — the headless server the
  launcher spawns as a child process. Run this directly instead if you
  want a Scheduled Task or Windows Service, rather than the tray UI.
- **`flexapp-vuln-scanner-cli.exe`** — scripted/CI/cron use: runs a scan
  (or a Stage 2-only refresh) from the terminal, no HTTP server or
  browser involved. See its `-h` output for flags.

**Administrator is required** for the tray launcher and the server (not
the CLI): Stage 1 mounts the VHDX being scanned
(`Mount-DiskImage`/`Add-PartitionAccessPath`), which needs elevated
privileges. Both `.exe` files carry a `requireAdministrator` manifest, so
Windows shows one UAC prompt on launch. The CLI deliberately carries no
such manifest, since a UAC prompt can't be answered in an unattended
context — run it from an already-elevated shell, or a Scheduled Task set
to "Run with highest privileges".

**`stage1-extract/` and `config/` must sit next to whichever binary you
run** (both are included in the release zip, alongside the binaries) —
they're read from disk at fixed relative paths
(`./stage1-extract/Invoke-FlexAppInventory.ps1`,
`./config/cpe-mappings.yaml`, both overridable via `FVS_STAGE1_SCRIPT`/
`FVS_CPE_MAPPINGS_PATH`), not embedded into the binary. Missing either
produces a clear error rather than a silent failure.

Copy `.env.example` to `.env` next to the binary to override any
`FVS_*` setting (HTTP port, log level, cache/output directories, NVD API
key, ...) without exporting environment variables by hand — see
`internal/dotenv` and `.env.example` itself for the full list.

## Malware scanning

This tool's core job is CVE matching (known-vulnerable component
versions), which is a different problem from malware detection
(malicious code that isn't a "vulnerability" in any known-good library).
As a complementary signal, Stage 1 also runs a Windows Defender scan of
the mounted package's contents (`stage1-extract/Invoke-DefenderScan.ps1`,
shelling out to `MpCmdRun.exe` — no extra install, since Defender ships
with every Windows machine) and reports the verdict (`clean`,
`threats-found`, `unavailable` if Defender isn't present, or `error`)
alongside the CVE findings on the Results screen, the Dashboard, and in
the CLI's summary output. It degrades gracefully to `unavailable` rather
than failing the scan on a machine without Defender (or one running a
different antivirus product instead), and can be skipped outright with
`FVS_SKIP_DEFENDER_SCAN=true` or the CLI's `-skip-defender-scan` flag.

This is one additional signal, not a certification -- it only catches
what Defender's current signatures recognize, on whatever definitions
happen to be installed on the scanning machine.

## Build (from source)

```sh
make build   # builds the Angular frontend, then the server + CLI binaries (this platform only)
make test    # go vet + go test
make run     # go run ./cmd/server
```

## Windows packaging

`scripts/build-windows.sh` cross-compiles `cmd/tray`, `cmd/server`, and
`cmd/cli` as Windows `.exe` files with embedded icon/version metadata and
(for the tray and server) the `requireAdministrator` manifest, mirroring
`ProfileUnityMSPConsole`'s packaging approach. `scripts/release.sh` runs
the full pipeline (sync version/legal, build frontend, vet/test,
regenerate the SBOM, run the Grype CVE gate, build every binary, produce
both a Linux and a Windows release zip bundling `stage1-extract/`,
`config/`, `.env.example`, and the compliance artifacts below).

## Version

`VERSION` is the single source of truth. Run `scripts/sync-version.sh`
after changing it, before building.

## Legal / compliance (Sparks Tool checklist)

`Spark_License.pdf`, `bom.cdx.json`, and `THIRD-PARTY-NOTICES.txt` at the
repo root are embedded into the binary (`internal/legal`) and served at
fixed paths for the About screen. `bom.cdx.json` (120 components: 3 Go
modules + 117 npm packages) and `THIRD-PARTY-NOTICES.txt` are generated
from this project's real dependency graph via `scripts/generate-sbom.sh`
(requires `cyclonedx-gomod` on `PATH`) and `scripts/generate-notices.py`
— re-run both, then `scripts/sync-legal.sh`, whenever dependencies
change.

`scripts/check-vulnerabilities.sh` runs Grype against `bom.cdx.json` for
the checklist's "zero Critical/High CVEs" requirement, and fails loudly
(rather than reporting a false clean bill of health) if Grype's own
vulnerability database is unreachable -- which is exactly what happens
in this project's own dev sandbox, since it blocks `grype.anchore.io`
(the same restriction already documented for
`FlexAppOneDownloadMonitor`'s Sparks audit). CI (GitHub-hosted runners,
normal internet access) and a real Windows machine have both confirmed
the actual verdict: **zero Critical/High vulnerabilities** in the full
Go + npm dependency graph.
