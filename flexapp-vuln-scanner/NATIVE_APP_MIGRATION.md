# Native Windows App Migration Plan (DRAFT — awaiting confirmation)

This is a proposal, not a decision. Following this repo's own working
agreement (audit/plan first, confirm, then implement), nothing here has
been built yet. See the mockup for what these screens would look like:
https://claude.ai/code/artifact/61d0073c-6a0a-4133-bf11-a80088e633f7

## Why

The current interface is a local Flask web app: it starts a server on
`127.0.0.1:5000`, and you drive it from a browser tab. That works, but
it's not what "a Windows app" means to most users - there's a server
process to keep alive, a port, a browser tab that looks like every other
web page, and a custom-built file/folder browser (`browse.py`) because a
web page can't call the OS's real file picker.

## Recommended approach: PySide6 (Qt for Python)

**Rationale**: this keeps 100% of `stage2-resolve/flexapp_vuln/` -
`resolve_vuln_matches`, `reporting.py`, `sbom.py`, `pdf_report.py`,
`cli.py`, `nvd_client.py`, `osv_client.py`, `coverage.py`,
`cpe_mappings.py` - completely untouched, along with its 121 existing
tests. Stage 1 keeps shelling out to `pwsh` exactly as it does today.
Only the presentation layer changes: Flask/HTML/CSS is replaced by a
real native window, with no browser and no server process.

**Alternative considered and not recommended**: a full WPF/.NET (C#)
rewrite. This gets the most "native Windows" look (WinUI/Fluent styling,
MSIX packaging, no Python runtime to bundle) but means porting every
line of Stage 2's OSV/NVD matching, SBOM building, and PDF/report
generation from Python to C# from scratch - a much larger effort, and
every one of the 121 `stage2-resolve` tests would need re-writing, not
just porting. Only worth it if there's an organizational reason to avoid
shipping a Python runtime at all.

## What changes

**Removed entirely**: `webui/app.py`, `webui/browse.py`, all Jinja
templates, `webui/static/`, the Flask/Werkzeug dependency, the
in-memory `JobRegistry` + HTTP-polling pattern (replaced by Qt's own
signal/slot mechanism - no more polling an endpoint every 1.5s, the
worker thread just emits a signal).

**Reused unchanged**: every file under `stage2-resolve/flexapp_vuln/`,
and `stage1-extract/*.ps1` (still invoked as a `pwsh` subprocess exactly
as today).

## New pieces needed

A new `desktop/` module, replacing `webui/`:

- `main.py` - `QApplication` entry point
- `main_window.py` - `QMainWindow`: the dashboard (Recent Scans as a
  `QTableWidget`), New Scan / Compare Scans actions
- `new_scan_dialog.py` - `QDialog` using `QFileDialog` for the package
  path and output folder - this is a native dialog, so UNC paths and
  network drives work automatically, with no equivalent of the
  `browse.py`/"jump to path" code the web version needed
- `scan_worker.py` - a `QThread` wrapping the existing
  `_run_stage1()`/`_run_stage2()`/`resolve_vuln_matches(on_progress=...)`
  calls, emitting Qt signals (`progress_changed`, `log_line`,
  `finished`, `error`) instead of writing into a polled job dict
- `results_view.py` - a `QTableView` bound to a `QAbstractTableModel`
  wrapping `confirmed_rows`/`heuristic_rows` - sortable/filterable for
  free, no custom JS needed
- `compare_view.py` - reuses `jobs.load_diff()`/`diff_finding_rows()`
  unchanged, rendered in two list widgets
- **New**: a persistence layer for "Recent Scans." The web app's
  `JobRegistry` is memory-only and resets every time the process
  restarts - a desktop app needs its scan history to survive being
  closed and reopened. Simplest option: a JSON file under
  `%APPDATA%\FlexAppVulnScanner\recent-scans.json`; SQLite if it grows
  past a few hundred entries.

## Packaging

- `PyInstaller --onefile` (or `--onedir` for faster startup) →
  `FlexAppVulnScanner.exe`.
- `pwsh` (PowerShell 7) must still be present on the target machine for
  Stage 1 - same requirement as today, unchanged.
- An unsigned `.exe` will trigger Windows SmartScreen on first run -
  worth noting in this repo's Sparks Tool distribution documentation
  (same treatment `Spark_License.pdf`/`bom.cdx.json` already get).

## What's gained

- No browser tab, no localhost port, no "is the server still running?"
- Real native file/folder pickers - deletes the ~150 lines of custom
  `browse.py`/`browse.html`/UNC-jump-box code just built for the web
  version, since `QFileDialog` handles UNC paths and network drives
  natively, out of the box.
- Real native progress bar and data grid widgets: sortable, filterable,
  resizable columns, copy/paste, right-click context menus - all free
  from Qt, none of it hand-rolled in JS.
- Can scan in the background while using other windows, with a system
  tray icon and a completion toast - not really available in a browser
  tab.

## What's lost, or needs a new answer

- **No "share a link to a result with a colleague."** The web version's
  URL-per-job/download-id model goes away. A native app would need
  something like "Copy report path to clipboard" or "Email the PDF"
  instead.
- **No remote/headless access.** Running the scanner on one machine and
  checking results from another goes away entirely - native apps are
  single-machine. **This is the one thing worth confirming before
  starting**: if you (or anyone else) ever run this on a server and
  check results from a different machine, a full native-only rewrite is
  the wrong call, and a native front-end that talks to the existing
  Flask API as a backend would fit better than retiring `webui/`.
- The Sparks Tool checklist's `/license` and `/sbom` web routes need a
  replacement - e.g. a "Help → About" dialog and a "Help → View
  License" menu item.

## Proposed migration phases

1. Scaffold the PySide6 app shell (main window, navigation) with static
   mock data - validate the look against the published mockup.
2. Wire the New Scan dialog + `QThread` worker to the real Stage 1/
   Stage 2 pipeline (existing `stage2-resolve` tests untouched).
3. Wire the Results view to real data (a `QAbstractTableModel` over
   `build_finding_rows()`).
4. Wire Recent Scans persistence (JSON/SQLite) and the Compare Scans
   view.
5. Package with PyInstaller; smoke-test the `.exe` on a clean Windows
   VM.
6. Retire `webui/` once the native app is confirmed as the sole
   distribution path (or keep both, if remote access turns out to still
   be wanted - see the open item above).

## Rough effort shape

- Phases 1-2 are mostly rearranging existing, already-tested logic into
  Qt widgets/threads - comparable in size to what `webui/app.py` +
  `webui/jobs.py` already took.
- Phases 3-4 are genuinely new work (Qt table models, a JSON-backed
  recent-scans store) with no existing analog to reuse - moderate
  effort.
- Phase 5 (packaging/signing) is typically the least predictable part of
  a first PyInstaller build (antivirus false positives, missing DLLs) -
  plan for a few iteration rounds against a real Windows machine, the
  same way Stage 1's live validation needed a few rounds.

## Open items needing a decision before any code is written

1. **Remote/headless access** - see above. If this is ever needed,
   say so before Phase 1 starts; it changes the target architecture.
2. **PySide6 is LGPL-licensed** - worth a one-line entry in
   `bom.cdx.json`/`THIRD-PARTY-NOTICES.txt` once this ships, same
   treatment every other dependency already gets in this repo.
3. **Go-ahead to start Phase 1.** Nothing above has been built - this
   document and the linked mockup are the proposal.
