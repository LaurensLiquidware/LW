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
#
# Local tool prerequisites beyond Go/Node: cyclonedx-gomod, cyclonedx-npm
# (both `go install`/`npm install -g`, see scripts/generate-sbom.sh's
# header), grype (go install github.com/anchore/grype/cmd/grype@latest,
# see scripts/check-vulnerabilities.sh), and goversioninfo for
# scripts/build-windows.sh (go install
# github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

echo "== 1/9: sync version and legal files =="
./scripts/sync-version.sh
./scripts/sync-legal.sh

echo "== 2/9: build frontend =="
(cd web/frontend && npm ci && npm run build)

echo "== 3/9: go vet + test =="
# Explicit packages, not ./... — web/frontend/node_modules ships at least
# one vendored .go file with no go.mod to bound it out of our module.
go vet ./cmd/... ./internal/... ./web
go test ./cmd/... ./internal/... ./web

echo "== 4/9: regenerate SBOM (Go + npm merged) =="
./scripts/generate-sbom.sh

echo "== 5/9: CVE gate (grype against the SBOM) =="
./scripts/check-vulnerabilities.sh

echo "== 6/9: generate THIRD-PARTY-NOTICES.txt from the SBOM =="
python3 ./scripts/generate-notices.py bom.cdx.json THIRD-PARTY-NOTICES.txt

echo "== 7/9: build backend + CLI (embeds VERSION, frontend, SBOM, notices) =="
# Stage 4/6 rewrote bom.cdx.json and THIRD-PARTY-NOTICES.txt at the repo
# root -- re-sync so internal/legal embeds the versions just generated,
# not whatever was there before this script ran.
./scripts/sync-legal.sh
go build -o "$repo_root/flexapp-vuln-scanner" ./cmd/server
go build -o "$repo_root/flexapp-vuln-scanner-cli" ./cmd/cli
echo "release: built ./flexapp-vuln-scanner and ./flexapp-vuln-scanner-cli"

echo "== 8/9: cross-compile Windows build (tray launcher + server + CLI, Liquidware icon + version info) =="
if ! command -v goversioninfo >/dev/null 2>&1; then
  echo "release: goversioninfo not found on PATH -- go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest" >&2
  exit 1
fi
./scripts/build-windows.sh "$repo_root"

echo "== 9/9: produce release zips (Linux + Windows) =="
# Every release bundles: the binary/binaries (Windows gets three -- the
# tray launcher an operator double-clicks, the headless server it spawns,
# and the CLI for scripted/CI/cron use; Linux gets the server and CLI
# binaries, since the tray launcher is Windows-only), stage1-extract/ and
# config/ (the PowerShell Stage 1 script + its supporting modules, and
# cpe-mappings.yaml -- both are read from disk next to the running binary
# at their config.go default paths, so a release without them is unable
# to run a scan at all, even though the server itself starts up fine), a
# starting .env.example (see internal/dotenv -- the server reads .env
# from its own working directory, so operators need this to get running
# without hand-exporting environment variables), the version history,
# and everything a compliance reviewer needs to sign off on third-party
# content -- the SBOM, the per-license breakdown, and the Sparks Tool
# license/disclaimer itself. README.md stays out on purpose: it's written
# for someone building this from source, not running it. One zip per
# platform, since the binaries differ but everything else in the bundle
# is shared.
version="$(cat VERSION)"
zip_dir="$(mktemp -d)"
trap 'rm -rf "$zip_dir"' EXIT

stage_common() {
  local stage="$1"
  cp -r stage1-extract config "$stage/"
  cp .env.example Spark_License.pdf bom.cdx.json THIRD-PARTY-NOTICES.txt CHANGELOG.md "$stage/"
}

linux_zip="flexapp-vuln-scanner-${version}-linux-amd64.zip"
linux_stage="$zip_dir/flexapp-vuln-scanner-${version}-linux-amd64"
mkdir -p "$linux_stage"
cp flexapp-vuln-scanner flexapp-vuln-scanner-cli "$linux_stage/"
stage_common "$linux_stage"
(cd "$zip_dir" && zip -qr "$repo_root/$linux_zip" "flexapp-vuln-scanner-${version}-linux-amd64")
echo "release: wrote $linux_zip"

windows_zip="flexapp-vuln-scanner-${version}-windows-amd64.zip"
windows_stage="$zip_dir/flexapp-vuln-scanner-${version}-windows-amd64"
mkdir -p "$windows_stage"
cp flexapp-vuln-scanner.exe flexapp-vuln-scanner-server.exe flexapp-vuln-scanner-cli.exe "$windows_stage/"
stage_common "$windows_stage"
(cd "$zip_dir" && zip -qr "$repo_root/$windows_zip" "flexapp-vuln-scanner-${version}-windows-amd64")
echo "release: wrote $windows_zip"
