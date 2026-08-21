#!/usr/bin/env bash
# Sparks Tool checklist §5: zero Critical/High CVEs in the shipped
# dependency graph. Runs Grype (https://github.com/anchore/grype)
# against bom.cdx.json (run scripts/generate-sbom.sh first) and fails
# the build if it finds a Critical or High severity match.
#
# Requires grype on PATH: go install github.com/anchore/grype/cmd/grype@latest
#
# Grype needs its vulnerability database, fetched from grype.anchore.io
# on first run (and periodically thereafter) -- if that host is
# unreachable (an egress-restricted network), this script says so
# explicitly and exits non-zero, the same way this project's other
# network-dependent checks do, rather than silently reporting a clean
# bill of health it never actually got.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if ! command -v grype >/dev/null 2>&1; then
  echo "check-vulnerabilities: grype not found on PATH -- go install github.com/anchore/grype/cmd/grype@latest" >&2
  exit 1
fi

if [[ ! -f bom.cdx.json ]]; then
  echo "check-vulnerabilities: bom.cdx.json not found -- run scripts/generate-sbom.sh first" >&2
  exit 1
fi

output="$(mktemp)"
trap 'rm -f "$output"' EXIT

echo "check-vulnerabilities: scanning bom.cdx.json with grype..."
if ! grype sbom:bom.cdx.json -o json --file "$output" 2>"$output.stderr"; then
  status=$?
  if grep -qi "forbidden\|unable to download\|unable to check for vulnerability database" "$output.stderr" 2>/dev/null; then
    echo "check-vulnerabilities: WARNING -- could not reach grype's vulnerability database (network policy)." >&2
    echo "check-vulnerabilities: this is an environment limitation, not a clean bill of health -- re-run this" >&2
    echo "check-vulnerabilities: script from a network-unrestricted environment before treating this build as" >&2
    echo "check-vulnerabilities: a real release." >&2
    cat "$output.stderr" >&2
    rm -f "$output.stderr"
    exit 1
  fi
  cat "$output.stderr" >&2
  rm -f "$output.stderr"
  exit "$status"
fi
rm -f "$output.stderr"

critical_and_high="$(python3 -c "
import json, sys
with open('$output') as f:
    data = json.load(f)
matches = [m for m in data.get('matches', []) if m.get('vulnerability', {}).get('severity', '').lower() in ('critical', 'high')]
for m in matches:
    vuln = m['vulnerability']
    art = m['artifact']
    print(f\"  {vuln.get('severity')}: {vuln.get('id')} in {art.get('name')}@{art.get('version')}\")
print(len(matches))
" )"

count="$(echo "$critical_and_high" | tail -1)"
if [[ "$count" -gt 0 ]]; then
  echo "check-vulnerabilities: found $count Critical/High vulnerabilities:" >&2
  echo "$critical_and_high" | head -n -1 >&2
  exit 1
fi

echo "check-vulnerabilities: no Critical/High vulnerabilities found."
