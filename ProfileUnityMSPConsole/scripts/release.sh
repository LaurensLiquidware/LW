#!/usr/bin/env bash
# Release pipeline. See project brief §11.8: items 4 (SBOM), 5 (Grype CVE
# gate), 6 (version), and 7 (license/SBOM packaging) are coupled and MUST
# run in that order -- clearing a CVE changes the component set, which
# changes the SBOM, which changes the version, and only then is the SBOM
# ready to package next to the license PDF.
#
# The frontend must build before any Go step that touches the httpapi/web
# packages, since they go:embed its output (web/dist) -- there is no
# fallback content once the Phase 1 placeholder is gone.
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

echo "== 4/8: regenerate SBOM (Go + npm merged) =="
./scripts/generate-sbom.sh

echo "== 5/8: CVE gate (govulncheck) =="
if ! command -v govulncheck >/dev/null 2>&1; then
  echo "release: govulncheck not found on PATH -- go install golang.org/x/vuln/cmd/govulncheck@v1.1.4" >&2
  exit 1
fi
govulncheck_output="$(mktemp)"
trap 'rm -f "$govulncheck_output"' EXIT
if govulncheck ./cmd/... ./internal/... ./web >"$govulncheck_output" 2>&1; then
  echo "release: govulncheck found no known vulnerabilities."
elif grep -q "fetching vulnerabilities" "$govulncheck_output"; then
  echo "release: WARNING -- govulncheck could not reach its vulnerability database (network policy)." >&2
  echo "release: this is an environment limitation, not a clean bill of health -- re-run this script" >&2
  echo "release: from a network-unrestricted environment before treating this build as a real release." >&2
  cat "$govulncheck_output" >&2
else
  echo "release: govulncheck found a real, known vulnerability -- fix it before releasing." >&2
  cat "$govulncheck_output" >&2
  exit 1
fi

echo "== 6/8: generate THIRD-PARTY-NOTICES.txt from the SBOM =="
python3 ./scripts/generate-notices.py bom.cdx.json THIRD-PARTY-NOTICES.txt

echo "== 7/8: build backend (embeds VERSION, frontend, SBOM, notices) =="
# Stage 4/6 rewrote bom.cdx.json and THIRD-PARTY-NOTICES.txt at the repo
# root -- re-sync so internal/legal embeds the versions just generated,
# not whatever was there before this script ran.
./scripts/sync-legal.sh
go build -o "$repo_root/profileunity-msp-console" ./cmd/server
echo "release: built ./profileunity-msp-console"

echo "== 8/8: produce one release zip =="
version="$(cat VERSION)"
zip_name="profileunity-msp-console-${version}.zip"
zip_dir="$(mktemp -d)"
trap 'rm -f "$govulncheck_output"; rm -rf "$zip_dir"' EXIT
stage_dir="$zip_dir/profileunity-msp-console-${version}"
mkdir -p "$stage_dir"
cp profileunity-msp-console Spark_License.pdf bom.cdx.json THIRD-PARTY-NOTICES.txt README.md CHANGELOG.md "$stage_dir/"
(cd "$zip_dir" && zip -qr "$repo_root/$zip_name" "profileunity-msp-console-${version}")
echo "release: wrote $zip_name"
