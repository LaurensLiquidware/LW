# Changelog

## 0.1.0 — unreleased

- Initial skeleton: Go backend (`cmd/server` + `cmd/tray`) + Angular
  frontend scaffold, copied from `ProfileUnityMSPConsole`'s project
  layout and stripped down to a local, single-user, no-auth, no-database
  tool (health/version endpoints only so far). See
  `../flexapp-vuln-scanner/GO_ANGULAR_REWRITE_PLAN.md` for the full
  rewrite plan and build order; this commit is build-order step 1.
- Ported the Python Stage 2 business logic to Go (build-order step 2):
  `internal/inventory` (Stage 1 contract structs/loader), `internal/cpemap`
  (`cpe-mappings.yaml` overrides), `internal/normalize` (purl/CPE
  building), `internal/coverage` (resolution coverage), `internal/osv`
  and `internal/nvd` (OSV.dev/NVD 2.0 clients, on-disk cache, NVD rate
  limiting), `internal/resolve` (the OSV/NVD matching orchestration),
  `internal/sbom` (per-scan CycloneDX SBOM), `internal/report`
  (coverage/findings Markdown, CSV, PDF via `go-pdf/fpdf`, scan diffing),
  and `internal/pipeline` (Stage 1 subprocess + Stage 2 orchestration,
  existing-result loading, scan diffing). Verified with Go unit tests
  ported 1:1 from the Python test suite (same assertions, several
  against the same fixture) for output parity, plus a manual end-to-end
  run producing a real SBOM/coverage/findings/PDF from the shared sample
  fixture. HTTP API wiring (build-order step 3) is not done yet — these
  packages aren't reachable from `cmd/server` yet.
- Wired the ported pipeline into `cmd/server`'s HTTP API (build-order
  step 3): `internal/httpapi/jobs.go` (`ScanJob`/`JobRegistry`, an
  in-memory adapter to `pipeline.ProgressSink` mirroring
  `../flexapp-vuln-scanner/webui/jobs.py`'s `ScanJob`/`JobRegistry`
  exactly) and `internal/httpapi/scans.go` (`POST /api/scans` start,
  `POST /api/scans/refresh`, `GET /api/scans` list, `GET /api/scans/{id}`
  poll, `GET /api/scans/{id}/files/{kind}` download — restricted to the
  job's own known report paths, never an arbitrary caller-supplied
  path —, `POST /api/scans/open`, `POST /api/scans/compare`). All
  JSON-serialized types (`pipeline.Result`, `pipeline.Diff`,
  `report.FindingRow`) now carry explicit camelCase JSON tags for a
  clean Angular-facing API contract. Verified with new `httpapi` unit
  tests plus a real end-to-end run: started the server, hit
  `/api/scans/open` against the shared fixture and got real coverage
  data back, started a scan and watched it surface Stage 1's missing-
  script error through `/api/scans` polling exactly as the job model
  intends. Not done yet: SSE/streaming progress (clients currently must
  poll `GET /api/scans/{id}`), and the Angular screens that will call
  any of this — they still show "Coming Soon" placeholders.
- Built the real Angular screens against the scan API (build-order step
  4): Dashboard (polling job list), New Scan (form with auto-filled
  output folder, Advanced NVD API key section), Scan Progress (polls a
  job, hands off to Results on completion), Results (coverage summary,
  severity counts, findings table with an expandable "Affected Files"
  disclosure per finding — mirrors the Flask web UI's
  `affected_files_cell` macro — and PDF/SBOM/CSV download links), and
  Compare (old/new scan diff). Added `core/scan.service.ts` +
  `core/models/scan.ts` as the typed API client, and a `saveError`/
  `saveErrorIsRaw`-style pair on every form (matching
  `ProfileUnityMSPConsole`'s settings screen convention) so translated
  app copy and raw backend error text never get conflated. Verified:
  `ng build` succeeds, `tsc --noEmit` is clean, and a real Playwright
  pass against the running server confirms every screen renders
  correctly — including a real end-to-end Results render against the
  shared fixture (66.7% coverage, 2/3 resolved, correct "no vuln data"
  notice) and a caught-and-fixed bug where an untranslated error case
  showed a raw i18n key instead of its message.
- Verified Windows packaging (build-order step 5) and regenerated the
  real Sparks Tool compliance artifacts (build-order step 6, partial):
  `scripts/build-windows.sh` actually cross-compiles from this Linux
  dev environment -- confirmed both `flexapp-vuln-scanner.exe` (GUI
  subsystem) and `flexapp-vuln-scanner-server.exe` (console subsystem)
  come out as valid Windows PE32+ binaries with the embedded icon/
  version resource. Regenerated `bom.cdx.json` (120 real components: 3
  Go modules + 117 npm packages, via `cyclonedx-gomod` + `cyclonedx-npm`)
  and `THIRD-PARTY-NOTICES.txt` from this project's actual dependency
  graph, replacing the stale `ProfileUnityMSPConsole` copies -- also
  fixed `generate-notices.py`'s hardcoded MSP Console title/font note
  (this project has no bundled PDF font; `go-pdf/fpdf` uses built-in
  Helvetica). Verified the About screen serves all three real artifacts
  end-to-end via a live server + Playwright screenshot. Not done:
  running Grype against the SBOM for the checklist's zero-Critical/High
  requirement -- blocked by this environment's network egress policy
  (same restriction already documented for
  `FlexAppOneDownloadMonitor`'s Sparks audit); needs a
  network-unrestricted machine.
- Added scan-history persistence across server restarts:
  `internal/scanstore` (ported from
  `../flexapp-vuln-scanner/desktop/recent_scans_store.py`) is a flat
  JSON file of lightweight scan-list rows (id, package path, status,
  and once done, package name/coverage/severity counts/inventory
  path). `ScanDeps` now persists to it alongside the in-memory
  `JobRegistry`, and `GET /api/scans` merges live jobs from this
  process with any persisted rows from a previous process run (marked
  `live: false`, no log/full result -- the dashboard opens those via
  their saved `inventoryPath` instead of a job id, since there's no
  live job left to poll). Fixes a real regression the Go rewrite had
  introduced versus the PySide6 desktop app: the dashboard previously
  went blank on every server restart. Verified: new Go unit tests
  (ported 1:1 from `test_recent_scans_store.py`, plus one covering the
  live/historical merge) all pass, and a real two-process manual test
  (start a scan, kill the server, start a fresh one) confirms the
  scan reappears with `live: false` and its error/status intact.
- Implemented real scan cancellation, resolving the open question
  flagged (unresolved) in both the PySide6 desktop app and this
  rewrite's plan. `context.Context` is now threaded through
  `pipeline.RunStage1`/`RunStage2`, `resolve.Resolve`, and the
  `osv`/`nvd` clients' HTTP calls; each `ScanJob` carries a
  `context.CancelFunc`, and a new `POST /api/scans/{id}/cancel`
  endpoint calls it. Canceling kills the Stage 1 `pwsh` subprocess
  outright (`exec.CommandContext`) or aborts an in-flight OSV/NVD HTTP
  request and stops the matching loop between items, landing the job
  in a new `canceled` status (distinct from `error`), persisted the
  same way. Added a Cancel button to the Scan Progress screen.
  Verified with two new tests that prove this is real, not just
  wired up: `TestRunStage1_CancelStopsSubprocessPromptly` cancels a
  30-second-sleeping real `pwsh` subprocess and asserts it returns in
  well under that, and `TestCancelScanHandler_StopsARunningScan` does
  the same through the HTTP handler end-to-end. Also manually verified
  via curl and a real Playwright browser session (screenshots
  reviewed): clicking Cancel in the UI genuinely stops the running
  subprocess within about a second.
- Added Server-Sent Events streaming for scan progress, replacing the
  Angular Scan Progress screen's polling loop with a push-based one:
  `GET /api/scans/{id}/events` (`internal/httpapi/scans.go`'s
  `SSEScanHandler`) sends an immediate snapshot, then a new one each
  time `ScanJob`'s new `version` mutation counter changes, closing the
  stream on a terminal status. The Angular side now opens an
  `EventSource` and falls back to the old polling loop if the
  connection never delivers a single message (e.g. a proxy that
  strips SSE). Verified with a new test
  (`TestSSEScanHandler_StreamsUpdatesUntilTerminal`, a real streaming
  `httptest.Server` request parsing `data:` lines), a real `curl -N`
  capture showing progressive log delivery, and Playwright screenshots
  of a live scan.
- Added a CLI mode (`cmd/cli`, `flexapp-vuln-scanner-cli`) for
  scripted/CI/cron use with no HTTP server involved: `-package` or
  `-refresh` plus `-output` run Stage 1 and/or Stage 2 directly against
  the same `internal/pipeline` code the server uses, printing progress
  and a final summary to stdout and honoring `Ctrl+C`/SIGTERM via
  `context`. Added to `Makefile`'s `build` target and
  `scripts/build-windows.sh`'s Windows cross-compile (now producing
  three binaries: tray, server, CLI). Verified with new tests
  (`cmd/cli/main_test.go`, including a real end-to-end run against a
  synthetic empty inventory that produces all six report files with no
  network access) and a real Windows cross-compile confirmed to
  produce a valid console-subsystem `flexapp-vuln-scanner-cli.exe`.
- Added the Grype CVE gate from the Sparks Tool checklist:
  `scripts/check-vulnerabilities.sh` runs Grype against `bom.cdx.json`
  and fails the build on any Critical/High match. Confirmed working
  end-to-end in this dev environment except for one specific, already-
  documented limitation: Grype's own vulnerability-database download
  (`grype.anchore.io`) is blocked by this sandbox's network egress
  policy, the same restriction affecting OSV/NVD elsewhere in this
  project. The script detects this exact failure mode (rather than a
  real scan failure) and exits non-zero with an explicit message
  instead of silently reporting a clean bill of health — verified by
  running it for real against the actual SBOM and observing that exact
  message. `scripts/release.sh` (previously an unmodified, partly
  broken copy of `ProfileUnityMSPConsole`'s release script — dead
  references to `.env.example`, `docs/MANUAL.md`, `cmd/gendemodb`)
  now calls this script as its CVE gate in place of `govulncheck`, adds
  a real `.env.example` documenting every `FVS_*` setting, drops the
  manual-PDF and demo-database steps (neither applies to this project),
  and packages `flexapp-vuln-scanner-cli`/`.exe` into both release
  zips.
- Added a native file/folder picker for the New Scan screen's Package
  Path and Output Folder fields, resolving the last open item from the
  rewrite plan. `GET /api/pick-file` and `GET /api/pick-folder`
  (`internal/httpapi/picker_windows.go`) show a real Win32 dialog via
  `lxn/walk`'s `FileDialog` (already `cmd/tray`'s GUI dependency)
  directly in the server process -- `GetOpenFileName`/
  `SHBrowseForFolder` are blocking calls that pump their own message
  loop, so no `walk.MainWindow` or a separate picker process is needed,
  and the picker works whether or not the tray launcher is running.
  `GET /api/config` reports whether this build supports it (`false` on
  the `picker_other.go` non-Windows stub); the New Scan screen's Browse
  buttons only render when it does. An initial version of this wrongly
  ran the picker as a separate loopback HTTP server hosted by
  `cmd/tray`, requiring cross-origin fetches from the browser and not
  working when the server was run without the tray -- corrected before
  it shipped, per real-world testing. Verified: new Go unit tests
  (`internal/httpapi/appconfig_test.go`, `picker_other_test.go`), a
  real Windows cross-compile of all three binaries, and a live server +
  Playwright check confirming the Browse buttons correctly stay hidden
  (zero rendered, no console errors) on this non-Windows build.
- Fixed a packaging bug caught in real Windows testing: the release
  bundle never included `stage1-extract/` (the PowerShell Stage 1
  script + its supporting modules) or `config/` (`cpe-mappings.yaml`),
  both read from disk next to the running binary at their `config.go`
  default paths -- every real scan failed with "Stage 1 script not
  found" because nothing had ever copied `stage1-extract/` into
  `flexapp-vuln-scanner-go/` in the first place, let alone into a
  release zip. `stage1-extract/` (unchanged from
  `../flexapp-vuln-scanner/`, minus its Pester tests) is now committed
  to this project, and `scripts/release.sh` packages both directories
  into every release zip.
- Fixed a real Windows scan failure caught testing the above:
  "WARNING: Failed to process '...': A required privilege is not held
  by the client" followed by "Stage 1 finished but no
  '<package>.inventory.json' path was found." Stage 1 mounts the VHDX
  being scanned (`Mount-DiskImage` + `Add-PartitionAccessPath`, see
  `stage1-extract/Mount-ClassicFlexApp.ps1`), which requires
  Administrator -- neither the tray launcher nor the server were
  requesting elevation. `cmd/tray/app.manifest` and the new
  `cmd/server/app.manifest` now both set
  `requestedExecutionLevel="requireAdministrator"`
  (`scripts/build-windows.sh` embeds the server's via `goversioninfo`
  the same way the tray's already was), so double-clicking either shows
  one UAC prompt and every child process inherits the elevated token.
  `cmd/cli` deliberately keeps no such manifest -- a UAC prompt has
  nothing to answer it in the unattended contexts the CLI is for, so
  its doc comment now says to run it from an already-elevated context
  (a Scheduled Task set to "Run with highest privileges", or an admin
  shell) instead.
- Added an Exit button to the tray launcher's main window, next to Show
  Log, matching `ProfileUnityMSPConsole`'s launcher layout -- Exit was
  previously reachable only from the tray icon's right-click menu, a
  discoverability gap flagged in real Windows testing.
- Fixed a broken launcher caught immediately after the elevation manifest
  change above: "The application has failed to start because its
  side-by-side configuration is incorrect" on `flexapp-vuln-scanner.exe`.
  Two real manifest bugs, both from the same edit: the new `trustInfo`
  element was placed after `dependency`/`application`, violating the
  order `<assembly>`'s children must appear in per the manifest schema;
  and its explanatory comment used a literal double hyphen (`--`)
  several times, which XML comments cannot contain at all -- either one
  on its own is enough to make the Windows manifest parser reject the
  whole file and refuse to start the process. Fixed the ordering in
  `cmd/tray/app.manifest` (`trustInfo` now directly follows
  `assemblyIdentity`, matching Visual Studio's own generated
  `requireAdministrator` template) and removed every double hyphen from
  both manifest files' comments. Verified with `xmllint --noout` against
  both files (clean) and a real Windows cross-compile; full end-to-end
  launch behavior still needs confirming on a real Windows machine.
