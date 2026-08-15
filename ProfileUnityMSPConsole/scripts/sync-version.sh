#!/usr/bin/env bash
# Copies the root VERSION file into internal/version/VERSION_EMBED so
# go:embed can pick it up. Run this before every build or test — see
# README.md "Version" for why a separate copy exists at all.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
src="$repo_root/VERSION"
dst="$repo_root/internal/version/VERSION_EMBED"

if [[ ! -f "$src" ]]; then
  echo "sync-version: $src not found" >&2
  exit 1
fi

version="$(tr -d '[:space:]' < "$src")"
if [[ -z "$version" ]]; then
  echo "sync-version: $src is empty" >&2
  exit 1
fi

printf '%s' "$version" > "$dst"
echo "sync-version: wrote $version to $dst"
