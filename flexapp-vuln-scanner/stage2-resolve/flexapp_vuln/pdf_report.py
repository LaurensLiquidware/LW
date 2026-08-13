"""Renders a single polished PDF combining the coverage summary and
vulnerability findings, via reportlab.

Same underlying data as reporting.py's Markdown renderers (coverage.py's
compute_coverage output, plus an optional vuln-matches.json) - this is a
presentation-layer alternative for a human reader/stakeholder (a report you
can hand someone), not a replacement for the machine-readable Markdown/JSON
outputs, which remain the source of truth.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import (
    PageBreak,
    Paragraph,
    SimpleDocTemplate,
    Spacer,
    Table,
    TableStyle,
)

from .reporting import build_finding_rows

_HEADER_BG = colors.HexColor("#1f2937")
_ROW_ALT_BG = colors.HexColor("#f3f4f6")
_GRID_COLOR = colors.HexColor("#d1d5db")

_SEVERITY_HEX = {
    "CRITICAL": "#7f1d1d",
    "HIGH": "#b91c1c",
    "MEDIUM": "#b45309",
    "MODERATE": "#b45309",
    "LOW": "#6b7280",
}


def _severity_hex(level: str | None) -> str:
    return _SEVERITY_HEX.get((level or "").upper(), "#374151")


def _table_style(header: bool) -> TableStyle:
    style = [
        ("FONTSIZE", (0, 0), (-1, -1), 9),
        ("GRID", (0, 0), (-1, -1), 0.5, _GRID_COLOR),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("TOPPADDING", (0, 0), (-1, -1), 4),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 4),
    ]
    if header:
        style += [
            ("BACKGROUND", (0, 0), (-1, 0), _HEADER_BG),
            ("TEXTCOLOR", (0, 0), (-1, 0), colors.white),
            ("FONTNAME", (0, 0), (-1, 0), "Helvetica-Bold"),
            ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, _ROW_ALT_BG]),
        ]
    return TableStyle(style)


def render_pdf_report(
    out_path: Path | str,
    package_name: str,
    package_meta: dict[str, Any],
    coverage: dict[str, Any],
    vuln_matches: dict[str, Any] | None,
) -> None:
    """Writes a combined coverage + findings PDF report to out_path."""
    styles = getSampleStyleSheet()
    title_style = ParagraphStyle("ReportTitle", parent=styles["Title"], fontSize=20)
    h2 = ParagraphStyle("H2", parent=styles["Heading2"], spaceBefore=14, spaceAfter=6)
    h3 = ParagraphStyle("H3", parent=styles["Heading3"], spaceBefore=10, spaceAfter=4)
    body = styles["BodyText"]
    cell = ParagraphStyle("Cell", parent=styles["BodyText"], fontSize=8, leading=10)

    story: list[Any] = [
        Paragraph("FlexApp Vulnerability Scan Report", title_style),
        Paragraph(package_name, styles["Heading2"]),
    ]

    meta_bits = []
    version = package_meta.get("versionMajorMinorBuildRevision")
    if version:
        meta_bits.append(f"Package version: {version}")
    if package_meta.get("scanFinishedUtc"):
        meta_bits.append(f"Scan completed: {package_meta['scanFinishedUtc']}")
    if meta_bits:
        story.append(Paragraph(" &nbsp;|&nbsp; ".join(meta_bits), body))
    story.append(Spacer(1, 0.2 * inch))

    # --- Coverage summary ---
    story.append(Paragraph("Resolution Coverage", h2))
    pct = coverage["coveragePercent"]
    pct_str = f"{pct:.1f}%" if pct is not None else "N/A (no candidate components found)"
    story.append(Paragraph(f"<b>Resolution coverage: {pct_str}</b>", body))
    story.append(Spacer(1, 0.1 * inch))
    summary_rows = [
        ["Total files scanned", str(coverage["totalFilesScanned"])],
        ["Files excluded (noise filtering)", str(coverage["excludedCount"])],
        ["Candidate components", str(coverage["candidateComponents"])],
        ["Components resolved", str(coverage["resolvedComponents"])],
        ["Components unresolved", str(coverage["unresolvedComponents"])],
    ]
    story.append(Table(summary_rows, colWidths=[3 * inch, 1.5 * inch], style=_table_style(header=False)))

    if coverage["resolvedByMethod"]:
        story.append(Paragraph("Resolved components, by method", h3))
        rows = [["Method", "Count"]] + [[m, str(c)] for m, c in coverage["resolvedByMethod"].items()]
        story.append(Table(rows, colWidths=[3 * inch, 1.5 * inch], repeatRows=1, style=_table_style(header=True)))

    if coverage["excludedByReason"]:
        story.append(Paragraph("Excluded files, by reason", h3))
        rows = [["Reason", "Count"]] + [[r, str(c)] for r, c in coverage["excludedByReason"].items()]
        story.append(Table(rows, colWidths=[3 * inch, 1.5 * inch], repeatRows=1, style=_table_style(header=True)))

    unresolved_files = coverage["unresolvedFiles"]
    if unresolved_files:
        story.append(Paragraph(f"Unresolved components ({len(unresolved_files)})", h3))
        rows = [["Path", "Component type"]] + [
            [Paragraph(f["relativePath"] or "", cell), f["componentType"] or ""] for f in unresolved_files
        ]
        story.append(Table(rows, colWidths=[5 * inch, 1.3 * inch], repeatRows=1, style=_table_style(header=True)))

    # --- Findings ---
    story.append(PageBreak())
    story.append(Paragraph("Vulnerability Findings", h2))

    if vuln_matches is None:
        story.append(Paragraph(
            "No vulnerability-matching data was supplied for this report "
            "(the <i>resolve</i> step wasn't run, or its output wasn't passed "
            "in). This is not the same thing as “no vulnerabilities "
            "found.”",
            body,
        ))
    else:
        rows = build_finding_rows(vuln_matches)
        confirmed = [r for r in rows if r["confidence"] in ("exact-purl", "mapped-cpe")]
        heuristic = [r for r in rows if r["confidence"] == "heuristic"]

        story.append(Paragraph(f"Confirmed matches — exact-purl / mapped-cpe ({len(confirmed)})", h3))
        if confirmed:
            story.append(_findings_table(confirmed, cell))
        else:
            story.append(Paragraph("None.", body))

        story.append(Spacer(1, 0.2 * inch))
        story.append(Paragraph(f"Low-confidence matches — heuristic ({len(heuristic)})", h3))
        story.append(Paragraph(
            "These matches used automatic vendor/product normalization, not "
            "a verified purl or a curated CPE mapping. <b>Verify manually "
            "before treating any of these as a confirmed finding.</b>",
            body,
        ))
        if heuristic:
            story.append(_findings_table(heuristic, cell))
        else:
            story.append(Paragraph("None.", body))

    doc = SimpleDocTemplate(
        str(out_path),
        pagesize=letter,
        title=f"FlexApp Vulnerability Report - {package_name}",
        topMargin=0.6 * inch,
        bottomMargin=0.6 * inch,
        leftMargin=0.6 * inch,
        rightMargin=0.6 * inch,
    )
    doc.build(story)


def _findings_table(entries: list[dict[str, Any]], cell_style: ParagraphStyle) -> Table:
    header = ["Severity", "ID", "Component", "Version", "Summary", "Source", "Confidence"]
    data = [header]
    for r in entries:
        severity = r["severityLevel"] or "UNKNOWN"
        severity_para = Paragraph(
            f'<font color="{_severity_hex(severity)}"><b>{severity}</b></font>', cell_style
        )
        data.append([
            severity_para,
            Paragraph(r["id"] or "", cell_style),
            Paragraph(r["product"] or "", cell_style),
            Paragraph(r["version"] or "", cell_style),
            Paragraph(r["summary"] or "", cell_style),
            Paragraph(r["source"] or "", cell_style),
            Paragraph(r["confidence"] or "", cell_style),
        ])
    col_widths = [0.65 * inch, 1.05 * inch, 1.25 * inch, 0.7 * inch, 2.65 * inch, 0.55 * inch, 0.85 * inch]
    return Table(data, colWidths=col_widths, repeatRows=1, style=_table_style(header=True))
