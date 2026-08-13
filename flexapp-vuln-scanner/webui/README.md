# Web UI — local pipeline runner + report browser

A local Flask app that runs the full Stage 1 → Stage 2 pipeline from a
browser instead of two manual commands, and lets you browse any past scan's
coverage/findings/SBOM/PDF output without re-running anything.

## Install

```bash
pip install -r ../requirements.txt -r requirements.txt
```

## Run

```bash
python app.py
```

Then open <http://127.0.0.1:5000>. Two things you can do from the
dashboard:

- **Run a new scan** — give it a package path (`.vhdx`/`.exe`/`.flexapp`)
  and an output directory. This shells out to `pwsh` to run Stage 1
  (`Invoke-FlexAppInventory.ps1`), then calls Stage 2's `flexapp_vuln`
  functions in-process (same code the CLI uses — results can't drift from
  what `resolve`/`report --pdf` would produce) to query OSV.dev/NVD and
  write `sbom.cdx.json`/`coverage-report.md`/`findings.md`/`report.pdf`/
  `vuln-matches.json`. Progress streams live via polling.
- **Open an existing scan output folder** — point it at any directory
  already containing a `*.inventory.json` (e.g. one you scanned from the
  command line) and it renders the same coverage/findings view, reusing a
  sibling `vuln-matches.json` if present, with no network calls.

Both path fields have a **Browse…** link next to them — a server-side
filesystem browser (drives → folders → files) so you don't have to type or
paste paths by hand. It defaults to this repo's own directory and lets you
navigate anywhere the app can already read; for the package-path field it
only lists `.vhdx`/`.exe`/`.flexapp` files, for the output-directory fields
it's folder-only with a "Select this folder" button.

## Requirements for "Run a new scan"

- `pwsh` (PowerShell 7) on `PATH` — Stage 1 is PowerShell, invoked as a
  subprocess.
- This process likely needs to run **elevated** (as Administrator) on
  Windows — mounting a VHDX typically requires it, same as running
  `Invoke-FlexAppInventory.ps1` directly.
- Network access to `api.osv.dev`/`services.nvd.nist.gov` for the
  vulnerability-matching step (same as the `resolve` CLI command).

None of this is needed for "Open an existing scan output folder" — that
path only reads files already on disk.

## Security

This is a **local, single-user tool**, not a multi-user web service:

- It binds to `127.0.0.1` only (see `app.py`'s `if __name__ == "__main__"`
  block) — do not change this to `0.0.0.0` or put it behind a reverse
  proxy reachable by anyone but you. The process can run arbitrary local
  PowerShell (whatever path you type into "Run a new scan") and read
  arbitrary files (whatever directory you type into "Open an existing
  scan output folder") — the same trust level as you having a terminal
  open on this machine, nothing more, nothing less.
- The **Browse…** filesystem picker lists directory contents on request
  (any path the app already has read access to) - it doesn't grant any
  access this process didn't already have, and is subject to the same
  "local, single-user, 127.0.0.1-only" trust boundary as everything else
  here.
- Download links never take a raw filesystem path from the browser. Every
  scan (run fresh or opened from disk) gets a random in-memory id, and
  `/download/job/<id>/<kind>` / `/download/open/<id>/<kind>` only serve
  the exact file paths this process already computed for that id — not an
  arbitrary-path-read endpoint.
- The in-memory job/opened-scan registries are per-process and
  unauthenticated by design (no login, no persistence) — restarting the
  app clears "Scans run this session"; past output files on disk are
  untouched and can always be re-opened.

## Tests

```bash
python -m pytest
```

Mocks/avoids anything that needs real infrastructure (VHDX mounting,
network calls to OSV/NVD) — `test_jobs.py` exercises the Stage 1
error-handling path (missing `pwsh`/script) and the full Stage 2 + PDF
pipeline via `load_existing_result` (real fixture data, no network);
`test_app.py` exercises the Flask routes with Flask's test client.
