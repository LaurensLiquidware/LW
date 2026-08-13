"""Renders coverage-report.md and findings.md from computed data.

coverage-report.md needs only compute_coverage()'s output (no network, no
vuln-matches needed) - see coverage.py's module docstring for why that
independence matters. findings.md needs a vuln-matches.json (from the
`resolve` command); if none is supplied, it says so plainly rather than
rendering an empty-looking report that could be mistaken for "no
vulnerabilities found."
"""

from __future__ import annotations

import csv
import io
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


def vulnerability_url(vuln_id: str | None) -> str | None:
    """Canonical public reference page for a vulnerability id, or None if
    the id doesn't match a recognized scheme. CVE ids go to NVD (the
    authoritative record regardless of which source matched it); GHSA ids
    go to GitHub's advisory database; everything else OSV-sourced
    (PYSEC-, RUSTSEC-, GO-, DSA-, ...) goes to osv.dev, which mirrors and
    links out to the real upstream advisory for every ecosystem it tracks.
    """
    if not vuln_id:
        return None
    if vuln_id.startswith("CVE-"):
        return f"https://nvd.nist.gov/vuln/detail/{vuln_id}"
    if vuln_id.startswith("GHSA-"):
        return f"https://github.com/advisories/{vuln_id}"
    return f"https://osv.dev/vulnerability/{vuln_id}"


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


def build_finding_rows(vuln_matches: dict[str, Any]) -> list[dict[str, Any]]:
    """Flattens vuln-matches.json into one row per distinct (component,
    vulnerability), severity-sorted. Shared by the Markdown and PDF
    renderers so both dedupe and sort identically.

    Keyed by (purl-or-cpe, vulnerability id) - the same physical component
    (e.g. the same bundled sqlite3.dll copied to more than one path) can
    appear as more than one candidate row here, each carrying an identical
    vulnerability list. Without deduping by identity, every CVE for that
    component would be rendered once per file instead of once overall -
    this is the same "one entry per distinct component" rule sbom.py
    already applies when building the SBOM.
    """
    rows_by_key: dict[tuple[str, str], dict[str, Any]] = {}
    for component in vuln_matches.get("components", []):
        identity = component.get("identity") or {}
        confidence = component.get("confidence")
        dedup_identity = component.get("purl") or component.get("cpe") or component.get("relativePath")
        for vuln in component.get("vulnerabilities", []):
            key = (dedup_identity, vuln.get("id") or "")
            if key in rows_by_key:
                continue
            rows_by_key[key] = {
                "severityLevel": vuln.get("severityLevel"),
                "id": vuln.get("id"),
                "url": vulnerability_url(vuln.get("id")),
                "summary": vuln.get("summary") or "",
                "product": identity.get("product") or component.get("relativePath"),
                "version": identity.get("version") or "",
                "relativePath": component.get("relativePath"),
                "confidence": confidence,
                "source": vuln.get("source"),
            }

    rows = list(rows_by_key.values())
    rows.sort(key=lambda r: (_severity_rank(r["severityLevel"]), r["id"] or ""))
    return rows


_DISPLAY_SEVERITIES = ["CRITICAL", "HIGH", "MEDIUM", "LOW"]


def count_by_severity(rows: list[dict[str, Any]]) -> dict[str, int]:
    """Counts build_finding_rows' output into the 4 severity buckets shown
    in the UI's summary counts (e.g. the "Scans Run This Session" table).
    "Moderate" (OSV/GHSA's spelling) folds into "Medium" (NVD's spelling) -
    same rank as _SEVERITY_RANK already treats them. Anything else
    (missing, "NONE", an unrecognized string) isn't one of these 4 and
    isn't counted - the caller already knows the total row count
    separately if it needs to reconcile the difference.
    """
    counts = {level: 0 for level in _DISPLAY_SEVERITIES}
    for r in rows:
        level = (r.get("severityLevel") or "").upper()
        if level == "MODERATE":
            level = "MEDIUM"
        if level in counts:
            counts[level] += 1
    return counts


_CSV_FIELDS = ["severityLevel", "id", "url", "product", "version", "summary", "source", "confidence", "relativePath"]
_CSV_HEADER = ["Severity", "ID", "URL", "Component", "Version", "Summary", "Source", "Confidence", "Path"]


def render_findings_csv(vuln_matches: dict[str, Any]) -> str:
    """CSV of every finding row - both confirmed and heuristic, in one
    table with a Confidence column, since a spreadsheet is more useful
    filtered/sorted by the reader than pre-split into two sheets. Only
    called when there IS vuln-matches data (see write_reports) - unlike
    render_findings, this has no way to write an explanatory sentence
    for the "no data supplied" case, so callers must not confuse an
    absent file with "zero vulnerabilities found."
    """
    rows = build_finding_rows(vuln_matches)
    buf = io.StringIO()
    writer = csv.writer(buf)
    writer.writerow(_CSV_HEADER)
    for r in rows:
        writer.writerow([r.get(field) or "" for field in _CSV_FIELDS])
    return buf.getvalue()


def diff_finding_rows(
    old_rows: list[dict[str, Any]], new_rows: list[dict[str, Any]]
) -> dict[str, Any]:
    """Compares two scans' already-flattened finding rows (build_finding_rows'
    output, confirmed + heuristic combined) and reports what changed.
    Matched by (product, version, vulnerability id) - not the internal
    purl/cpe dedup key build_finding_rows uses internally, since two scans
    of the "same" package can resolve a component via a different purl/cpe
    confidence path (e.g. a CPE mapping added between runs) while still
    being the same real-world component+CVE pairing a human would want
    treated as unchanged.
    """
    def key(r: dict[str, Any]) -> tuple[Any, Any, Any]:
        return (r["product"], r["version"], r["id"])

    old_keys = {key(r) for r in old_rows}
    new_keys = {key(r) for r in new_rows}

    new_findings = [r for r in new_rows if key(r) not in old_keys]
    resolved_findings = [r for r in old_rows if key(r) not in new_keys]
    unchanged_count = sum(1 for r in new_rows if key(r) in old_keys)

    return {
        "new_findings": new_findings,
        "resolved_findings": resolved_findings,
        "unchanged_count": unchanged_count,
    }


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

    rows = build_finding_rows(vuln_matches)
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
            id_cell = f"[{r['id']}]({r['url']})" if r["url"] else (r["id"] or "")
            table.append(
                f"| {severity} | {id_cell} | {r['product']} | {r['version']} | "
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
