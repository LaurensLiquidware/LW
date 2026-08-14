#!/usr/bin/env bash
# Cross-compiles the Windows amd64 build and embeds the Liquidware brand
# icon (web/frontend/public/favicon.ico -- the same icon already used for
# the app's browser tab, so the .exe and the web UI show the same mark)
# plus version/company metadata into the binary via a Windows resource
# (.syso), so Explorer/Properties show it as a genuine Liquidware product
# instead of a bare Go binary icon.
#
# Requires goversioninfo on PATH:
#   go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
#
# Usage: ./scripts/build-windows.sh [output-path]
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if ! command -v goversioninfo >/dev/null 2>&1; then
  echo "build-windows: goversioninfo not found on PATH -- go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest" >&2
  exit 1
fi

out_path="${1:-$repo_root/profileunity-msp-console.exe}"
version="$(cat VERSION)"
IFS='.' read -r major minor patch <<<"$version"

versioninfo_json="$(mktemp --suffix=.json)"
syso_path="cmd/server/resource_windows_amd64.syso"
trap 'rm -f "$versioninfo_json" "$syso_path"' EXIT

cat >"$versioninfo_json" <<EOF
{
  "FixedFileInfo": {
    "FileVersion": {"Major": ${major:-0}, "Minor": ${minor:-0}, "Patch": ${patch:-0}, "Build": 0},
    "ProductVersion": {"Major": ${major:-0}, "Minor": ${minor:-0}, "Patch": ${patch:-0}, "Build": 0},
    "FileFlagsMask": "3f",
    "FileFlags": "00",
    "FileOS": "040004",
    "FileType": "01",
    "FileSubType": "00"
  },
  "StringFileInfo": {
    "CompanyName": "Liquidware",
    "FileDescription": "ProfileUnity MSP Licensing Console",
    "FileVersion": "${version}",
    "InternalName": "profileunity-msp-console",
    "LegalCopyright": "(c) Liquidware. See Spark_License.pdf.",
    "OriginalFilename": "profileunity-msp-console.exe",
    "ProductName": "ProfileUnity MSP Licensing Console",
    "ProductVersion": "${version}"
  },
  "VarFileInfo": {
    "Translation": {"LangID": "0409", "CharsetID": "04B0"}
  },
  "IconPath": "web/frontend/public/favicon.ico"
}
EOF

echo "build-windows: generating Windows resource (icon + version info)..."
goversioninfo -o "$syso_path" "$versioninfo_json"

echo "build-windows: cross-compiling windows/amd64..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$out_path" ./cmd/server
echo "build-windows: built $out_path"
