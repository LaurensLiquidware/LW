# Changelog

## 0.1.5 — unreleased

- Fixed a real-detection bug in the Defender scan's verdict cross-check:
  `Invoke-DefenderScan.ps1` matched `Get-MpThreatDetection`'s `Resources`
  entries against the scanned path with a prefix match
  (`-like "$Path*"`), but those entries are URI-like
  (`file:_C:\mount\evil.exe`) or, for a detection inside a nested
  archive, a chain like `containerfile:_C:\mount\a.zip->...->eicar.com`
  -- neither ever starts with the bare scanned path, so the prefix
  match always failed and every real detection fell through to status
  `error` ("MpCmdRun.exe exited with code 2 but no matching detection
  was found...") instead of `threats-found`, even though MpCmdRun's own
  output (visible in the "Raw scan output" section added in 0.1.3)
  showed the detection plainly. Found from a real scan against a
  package containing an EICAR test file inside a `.zip`. Changed the
  match to `-like "*$Path*"` (the scanned path anywhere in the resource
  string) and confirmed against the exact resource string from that
  scan's output that the old pattern failed and the new one matches.
  This project's own `with-malware-scan.inventory.json` test fixture
  had encoded the same `file:_` prefix all along but never exercised
  the match against it, since fixture-based inventory tests only cover
  Go-side JSON decoding, not the PowerShell matching logic that
  produces the JSON in the first place.

## 0.1.4 — unreleased

- Made the Windows Defender scan detection-only:
  `Invoke-DefenderScan.ps1` now passes `-DisableRemediation` to
  `MpCmdRun.exe`, so Defender reports a detection instead of
  automatically quarantining/removing/cleaning it. Previously
  Defender's configured default action would run against the mounted
  package the moment something was flagged -- surprising for a tool
  whose job is to detect and report, not silently alter a customer's
  package, and actively risky for a classic VHDX, where Defender
  touching a file inside a live mount can corrupt the mount itself
  rather than just the flagged file. Verified with `pwsh` under strict
  error-action preference (this sandbox has no Defender install, so
  the flag itself can't be exercised end-to-end here; the change is a
  single added CLI argument, correctness confirmed against Microsoft's
  documented `MpCmdRun.exe` switches).

## 0.1.3 — unreleased

- Made the Windows Defender malware scan report what it actually did,
  not just the clean/threats-found verdict: the path scanned, when it
  started/finished and how long it took, the signature and engine
  versions Defender ran with, and (in a collapsible "Raw scan output"
  section) the actual `MpCmdRun.exe` output.
  `stage1-extract/Invoke-DefenderScan.ps1` now captures all of this
  (best-effort via `Get-MpComputerStatus` for the signature/engine
  info -- never fatal if unavailable), threaded through
  `internal/inventory.MalwareScan` → `pipeline.Result` (already
  exposed, no API shape change) → the Results screen, the CLI
  summary, and a new "Malware Scan (Windows Defender)" section in the
  PDF report (previously the PDF didn't mention the malware scan at
  all). Verified with `go test ./...`, `npx tsc --noEmit`,
  `npm run build`, and a live server + Playwright check of the
  Results screen and a rendered PDF against a threats-found fixture,
  confirming the new detail (path, duration, signature version,
  threats, raw output) renders correctly in both.

## 0.1.2 — unreleased

- Added native folder-picker Browse buttons to the Compare Scans
  screen's "Old Scan Output Folder" and "New Scan Output Folder"
  fields, matching the picker already on the New Scan screen (reuses
  `PickerService`/`GET /api/pick-folder`; degrades to plain text entry
  when the picker isn't available, e.g. non-Windows or the server run
  without the tray launcher). Verified with `npx tsc --noEmit` and
  `npm run build`.

## 0.1.1 — unreleased

- Renamed the product, in every user-facing surface, from "FlexApp
  Vulnerability Scanner" to "FlexApp Vulnerability and Security
  Scanner" -- reflecting that it now does more than CVE matching (see
  the Windows Defender malware scan above). Changed: the web app's
  `<title>` and i18n `app.title` (English and Dutch), the README
  heading, the tray app's window/status-bar title, the server's
  package doc comment, the PDF report's title text, the Windows
  `.exe` version-info metadata (`ProductName`/`FileDescription` --
  the internal names and actual `.exe` filenames are unchanged, so
  existing shortcuts/scripts/Scheduled Tasks keep working), the
  rewrite plan doc heading, and the generated
  `THIRD-PARTY-NOTICES.txt` title (also fixed a pre-existing bug
  where the underline below it was a hardcoded length that no longer
  matched the title -- it's now computed from the actual title
  length). Deliberately left unchanged, per the scope of this
  request: the Go module path and binary/package names
  (`flexapp-vuln-scanner*`) and the `flexapp-vuln-scanner-go/`
  directory name. Verified with `npx tsc --noEmit`, `npm run build`,
  `gofmt`, `go vet`, `go test ./...`, a full cross-compiled Windows
  build via `scripts/build-windows.sh`, and `strings` on the built
  `.exe` confirming the new name is embedded in the binary's
  version-info resource.

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
- Fixed the sidebar's Results link (and any bare `/results` visit)
  showing "No scan specified" even with finished scans on the
  Dashboard: `ResultsComponent` only ever looked at `?jobId=`/
  `?inventoryPath=`, which the sidebar link never sets. It now falls
  back to `GET /api/scans` (the same list the Dashboard shows, already
  sorted newest first) and opens the most recently created `done`/
  `error` scan -- live or historical -- updating the URL to match so
  refresh/share/back keep working. A genuinely empty history now shows
  a "No scans have finished yet" message with a New Scan button instead
  of the confusing "No scan specified." Verified with a live server (a
  historical scan-history.json entry, and separately an empty one) and
  Playwright, confirming both the auto-redirect and the empty state
  render correctly.
- Colorized the plain "6C / 29H / 27M / 4L" severity counts on the
  Results and Dashboard screens with a small dot per severity (using
  the existing `--cell-crit`/`--cell-elev`/`--cell-warn`/`--cell-good`
  heatmap tokens already in `tokens.css`, not invented colors) plus a
  "C = Critical, H = High, M = Medium, L = Low" legend, so the counts
  read at a glance instead of needing the letter abbreviations decoded
  every time.
- Retired the original Python implementation now that this rewrite is
  validated end-to-end on a real Windows machine (build order step 7 of
  `GO_ANGULAR_REWRITE_PLAN.md`, now moved into this project from
  `../flexapp-vuln-scanner/` so its history isn't lost): the entire
  `../flexapp-vuln-scanner/` directory (`webui/`, `desktop/`,
  `stage1-extract/` and `stage2-resolve/` reference implementations,
  `schemas/`, and its own planning docs) has been deleted, per explicit
  confirmation. Rewrote `README.md` to describe the finished tool as it
  actually runs today (the three binaries and what each is for, the
  Administrator requirement and why, `stage1-extract/`/`config/` needing
  to sit next to the binary, `.env.example`) instead of the mid-rewrite
  status it previously described, and updated `CLAUDE.md`'s Origin
  section to match. Verified `go vet`/`go test` still pass with the
  Python source gone -- nothing in this project ever read from it at
  build or run time, only from doc comments citing it as the origin of
  ported logic/tests, which remain accurate as historical attribution.
- Fixed a CI compliance-job failure caught on the first real run of the
  new GitHub Actions workflow: `scripts/generate-sbom.sh` fetched
  `cyclonedx-npm` via unpinned `npx --yes @cyclonedx/cyclonedx-npm`,
  which resolves whatever's newest on the npm registry at invocation
  time. A newer release started emitting per-component `cdx:npm:
  package:constraint:engine:*` properties the committed `bom.cdx.json`
  didn't have, making CI's "is the committed SBOM current" check fail
  on pure tooling drift, not a real dependency change. Pinned to
  `@cyclonedx/cyclonedx-npm@6.0.1` and regenerated `bom.cdx.json` with
  it so the committed file matches what CI now deterministically
  produces.
- Fixed `scripts/check-vulnerabilities.sh` hardcoding `python3`, found
  running it locally on a real Windows machine: a typical Windows
  install only has `python`, not `python3`, and running a bare
  `python3` there can silently invoke the Microsoft Store's alias shim
  instead of failing cleanly ("Python was not found; run without
  arguments to install from the Microsoft Store"). Now resolves a real,
  working interpreter (`python3` first, falling back to `python`) and
  fails with a clear message if neither works, instead of failing
  confusingly on this project's own actual target platform. Confirmed
  via CI (GitHub-hosted runners, real internet access) that this
  project has **zero Critical/High vulnerabilities** in its full Go +
  npm dependency graph -- the Sparks Tool checklist's last open item.
- Fixed a second Windows-only bug in `scripts/check-vulnerabilities.sh`,
  found immediately after the `python3`/`python` fix above on the same
  real Windows machine: `mktemp`'s output on Git Bash (MSYS) is a
  POSIX-style path (`/tmp/tmp.xxxx`). Grype itself (a native `.exe`)
  handles that fine -- MSYS auto-converts a standalone path-like argv
  entry when invoking a native executable -- but that path was also
  embedded as a substring inside a larger Python source string passed
  via `python -c`, which that auto-conversion doesn't touch, so the
  native Windows `python.exe` failed with `FileNotFoundError: No such
  file or directory: '/tmp/tmp.xxxx'` -- correct behavior given what it
  was actually asked to open, not a bug in Python. Converts the path via
  `cygpath -m` (present on Git Bash, a no-op everywhere else) before
  handing it to Python.
- Added a Windows Defender malware scan as a complementary signal
  alongside CVE matching: `stage1-extract/Invoke-DefenderScan.ps1` shells
  out to `MpCmdRun.exe` (ships with every Windows install, no extra
  dependency) to scan the mounted package's contents, then cross-checks
  `Get-MpThreatDetection` for a real, scoped verdict rather than trusting
  `MpCmdRun.exe`'s own undocumented exit code -- degrades gracefully to
  `unavailable`/`error` instead of failing Stage 1 outright when Defender
  isn't present or the scan can't be confirmed. The result
  (`clean`/`threats-found`/`unavailable`/`error`, plus threat names when
  found) flows through the new `inventory.MalwareScan` field into
  `pipeline.Result`, the Results screen (a colored status badge and
  threat list), the Dashboard (a compact badge column, persisted to
  `scanstore` so it survives restarts), and the CLI's summary output.
  Opt-out via `FVS_SKIP_DEFENDER_SCAN=true` or the CLI's
  `-skip-defender-scan` flag, or `-SkipDefenderScan` directly on the
  PowerShell script, for machines running a different antivirus product
  or where the extra scan time isn't wanted. Verified: new Go unit tests
  (`internal/inventory`, `internal/config`), a real `pwsh` syntax check
  and function-level exercise of the graceful-degradation path (this
  sandbox has no Defender, so this is the actual "unavailable" code path
  running for real, not simulated) -- which caught and fixed a genuine
  bug where `$env:ProgramData`/`$env:ProgramFiles` being unset would
  have thrown a terminating error under `Invoke-FlexAppInventory.ps1`'s
  `$ErrorActionPreference = 'Stop'` instead of degrading gracefully --
  and a live server + Playwright check of both the clean and
  threats-found states on Results and the Dashboard. The actual Windows
  Defender integration (the scan itself running against a real mounted
  VHDX) still needs verification on a real Windows machine, the same
  boundary every other Windows-only piece of this project has.
- Made the Windows Defender scan a per-scan choice instead of only a
  server-level/CLI-level default: the New Scan form now has a "Run
  Windows Defender malware scan" checkbox (checked by default, matching
  the scan-unless-skipped default), sent as `skipDefenderScan` on
  `POST /api/scans`. The deps-level `FVS_SKIP_DEFENDER_SCAN` (an
  operator-level "this install never scans" setting) is OR'd with the
  per-request value, so a global disable can't be silently overridden by
  a request that never asked to skip it.
- Redesigned the Scan Progress screen: the full raw log -- previously
  the main thing on the page, growing without bound -- is now a
  scrollable panel on the left (there to check if something failed, not
  something to stare at while a scan is healthy), and the main area
  shows a plain-language "what's happening right now" line (e.g. "Stage
  2: Querying OSV.dev") with a progress bar (indeterminate before the
  first progress event arrives, determinate once a done/total count is
  known) as the primary view. Verified with a live server + Playwright:
  the checkbox renders checked by default with no console errors, and
  the new two-column layout renders correctly for both an in-progress
  and a failed job (log panel showing the real `pwsh` invocation and
  output, main area showing the plain-language status and the error
  message).
- Branded the PDF report to match the rest of the app: a running
  Liquidware-blue (`--p-primary-600`) header banner with the white
  wordmark repeats on every page (rendered once from
  `web/frontend/src/assets/images/logo-primary-light.svg` -- the same
  asset the Angular header bar already uses over the same blue -- into
  `internal/report/assets/liquidware-logo-white.png`, since `fpdf` can't
  embed SVG directly), and every heading/table header uses the brand
  blue/dark-gray tokens instead of plain black. Verified by generating a
  real PDF (via the CLI's `-refresh` path) and viewing it rendered in
  Chromium: the banner and wordmark repeat correctly on both pages, and
  headings render in the correct brand color. Noted, not fixed as
  out-of-scope for this change: table headers already used a pre-existing
  em dash (`—`) that Helvetica's built-in Latin-1 encoding can't render,
  showing as `â€"` -- pre-existing, unrelated to branding.
