"""Resolution coverage computation, per PLAN.md's exact definition.

Deliberately independent of OSV/NVD matching - the headline coverage
percentage is entirely about identity resolution (did Stage 1 figure out
what a file is?), not vulnerability matching (did we find a CVE for it?).
This means coverage-report.md can always be produced from an inventory
JSON alone, with no network access required - which matters, since the
whole point of this PoC is an honest coverage number even when "we need
SBOMs supplied at package build time" is the actual finding.

Definitions (verbatim intent from PLAN.md):
  denominator = every file with excluded: false (candidate components)
  numerator   = of those, every file with identity != null (resolved)
  unresolved  = denominator - numerator
  excluded files are reported but outside both numerator and denominator
"""

from __future__ import annotations

import collections
from typing import Any

from .inventory import iter_non_excluded_files


def compute_coverage(inventory: dict[str, Any]) -> dict[str, Any]:
    files = inventory.get("files", [])

    excluded_files = [f for f in files if f.get("excluded", False)]
    excluded_by_reason: collections.Counter[str] = collections.Counter(
        f.get("exclusionReason") or "unknown" for f in excluded_files
    )

    candidates = list(iter_non_excluded_files(inventory))
    resolved = [f for f in candidates if f.get("identity")]
    unresolved = [f for f in candidates if not f.get("identity")]

    resolved_by_method: collections.Counter[str] = collections.Counter(
        f["identity"]["method"] for f in resolved
    )

    total_files = len(files)
    denominator = len(candidates)
    numerator = len(resolved)
    coverage_pct = (numerator / denominator * 100) if denominator else None

    return {
        "totalFilesScanned": total_files,
        "excludedCount": len(excluded_files),
        "excludedByReason": dict(sorted(excluded_by_reason.items())),
        "candidateComponents": denominator,
        "resolvedComponents": numerator,
        "resolvedByMethod": dict(sorted(resolved_by_method.items())),
        "unresolvedComponents": len(unresolved),
        "unresolvedFiles": [
            {
                "relativePath": f.get("relativePath"),
                "componentType": f.get("componentType"),
                "readError": f.get("readError"),
            }
            for f in unresolved
        ],
        "coveragePercent": coverage_pct,
    }
