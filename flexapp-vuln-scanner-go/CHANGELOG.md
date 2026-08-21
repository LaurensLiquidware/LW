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
