#!/usr/bin/env bash
# Regenerates bom.cdx.json from the current Go module graph and the
# frontend's production npm dependency tree, merged into one CycloneDX
# 1.6 document (see project brief §11.8 stage 4/scripts/release.sh).
#
# Requires cyclonedx-gomod (go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.10.0)
# on PATH; cyclonedx-npm is fetched on demand via npx, pinned to a fixed
# version below -- an unpinned `npx --yes @cyclonedx/cyclonedx-npm`
# resolves whatever's newest on the npm registry at invocation time,
# which drifts the SBOM's contents (a newer cyclonedx-npm started adding
# per-component "engine" constraint properties) independently of any
# real dependency change, making CI's "is the committed SBOM current"
# check fail nondeterministically depending only on when it happens to
# run relative to a cyclonedx-npm release.
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
(cd web/frontend && npx --yes @cyclonedx/cyclonedx-npm@6.0.1 --omit dev --output-file "$npm_sbom" --output-format JSON)

echo "generate-sbom: merging..."
python3 "$repo_root/scripts/merge-sbom.py" "$go_sbom" "$npm_sbom" "$version" "$repo_root/bom.cdx.json"
