# Native Windows App Migration

**Status: Phases 1-4 built** (see `desktop/`). Phase 5 (packaging) has a
working `.spec` file, smoke-tested as a Linux binary in this repo's
dev/test environment, but **not yet built or run as a real Windows
`.exe`** - see `desktop/README.md`'s Packaging section for exactly what
has and hasn't been confirmed. Phase 6 (retiring `webui/`) has not
happened - both front ends are live and share the same
`flexapp_vuln.pipeline` scanning logic. Original mockup:
https://claude.ai/code/artifact/61d0073c-6a0a-4133-bf11-a80088e633f7

## Why

The web UI is a local Flask app: it starts a server on
`127.0.0.1:5000`, and you drive it from a browser tab. That works, but
it's not what "a Windows app" means to most users - there's a server
process to keep alive, a port, a browser tab that looks like every other
web page, and a custom-built file/folder browser (`webui/browse.py`)
because a web page can't call the OS's real file picker.

## Approach taken: PySide6 (Qt for Python)

**Rationale**: this keeps 100% of `stage2-resolve/flexapp_vuln/` -
`resolve_vuln_matches`, `reporting.py`, `sbom.py`, `pdf_report.py`,
`cli.py`, `nvd_client.py`, `osv_client.py`, `coverage.py`,
`cpe_mappings.py` - completely untouched, along with its tests. Stage 1
keeps shelling out to `pwsh` exactly as it does today. Only the
presentation layer changes: Flask/HTML/CSS is replaced by a real native
window, with no browser and no server process.

The web UI's own scan orchestration (`webui/jobs.py`) was extracted into
`stage2-resolve/flexapp_vuln/pipeline.py` as part of this work, front-end
agnostic - both `webui/jobs.py` and `desktop/scan_worker.py` now call the
exact same `run_stage1`/`run_stage2`/`write_reports`/`load_existing_result`/
`load_diff` functions, adapted via a duck-typed progress-sink interface
(`append_log`/`status`/`set_progress`) that `ScanJob` (web) and
`ScanWorker` (desktop, backed by Qt signals) both satisfy. Neither front
end's scanning behavior can drift from the other's.

**Alternative considered and not taken**: a full WPF/.NET (C#) rewrite.
Would have gotten the most "native Windows" look (WinUI/Fluent styling,
MSIX packaging, no Python runtime to bundle) but meant porting every
line of Stage 2's OSV/NVD matching, SBOM building, and PDF/report
generation from Python to C# from scratch, and rewriting every existing
test rather than reusing them. Only worth reconsidering if there's an
organizational reason to avoid shipping a Python runtime at all.

## What changed

**Not removed** (contrary to the original draft plan): `webui/` is
still here and still works - nothing has been decided yet about
retiring it (Phase 6, below). `webui/jobs.py` was refactored to delegate
to the new shared `pipeline.py` rather than removed.

**New**: `desktop/` - see `desktop/README.md` for the full file-by-file
breakdown. In short: `main.py` (entry point), `main_window.py`
(dashboard), `new_scan_dialog.py`, `scan_worker.py` (`QThread` +
`pipeline` calls), `results_window.py` (`QTableView` +
`QSortFilterProxyModel`, both stock Qt - sortable/filterable for free),
`compare_dialog.py`, `models.py` (`FindingsTableModel`), and
`recent_scans_store.py` (JSON persistence, since a desktop app's scan
history needs to survive being closed and reopened - the web UI's
`JobRegistry` is memory-only and resets on restart, which is fine for a
page you reload).

## What's gained (confirmed, not just anticipated)

- No browser tab, no localhost port, no "is the server still running?"
- Real native file/folder pickers (`QFileDialog`) - this deleted the
  entire need for the web UI's `browse.py`/UNC-jump-box code. Verified:
  a `QFileDialog.getOpenFileName`/`getExistingDirectory` call resolves
  UNC and network paths on Windows with zero custom code, unlike the web
  UI which needed a dedicated "jump to path" feature for the same thing.
- Real native sortable/filterable data grid (`QTableView` +
  `QSortFilterProxyModel`), with zero hand-rolled JS - though this
  needed one real fix: the proxy's default sort role is `Qt.DisplayRole`
  (alphabetical text), which sorts "HIGH" before "CRITICAL" - the
  severity column needed `Qt.UserRole` wired as the sort role, carrying
  the model's own severity-rank key. Caught via a screenshot during
  manual verification, not by the unit tests (which tested the model's
  rank data in isolation, not the proxy's actual sort behavior) - fixed
  and a regression test added.
- Export buttons just open the already-written PDF/SBOM/CSV with the
  OS's default app (`QDesktopServices.openUrl`) - no download route
  needed, since this process can already touch the local filesystem.

## What's lost, or still needs an answer

- **No "share a link to a result with a colleague."** The web version's
  URL-per-job/download-id model is gone. Not yet addressed - a "Copy
  report path to clipboard" affordance would be the simplest fix if this
  turns out to matter.
- **No remote/headless access** - confirmed not needed (asked directly:
  scans and results-viewing always happen on the same machine), so this
  is settled, not open.
- The Sparks Tool checklist's `/license` and `/sbom` web routes still
  need a desktop equivalent (e.g. a "Help → About" dialog) - not yet
  built.
- **Cancelling a running scan** isn't real yet - the Scan Progress
  window's Close button closes the window, not the underlying
  `QThread`/`pwsh` subprocess. `QThread.terminate()` is unsafe to call
  on a thread that's blocked in a subprocess wait, so this needs
  deliberate design (e.g. killing the `pwsh` child process handle), not
  a quick fix - flagged here rather than silently left half-working.

## Testing

43 tests in `desktop/tests/` (`recent_scans_store`, `scan_worker` via
`pytest-qt`, `models`, `new_scan_dialog`, `results_window`,
`compare_dialog`, `main_window`), run with `QT_QPA_PLATFORM=offscreen`.
Plus manual verification: every screen was actually launched (as real
Qt widgets, `QT_QPA_PLATFORM=offscreen`, screenshotted via
`widget.grab()`) and driven with real data - the dashboard with
done/running/error entries, the New Scan dialog's auto-fill and Advanced
toggle, the Results window with a 3-file shared CVE, and the Compare
dialog with a real old/new diff. This caught three real bugs before
they'd have been found any other way:

1. `PureWindowsPath` vs. plain `Path` for deriving the output folder
   from a package name - plain `Path` doesn't parse backslash as a
   separator on a non-Windows host, silently breaking the auto-fill
   under any Linux/CI test run (though not on the real Windows target).
2. The severity-sort proxy role, above.
3. The PyInstaller packaging bug, below.

## Packaging

`desktop/flexapp_scanner.spec` builds successfully and the resulting
binary launches and runs cleanly - but only verified as a **Linux**
binary in this dev/test environment (PyInstaller builds for the OS it
runs on; it does not cross-compile), so this is not a validated Windows
build. Building it here caught a real bug worth flagging: `jsonschema`'s
optional `rfc3987_syntax` format-checker dependency loads a `.lark`
grammar file from disk at import time, which PyInstaller's default
import-graph analysis doesn't catch (it follows Python imports, not
arbitrary file reads) - the frozen exe crashed on startup with a
`FileNotFoundError` until the spec was fixed to bundle that file via
`collect_data_files("rfc3987_syntax")`. A real Windows build may well
surface its own, different packaging issues (DLL bundling, antivirus
false positives, Qt platform plugin discovery) that this Linux build
can't catch - see `desktop/README.md`.

## Rough effort shape (in hindsight)

Phases 1-4 (scaffold, wire to the real pipeline, results view, compare +
persistence) took roughly comparable effort to what the web UI's
`app.py` + `jobs.py` originally took, plus the `pipeline.py` extraction
refactor (front-loaded, since both front ends needed to share it from
the start rather than duplicating scan orchestration). Phase 5
(packaging) surfaced one real, non-obvious bug even in a same-OS Linux
build; a first real Windows build should be expected to surface at
least one more thing this environment couldn't catch.

## Remaining open items

1. **Phase 6 (retire `webui/`) has not been decided.** Both front ends
   are live. Nothing here forces a choice - keep both, or retire the web
   UI once the desktop app has had a real Windows validation pass.
2. **PySide6 is LGPL-licensed** - still needs a one-line entry in
   `bom.cdx.json`/`THIRD-PARTY-NOTICES.txt`, same treatment every other
   dependency already gets in this repo. Not yet done.
3. **A real Windows `.exe` build and smoke test** - the one thing this
   Linux dev/test environment genuinely cannot do. Needs a real Windows
   machine, the same way Stage 1's PowerShell scripts needed one before
   their live validation.
