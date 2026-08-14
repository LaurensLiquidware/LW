#!/usr/bin/env bash
# Regenerates bom.cdx.json from the current Go module graph and the
# frontend's production npm dependency tree, merged into one CycloneDX
# 1.6 document (see project brief §11.8 stage 4/scripts/release.sh).
#
# Requires cyclonedx-gomod (go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.10.0)
# on PATH; cyclonedx-npm is fetched on demand via npx.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if ! command -v cyclonedx-gomod >/dev/null 2>&1; then
  echo "generate-sbom: cyclonedx-gomod not found on PATH -- go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.10.0" >&2
  exit 1
fi

version="$(cat VERSION)"
go_sbom="$(mktemp)"
npm_sbom="$(mktemp)"
trap 'rm -f "$go_sbom" "$npm_sbom"' EXIT

echo "generate-sbom: scanning Go module graph..."
cyclonedx-gomod app -json -output "$go_sbom" -licenses -main cmd/server .

echo "generate-sbom: scanning frontend production npm dependencies..."
(cd web/frontend && npx --yes @cyclonedx/cyclonedx-npm --omit dev --output-file "$npm_sbom" --output-format JSON)

echo "generate-sbom: merging..."
python3 "$repo_root/scripts/merge-sbom.py" "$go_sbom" "$npm_sbom" "$version" "$repo_root/bom.cdx.json"
