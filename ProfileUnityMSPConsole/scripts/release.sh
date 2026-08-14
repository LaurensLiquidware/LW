#!/usr/bin/env bash
# Release pipeline skeleton. See project brief §11.8: items 4 (SBOM), 5
# (Grype CVE gate), 6 (version), and 7 (license/SBOM packaging) are
# coupled and MUST run in that order — clearing a CVE changes the
# component set, which changes the SBOM, which changes the version, and
# only then is the SBOM ready to package next to the license PDF.
#
# The frontend must build before any Go step that touches the httpapi/web
# packages, since they go:embed its output (web/dist) — there is no
# fallback content once the Phase 1 placeholder is gone.
#
# This script is a skeleton for the stages that don't exist yet (SBOM
# tooling, Grype). Running it today does the real version sync, frontend
# build, and Go build/test, and stops with a clear message for the rest.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

echo "== 1/8: sync version and legal files =="
./scripts/sync-version.sh
./scripts/sync-legal.sh

echo "== 2/8: build frontend =="
(cd web/frontend && npm ci && npm run build)

echo "== 3/8: go vet + test =="
# Explicit packages, not ./... — web/frontend/node_modules ships at least
# one vendored .go file with no go.mod to bound it out of our module.
go vet ./cmd/... ./internal/... ./web
go test ./cmd/... ./internal/... ./web

echo "== 4/8: regenerate SBOM (Syft, Go + npm merged) =="
echo "NOT IMPLEMENTED YET — held for the Phase 8 compliance pass." >&2

echo "== 5/8: Grype scan of the SBOM, hard gate on Critical/High =="
echo "NOT IMPLEMENTED YET — depends on stage 4." >&2

echo "== 6/8: generate THIRD-PARTY-NOTICES.txt from the SBOM =="
echo "NOT IMPLEMENTED YET — depends on stage 4." >&2

echo "== 7/8: build backend (embeds VERSION, frontend, SBOM, notices) =="
go build -o "$repo_root/profileunity-msp-console" ./cmd/server
echo "Built ./profileunity-msp-console (this is a dev build; the release build additionally embeds the post-remediation SBOM and notices per stages 4-6)."

echo "== 8/8: produce one release zip (binary + Spark_License.pdf + bom.cdx.json + THIRD-PARTY-NOTICES.txt + README.md + CHANGELOG.md) =="
echo "NOT IMPLEMENTED YET — held until stages 4-6 are real." >&2

exit 1
