#!/usr/bin/env bash
# Release pipeline skeleton. See project brief §11.8: items 4 (SBOM), 5
# (Grype CVE gate), 6 (version), and 7 (license/SBOM packaging) are
# coupled and MUST run in that order — clearing a CVE changes the
# component set, which changes the SBOM, which changes the version, and
# only then is the SBOM ready to package next to the license PDF.
#
# This script is a skeleton: each stage below is a placeholder until the
# corresponding build phase lands (SBOM tooling, Grype, the Angular
# frontend build). Running it today does the version sync and the Go
# build/test only, and stops with a clear message for everything else.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

echo "== 1/8: sync version =="
./scripts/sync-version.sh

echo "== 2/8: go vet + test =="
go vet ./...
go test ./...

echo "== 3/8: regenerate SBOM (Syft, Go + npm merged) =="
echo "NOT IMPLEMENTED YET — no npm tree exists before the Phase 4 frontend." >&2

echo "== 4/8: Grype scan of the SBOM, hard gate on Critical/High =="
echo "NOT IMPLEMENTED YET — depends on stage 3." >&2

echo "== 5/8: generate THIRD-PARTY-NOTICES.txt from the SBOM =="
echo "NOT IMPLEMENTED YET — depends on stage 3." >&2

echo "== 6/8: build frontend =="
echo "NOT IMPLEMENTED YET — Angular app does not exist before Phase 4." >&2

echo "== 7/8: build backend (embeds VERSION, SBOM, notices) =="
go build -o "$repo_root/profileunity-msp-console" ./cmd/server
echo "Built ./profileunity-msp-console (this is a dev build; the release build additionally embeds the post-remediation SBOM and notices per stage 3-5)."

echo "== 8/8: produce one release zip (binary + Spark_License.pdf + bom.cdx.json + THIRD-PARTY-NOTICES.txt + README.md + CHANGELOG.md) =="
echo "NOT IMPLEMENTED YET — held until stages 3-6 are real." >&2

exit 1
