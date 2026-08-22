#!/usr/bin/env bash
# Cross-compiles both Windows amd64 binaries and embeds the Liquidware
# brand icon (web/frontend/public/favicon.ico -- the same icon already
# used for the app's browser tab, so both .exe files and the web UI show
# the same mark) plus version/company metadata into each via a Windows
# resource (.syso), so Explorer/Properties show genuine Liquidware
# products instead of bare Go binary icons.
#
# Two binaries, two different jobs:
#   - flexapp-vuln-scanner.exe: the tray launcher (cmd/tray) --
#     what an operator double-clicks in Explorer. Windows-GUI subsystem
#     (no console window), starts/stops/restarts the server below and
#     shows a live log viewer.
#   - flexapp-vuln-scanner-server.exe: the actual headless server
#     (cmd/server, unchanged) -- what the launcher spawns, and what
#     anyone running this from PowerShell, a Scheduled Task, or a
#     Windows Service should point at directly instead.
#
# Requires goversioninfo on PATH:
#   go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
#
# Usage: ./scripts/build-windows.sh [output-dir]
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if ! command -v goversioninfo >/dev/null 2>&1; then
  echo "build-windows: goversioninfo not found on PATH -- go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest" >&2
  exit 1
fi

out_dir="${1:-$repo_root}"
mkdir -p "$out_dir"
version="$(cat VERSION)"
IFS='.' read -r major minor patch <<<"$version"

# generate_syso <package_dir> <internal_name> <original_filename> <file_description> [manifest_path]
# Writes a versioninfo.json describing one binary and runs goversioninfo
# to produce a .syso in that package's directory, which `go build`
# picks up automatically for GOOS=windows GOARCH=amd64 -- same mechanism
# used for the single binary before this script built two. The optional
# manifest embeds comctl32 v6 + per-monitor DPI awareness (only needed
# for the tray launcher's actual UI, not the headless server).
generate_syso() {
  local pkg_dir="$1" internal_name="$2" original_filename="$3" file_description="$4" manifest_path="${5:-}"
  local json_path syso_path manifest_json="null"
  json_path="$(mktemp --suffix=.json)"
  syso_path="$pkg_dir/resource_windows_amd64.syso"
  cleanup_paths+=("$json_path" "$syso_path")

  if [[ -n "$manifest_path" ]]; then
    manifest_json="\"$manifest_path\""
  fi

  cat >"$json_path" <<EOF
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
    "FileDescription": "${file_description}",
    "FileVersion": "${version}",
    "InternalName": "${internal_name}",
    "LegalCopyright": "(c) Liquidware. See Spark_License.pdf.",
    "OriginalFilename": "${original_filename}",
    "ProductName": "FlexApp Vulnerability Scanner",
    "ProductVersion": "${version}"
  },
  "VarFileInfo": {
    "Translation": {"LangID": "0409", "CharsetID": "04B0"}
  },
  "IconPath": "web/frontend/public/favicon.ico",
  "ManifestPath": ${manifest_json}
}
EOF

  goversioninfo -o "$syso_path" "$json_path"
}

cleanup_paths=()
trap 'rm -f "${cleanup_paths[@]}"' EXIT

echo "build-windows: generating Windows resources (icon + version info)..."
generate_syso cmd/tray flexapp-vuln-scanner flexapp-vuln-scanner.exe "FlexApp Vulnerability Scanner" cmd/tray/app.manifest
generate_syso cmd/server flexapp-vuln-scanner-server flexapp-vuln-scanner-server.exe "FlexApp Vulnerability Scanner (Server)" cmd/server/app.manifest
generate_syso cmd/cli flexapp-vuln-scanner-cli flexapp-vuln-scanner-cli.exe "FlexApp Vulnerability Scanner (CLI)"

echo "build-windows: cross-compiling windows/amd64..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-H=windowsgui" -o "$out_dir/flexapp-vuln-scanner.exe" ./cmd/tray
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$out_dir/flexapp-vuln-scanner-server.exe" ./cmd/server
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$out_dir/flexapp-vuln-scanner-cli.exe" ./cmd/cli
echo "build-windows: built $out_dir/flexapp-vuln-scanner.exe, $out_dir/flexapp-vuln-scanner-server.exe, and $out_dir/flexapp-vuln-scanner-cli.exe"
