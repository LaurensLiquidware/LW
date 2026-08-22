#!/usr/bin/env bash
# Renders docs/MANUAL.md to a standalone PDF for release packaging (see
# scripts/release.sh). Two local tools, no network, no external CDN:
# pandoc converts Markdown to a single self-contained HTML file (fonts/
# CSS inlined, per this project's no-external-references policy), then a
# headless Chromium prints that HTML to PDF -- avoids pulling in a full
# LaTeX toolchain (pandoc's usual PDF path) just for one document.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if ! command -v pandoc >/dev/null 2>&1; then
  echo "render-manual-pdf: pandoc not found on PATH -- apt-get install pandoc (or your platform's equivalent)" >&2
  exit 1
fi

chromium_bin=""
for candidate in "${CHROMIUM_BIN:-}" chromium chromium-browser google-chrome google-chrome-stable \
  /opt/pw-browsers/chromium-*/chrome-linux/chrome; do
  # The glob above only expands via the loop when unquoted; skip literal
  # misses instead of erroring.
  for expanded in $candidate; do
    if command -v "$expanded" >/dev/null 2>&1 || [[ -x "$expanded" ]]; then
      chromium_bin="$expanded"
      break 2
    fi
  done
done
if [[ -z "$chromium_bin" ]]; then
  echo "render-manual-pdf: no Chromium/Chrome binary found -- set CHROMIUM_BIN or install one" >&2
  exit 1
fi

out_path="${1:-$repo_root/MANUAL.pdf}"
html_tmp="$(mktemp --suffix=.html)"
trap 'rm -f "$html_tmp"' EXIT

pandoc docs/MANUAL.md \
  --standalone \
  --embed-resources \
  --metadata pagetitle="FlexApp Vulnerability and Security Scanner — User Manual" \
  --css <(cat <<'CSS'
body { font-family: -apple-system, "Segoe UI", Helvetica, Arial, sans-serif; max-width: 46em; margin: 2em auto; padding: 0 1.5em; line-height: 1.5; color: #1a1a1a; }
h1, h2, h3 { color: #0b3d5c; }
h1 { border-bottom: 2px solid #0b3d5c; padding-bottom: 0.3em; }
h2 { border-bottom: 1px solid #ccc; padding-bottom: 0.2em; margin-top: 1.8em; }
code { background: #f2f2f2; padding: 0.1em 0.3em; border-radius: 3px; font-size: 0.92em; }
pre { background: #f2f2f2; padding: 0.8em; border-radius: 4px; overflow-x: auto; }
table { border-collapse: collapse; width: 100%; margin: 1em 0; }
th, td { border: 1px solid #ccc; padding: 0.4em 0.6em; text-align: left; }
th { background: #eef4f8; }
blockquote { border-left: 4px solid #0b3d5c; margin-left: 0; padding-left: 1em; color: #444; }
a { color: #0b6cb3; }
CSS
) \
  -o "$html_tmp"

"$chromium_bin" --headless=new --disable-gpu --no-sandbox \
  --print-to-pdf="$out_path" --no-pdf-header-footer \
  "file://$html_tmp" >/dev/null 2>&1

echo "render-manual-pdf: wrote $out_path"
