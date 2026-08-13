from pathlib import Path

from flexapp_vuln.coverage import compute_coverage
from flexapp_vuln.inventory import load_inventory
from flexapp_vuln.reporting import build_finding_rows, render_coverage_report, render_findings, vulnerability_url

FIXTURE = Path(__file__).parent / "fixtures" / "sample.inventory.json"


def test_vulnerability_url_cve_goes_to_nvd():
    assert vulnerability_url("CVE-2023-51791") == "https://nvd.nist.gov/vuln/detail/CVE-2023-51791"


def test_vulnerability_url_ghsa_goes_to_github_advisories():
    assert vulnerability_url("GHSA-aaaa-bbbb-cccc") == "https://github.com/advisories/GHSA-aaaa-bbbb-cccc"


def test_vulnerability_url_other_osv_ids_go_to_osv_dev():
    assert vulnerability_url("PYSEC-2021-1") == "https://osv.dev/vulnerability/PYSEC-2021-1"


def test_vulnerability_url_none_for_missing_id():
    assert vulnerability_url(None) is None
    assert vulnerability_url("") is None


def test_build_finding_rows_includes_url():
    vuln_matches = {"components": [
        {
            "relativePath": "a.jar",
            "identity": {"product": "a", "version": "1.0"},
            "confidence": "exact-purl",
            "vulnerabilities": [
                {"id": "CVE-2023-0001", "summary": "x", "severityLevel": "HIGH", "source": "nvd"}
            ],
        },
    ]}
    rows = build_finding_rows(vuln_matches)
    assert rows[0]["url"] == "https://nvd.nist.gov/vuln/detail/CVE-2023-0001"


def test_render_findings_links_the_id():
    vuln_matches = {"components": [
        {
            "relativePath": "a.jar",
            "identity": {"product": "a", "version": "1.0"},
            "confidence": "exact-purl",
            "vulnerabilities": [
                {"id": "CVE-2023-0001", "summary": "x", "severityLevel": "HIGH", "source": "nvd"}
            ],
        },
    ]}
    report = render_findings(vuln_matches, "TestApp")
    assert "[CVE-2023-0001](https://nvd.nist.gov/vuln/detail/CVE-2023-0001)" in report


def test_render_coverage_report_contains_required_sections():
    inventory = load_inventory(FIXTURE)
    coverage = compute_coverage(inventory)
    report = render_coverage_report(coverage, "TestApp")

    assert "TestApp" in report
    assert "Resolution coverage: 66.7%" in report
    assert "Total files scanned: 4" in report
    assert "Files excluded (noise filtering): 1" in report
    assert "Candidate components (excluded: false): 3" in report
    assert "Components resolved: 2" in report
    assert "Components unresolved: 1" in report
    assert "os-system-path | 1" in report
    assert "jar-pom-properties | 1" in report
    assert "unresolved.bin" in report


def test_render_coverage_report_zero_candidates_says_na():
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
    report = render_coverage_report(coverage, "TestApp")
    assert "N/A" in report


def test_render_findings_no_data_says_so_plainly():
    report = render_findings(None, "TestApp")
    assert "No vulnerability-matching data was supplied" in report
    assert "not the same thing as" in report


def test_render_findings_no_matches_found():
    vuln_matches = {"components": [
        {"relativePath": "a.jar", "identity": {"product": "a", "version": "1.0"},
         "confidence": "exact-purl", "vulnerabilities": []}
    ]}
    report = render_findings(vuln_matches, "TestApp")
    assert "No vulnerability matches found." in report


def test_render_findings_separates_confirmed_from_heuristic():
    vuln_matches = {"components": [
        {
            "relativePath": "a.jar",
            "identity": {"product": "a", "version": "1.0"},
            "confidence": "exact-purl",
            "vulnerabilities": [
                {"id": "GHSA-aaaa", "summary": "Bad thing", "severityLevel": "HIGH", "source": "osv"}
            ],
        },
        {
            "relativePath": "b.exe",
            "identity": {"product": "b", "version": "2.0"},
            "confidence": "heuristic",
            "vulnerabilities": [
                {"id": "CVE-2023-9999", "summary": "Maybe bad", "severityLevel": "LOW", "source": "nvd"}
            ],
        },
    ]}
    report = render_findings(vuln_matches, "TestApp")

    confirmed_section = report.split("## Low-confidence")[0]
    heuristic_section = report.split("## Low-confidence")[1]

    assert "GHSA-aaaa" in confirmed_section
    assert "CVE-2023-9999" not in confirmed_section
    assert "CVE-2023-9999" in heuristic_section
    assert "Verify manually" in heuristic_section


def test_render_findings_dedupes_same_cve_across_files_sharing_an_identity():
    # Two physical files (e.g. the same bundled sqlite3.dll copied to two
    # locations) can both resolve to the same CPE and carry an identical
    # vulnerability list - each CVE should appear once, not once per file.
    vuln_matches = {"components": [
        {
            "relativePath": "a\\sqlite3.dll",
            "identity": {"product": "SQLite", "version": "3.15.2"},
            "cpe": "cpe:2.3:a:sqlite:sqlite:3.15.2:*:*:*:*:*:*:*",
            "confidence": "mapped-cpe",
            "vulnerabilities": [
                {"id": "CVE-2017-10989", "summary": "Bad thing", "severityLevel": "CRITICAL", "source": "nvd"}
            ],
        },
        {
            "relativePath": "b\\sqlite3.dll",
            "identity": {"product": "SQLite", "version": "3.15.2"},
            "cpe": "cpe:2.3:a:sqlite:sqlite:3.15.2:*:*:*:*:*:*:*",
            "confidence": "mapped-cpe",
            "vulnerabilities": [
                {"id": "CVE-2017-10989", "summary": "Bad thing", "severityLevel": "CRITICAL", "source": "nvd"}
            ],
        },
    ]}
    report = render_findings(vuln_matches, "TestApp")

    # Count the linked-row form, not raw substring - the id also appears
    # once more inside its own URL now that findings link out (see
    # vulnerability_url), which would otherwise double-count each row.
    assert report.count("[CVE-2017-10989]") == 1


def test_render_findings_same_cve_different_versions_both_shown():
    # Different versions of the same product legitimately need separate
    # rows - dedup must key on identity (purl/cpe), not just the CVE id.
    vuln_matches = {"components": [
        {
            "relativePath": "a\\sqlite3.dll",
            "identity": {"product": "SQLite", "version": "3.15.2"},
            "cpe": "cpe:2.3:a:sqlite:sqlite:3.15.2:*:*:*:*:*:*:*",
            "confidence": "mapped-cpe",
            "vulnerabilities": [
                {"id": "CVE-2020-13434", "summary": "x", "severityLevel": "MEDIUM", "source": "nvd"}
            ],
        },
        {
            "relativePath": "b\\sqlite3.dll",
            "identity": {"product": "SQLite", "version": "3.7.15"},
            "cpe": "cpe:2.3:a:sqlite:sqlite:3.7.15:*:*:*:*:*:*:*",
            "confidence": "mapped-cpe",
            "vulnerabilities": [
                {"id": "CVE-2020-13434", "summary": "x", "severityLevel": "MEDIUM", "source": "nvd"}
            ],
        },
    ]}
    report = render_findings(vuln_matches, "TestApp")

    assert report.count("[CVE-2020-13434]") == 2
    assert "3.15.2" in report
    assert "3.7.15" in report


def test_render_findings_sorts_by_severity_critical_first():
    vuln_matches = {"components": [
        {
            "relativePath": "a.jar", "identity": {"product": "a", "version": "1.0"},
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": "LOW-1", "summary": "", "severityLevel": "LOW", "source": "osv"}],
        },
        {
            "relativePath": "b.jar", "identity": {"product": "b", "version": "1.0"},
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": "CRIT-1", "summary": "", "severityLevel": "CRITICAL", "source": "osv"}],
        },
    ]}
    report = render_findings(vuln_matches, "TestApp")
    assert report.index("CRIT-1") < report.index("LOW-1")
