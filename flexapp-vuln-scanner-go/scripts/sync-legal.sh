#!/usr/bin/env bash
# Copies the root-level Sparks Tool License, SBOM, and third-party
# notices into internal/legal so go:embed can pick them up (go:embed
# cannot reach outside its own package directory). Run this before every
# build — see internal/legal/legal.go.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dst="$repo_root/internal/legal"

for name in Spark_License.pdf bom.cdx.json THIRD-PARTY-NOTICES.txt; do
  src="$repo_root/$name"
  if [[ ! -f "$src" ]]; then
    echo "sync-legal: $src not found" >&2
    exit 1
  fi
  cp "$src" "$dst/$name"
done

echo "sync-legal: copied Spark_License.pdf, bom.cdx.json, THIRD-PARTY-NOTICES.txt into internal/legal/"
