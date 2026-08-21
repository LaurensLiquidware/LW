# FlexApp Vulnerability Scanner — Desktop App

A native Windows desktop app (PySide6/Qt) for the FlexApp Vulnerability
Scanner - the same Stage 1 (PowerShell inventory) + Stage 2 (OSV.dev/NVD
matching) pipeline as `../webui/`, with a real window instead of a
browser tab and local Flask server. See `../NATIVE_APP_MIGRATION.md` for
why this exists and what changed.

## Running it

```powershell
pip install -r requirements.txt -r ..\requirements.txt
python main.py
```

Requires `pwsh` (PowerShell 7) on `PATH`, same as the web UI - Stage 1
still shells out to it, unchanged.

## What's here

- `main.py` - entry point (`QApplication` + `MainWindow`)
- `main_window.py` - the dashboard: New Scan / Compare Scans actions, a
  Recent Scans table that persists across restarts
- `new_scan_dialog.py` - package path + output folder (auto-derived from
  the package name, editable) + a collapsible Advanced section for the
  optional NVD API key. Uses the OS's real file/folder picker
  (`QFileDialog`) - this is why there's no equivalent of the web UI's
  `browse.py`: UNC paths and network drives just work.
- `scan_worker.py` - runs a scan (or a refresh) on a background
  `QThread`, adapting Qt signals to `flexapp_vuln.pipeline`'s duck-typed
  progress-sink interface. No polling endpoint needed - the UI just
  connects to signals.
- `scan_progress_window.py` - live progress: an indeterminate bar during
  Stage 1 and the start of Stage 2, a real determinate one once the
  OSV/NVD phase starts (mirrors the web UI's progress bar), plus a log.
- `results_window.py` - coverage stat, severity-count cards, and a
  sortable/filterable findings table (`QTableView` + `QSortFilterProxyModel`,
  both stock Qt). Export buttons open the already-written PDF/SBOM/CSV
  with the OS's default app - no download route needed, unlike the web
  UI, since this process can already touch the local filesystem.
- `compare_dialog.py` - the desktop equivalent of the web UI's Compare
  Scans page, over the same `flexapp_vuln.pipeline.load_diff()`.
- `models.py` - `FindingsTableModel`, a `QAbstractTableModel` over
  `build_finding_rows()`'s output. The Affected Files column shows the
  single path inline, or "N files" with the full list on hover
  (tooltip) and via double-click (a details popup) when a finding spans
  more than one file.
- `recent_scans_store.py` - persists the dashboard's scan list as JSON
  under `%APPDATA%\FlexAppVulnScanner\recent-scans.json`, so it survives
  the app being closed and reopened (the web UI's job list is
  memory-only and resets on restart - fine for a page you reload, wrong
  for a desktop app).

All the actual scanning logic - Stage 1/2 orchestration, report writing,
diffing - lives in `../stage2-resolve/flexapp_vuln/pipeline.py`, shared
with the web UI. Nothing in this directory duplicates that logic.

## Packaging

```powershell
pip install pyinstaller
pyinstaller flexapp_scanner.spec
```

Produces `dist\FlexAppVulnScanner\FlexAppVulnScanner.exe`. Must be built
on a real Windows machine - PyInstaller builds for the OS it runs on, it
does not cross-compile. The target machine still needs `pwsh` on `PATH`
for Stage 1; PyInstaller only bundles the Python side.

**Not yet validated against a real Windows build** - this repo's
dev/test environment is Linux. The spec has been built and smoke-run
successfully as a Linux binary in that environment (confirming the
import graph and bundled data files resolve correctly, including a real
`FileNotFoundError` caught and fixed this way - `jsonschema`'s optional
`rfc3987_syntax` dependency loads a `.lark` grammar file from disk at
import time, which PyInstaller's default analysis doesn't catch), but a
Windows `.exe` build has not been produced or run. Treat the `.spec` as
a validated starting point, not a guarantee of a clean first Windows
build - the same posture this project already takes with Stage 1's
PowerShell scripts before their first real-Windows run (see PLAN.md).

## Testing

```bash
pip install pytest-qt
QT_QPA_PLATFORM=offscreen pytest tests/
```

`QT_QPA_PLATFORM=offscreen` runs the real Qt widgets without a display -
needed in CI/headless environments (and this dev sandbox); omit it when
running interactively on a machine with a display.
