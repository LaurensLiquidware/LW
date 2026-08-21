from pathlib import Path

from flexapp_vuln.coverage import compute_coverage
from flexapp_vuln.inventory import load_inventory
from flexapp_vuln.pdf_report import render_pdf_report

FIXTURE = Path(__file__).parent / "fixtures" / "sample.inventory.json"


def test_render_pdf_report_writes_valid_pdf_with_coverage_and_findings(tmp_path):
    inventory = load_inventory(FIXTURE)
    coverage = compute_coverage(inventory)
    vuln_matches = {"components": [
        {
            "relativePath": "a.jar",
            "identity": {"product": "Acme Library", "version": "1.0"},
            "confidence": "exact-purl",
            "vulnerabilities": [
                {"id": "GHSA-aaaa", "summary": "Something bad happened", "severityLevel": "CRITICAL", "source": "osv"}
            ],
        },
    ]}

    out_path = tmp_path / "report.pdf"
    render_pdf_report(
        out_path,
        package_name="TestApp",
        package_meta={"versionMajorMinorBuildRevision": "1.2.3.4", "scanFinishedUtc": "2026-08-13T00:00:00Z"},
        coverage=coverage,
        vuln_matches=vuln_matches,
    )

    assert out_path.exists()
    raw = out_path.read_bytes()
    assert raw.startswith(b"%PDF-")
    assert raw.rstrip().endswith(b"%%EOF")
    assert len(raw) > 1000


def test_render_pdf_report_handles_multiple_affected_files(tmp_path):
    inventory = load_inventory(FIXTURE)
    coverage = compute_coverage(inventory)
    vuln_matches = {"components": [
        {
            "relativePath": "a\\outer-app.jar",
            "identity": {"product": "OuterApp", "version": "9.9.9"},
            "purl": "pkg:maven/a/outer-app@9.9.9",
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": "CVE-2026-0001", "summary": "x", "severityLevel": "CRITICAL", "source": "nvd"}],
        },
        {
            "relativePath": "b\\outer-app-legacy.jar",
            "identity": {"product": "OuterApp", "version": "9.9.9"},
            "purl": "pkg:maven/a/outer-app@9.9.9",
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": "CVE-2026-0001", "summary": "x", "severityLevel": "CRITICAL", "source": "nvd"}],
        },
    ]}

    out_path = tmp_path / "report.pdf"
    render_pdf_report(
        out_path, package_name="TestApp", package_meta={}, coverage=coverage, vuln_matches=vuln_matches,
    )

    assert out_path.exists()
    assert out_path.read_bytes().startswith(b"%PDF-")


def test_render_pdf_report_handles_no_vuln_matches(tmp_path):
    inventory = load_inventory(FIXTURE)
    coverage = compute_coverage(inventory)

    out_path = tmp_path / "report.pdf"
    render_pdf_report(
        out_path,
        package_name="TestApp",
        package_meta={},
        coverage=coverage,
        vuln_matches=None,
    )

    assert out_path.exists()
    assert out_path.read_bytes().startswith(b"%PDF-")


def test_render_pdf_report_handles_empty_findings(tmp_path):
    inventory = load_inventory(FIXTURE)
    coverage = compute_coverage(inventory)
    vuln_matches = {"components": [
        {"relativePath": "a.jar", "identity": {"product": "a", "version": "1.0"},
         "confidence": "exact-purl", "vulnerabilities": []}
    ]}

    out_path = tmp_path / "report.pdf"
    render_pdf_report(
        out_path,
        package_name="TestApp",
        package_meta={},
        coverage=coverage,
        vuln_matches=vuln_matches,
    )

    assert out_path.exists()
    assert out_path.read_bytes().startswith(b"%PDF-")


def test_render_pdf_report_zero_candidates_does_not_crash(tmp_path):
    coverage = {
        "totalFilesScanned": 1,
        "excludedCount": 1,
        "excludedByReason": {"font-file": 1},
        "candidateComponents": 0,
        "resolvedComponents": 0,
        "resolvedByMethod": {},
        "unresolvedComponents": 0,
        "unresolvedFiles": [],
        "coveragePercent": None,
    }

    out_path = tmp_path / "report.pdf"
    render_pdf_report(
        out_path,
        package_name="TestApp",
        package_meta={},
        coverage=coverage,
        vuln_matches=None,
    )

    assert out_path.exists()
    assert out_path.read_bytes().startswith(b"%PDF-")
