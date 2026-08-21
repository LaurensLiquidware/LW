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
