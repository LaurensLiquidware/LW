"""Renders coverage-report.md and findings.md from computed data.

coverage-report.md needs only compute_coverage()'s output (no network, no
vuln-matches needed) - see coverage.py's module docstring for why that
independence matters. findings.md needs a vuln-matches.json (from the
`resolve` command); if none is supplied, it says so plainly rather than
rendering an empty-looking report that could be mistaken for "no
vulnerabilities found."
"""

from __future__ import annotations

from typing import Any

_SEVERITY_RANK = {
    "CRITICAL": 0,
    "HIGH": 1,
    "MODERATE": 2,  # OSV/GHSA sometimes uses "Moderate" where NVD uses "Medium"
    "MEDIUM": 2,
    "LOW": 3,
    "NONE": 4,
}


def _severity_rank(level: str | None) -> int:
    return _SEVERITY_RANK.get((level or "").upper(), 5)


def render_coverage_report(coverage: dict[str, Any], package_name: str) -> str:
    lines = [f"# Coverage Report — {package_name}", ""]

    pct = coverage["coveragePercent"]
    pct_str = f"{pct:.1f}%" if pct is not None else "N/A (no candidate components found)"
    lines += [
        f"**Resolution coverage: {pct_str}**",
        "",
        f"- Total files scanned: {coverage['totalFilesScanned']}",
        f"- Files excluded (noise filtering): {coverage['excludedCount']}",
        f"- Candidate components (excluded: false): {coverage['candidateComponents']}",
        f"- Components resolved: {coverage['resolvedComponents']}",
        f"- Components unresolved: {coverage['unresolvedComponents']}",
        "",
    ]

    lines.append("## Excluded files, by reason")
    lines.append("")
    if coverage["excludedByReason"]:
        lines.append("| Reason | Count |")
        lines.append("|---|---|")
        for reason, count in coverage["excludedByReason"].items():
            lines.append(f"| {reason} | {count} |")
    else:
        lines.append("None excluded.")
    lines.append("")

    lines.append("## Resolved components, by method")
    lines.append("")
    if coverage["resolvedByMethod"]:
        lines.append("| Method | Count |")
        lines.append("|---|---|")
        for method, count in coverage["resolvedByMethod"].items():
            lines.append(f"| {method} | {count} |")
    else:
        lines.append("None resolved.")
    lines.append("")

    lines.append("## Unresolved components")
    lines.append("")
    unresolved_files = coverage["unresolvedFiles"]
    if unresolved_files:
        lines.append("| Path | Component type | Read error |")
        lines.append("|---|---|---|")
        for f in unresolved_files:
            read_error = f["readError"] or ""
            lines.append(f"| `{f['relativePath']}` | {f['componentType']} | {read_error} |")
    else:
        lines.append("None — every candidate component resolved.")
    lines.append("")

    return "\n".join(lines)


def render_findings(vuln_matches: dict[str, Any] | None, package_name: str) -> str:
    lines = [f"# Findings — {package_name}", ""]

    if vuln_matches is None:
        lines += [
            "No vulnerability-matching data was supplied for this report "
            "(the `resolve` step wasn't run, or its output wasn't passed in). "
            "This report has no findings to show - that is not the same "
            "thing as \"no vulnerabilities found.\"",
            "",
        ]
        return "\n".join(lines)

    rows = []
    for component in vuln_matches.get("components", []):
        identity = component.get("identity") or {}
        confidence = component.get("confidence")
        for vuln in component.get("vulnerabilities", []):
            rows.append({
                "severityLevel": vuln.get("severityLevel"),
                "id": vuln.get("id"),
                "summary": vuln.get("summary") or "",
                "product": identity.get("product") or component.get("relativePath"),
                "version": identity.get("version") or "",
                "relativePath": component.get("relativePath"),
                "confidence": confidence,
                "source": vuln.get("source"),
            })

    rows.sort(key=lambda r: (_severity_rank(r["severityLevel"]), r["id"] or ""))

    confirmed = [r for r in rows if r["confidence"] in ("exact-purl", "mapped-cpe")]
    heuristic = [r for r in rows if r["confidence"] == "heuristic"]

    if not rows:
        lines.append("No vulnerability matches found.")
        lines.append("")
        return "\n".join(lines)

    def _render_table(entries: list[dict[str, Any]]) -> list[str]:
        table = ["| Severity | ID | Component | Version | Summary | Source | Confidence |",
                 "|---|---|---|---|---|---|---|"]
        for r in entries:
            severity = r["severityLevel"] or "UNKNOWN"
            summary = (r["summary"][:100] + "…") if len(r["summary"]) > 100 else r["summary"]
            table.append(
                f"| {severity} | {r['id']} | {r['product']} | {r['version']} | "
                f"{summary} | {r['source']} | {r['confidence']} |"
            )
        return table

    lines.append("## Confirmed matches (exact-purl / mapped-cpe)")
    lines.append("")
    if confirmed:
        lines += _render_table(confirmed)
    else:
        lines.append("None.")
    lines.append("")

    lines.append("## Low-confidence matches (heuristic)")
    lines.append("")
    lines.append(
        "These matches used automatic vendor/product normalization, not a "
        "verified purl or a curated CPE mapping. **Verify manually before "
        "treating any of these as a confirmed finding.**"
    )
    lines.append("")
    if heuristic:
        lines += _render_table(heuristic)
    else:
        lines.append("None.")
    lines.append("")

    return "\n".join(lines)
