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
