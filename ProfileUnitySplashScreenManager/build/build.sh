#!/usr/bin/env bash
# Builds the Windows executable and the distributable zip.
#
# All of the logic lives in cmd/build so there is one implementation rather than
# a bash copy and a PowerShell copy drifting apart. This wrapper only checks the
# toolchain is reachable and forwards its arguments.
#
# Cross-compiles cleanly from Linux or macOS: the application is CGO-free, so no
# Windows toolchain is needed to produce the .exe.
#
#   ./build/build.sh                    # full build
#   ./build/build.sh -skip-ui           # reuse the existing Angular output
#   ./build/build.sh -goos linux        # a development binary for this machine
#
# Set PRIMEUI_LICENSE_KEY to embed the PrimeNG commercial license key. Without it
# the build succeeds but the running application shows PrimeNG's
# "Invalid PrimeUI License" banner. See README.md, "PrimeNG licensing".
set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is not on PATH" >&2
  exit 1
fi

exec go run ./cmd/build "$@"
