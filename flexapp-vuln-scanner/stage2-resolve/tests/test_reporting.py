from pathlib import Path

from flexapp_vuln.coverage import compute_coverage
from flexapp_vuln.inventory import load_inventory
from flexapp_vuln.reporting import (
    build_finding_rows,
    count_by_severity,
    diff_finding_rows,
    render_coverage_report,
    render_findings,
    render_findings_csv,
    vulnerability_url,
)

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


def test_build_finding_rows_collects_all_affected_files_for_a_shared_vulnerability():
    # Same real-world component (same purl), copied to 3 different paths -
    # one row, but every affected path must be recoverable from it.
    vuln_matches = {"components": [
        {
            "relativePath": "Program Files\\App\\outer-app.jar",
            "identity": {"product": "OuterApp", "version": "9.9.9"},
            "purl": "pkg:maven/a/outer-app@9.9.9",
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": "CVE-2026-0001", "summary": "x", "severityLevel": "CRITICAL", "source": "nvd"}],
        },
        {
            "relativePath": "Program Files\\App\\plugins\\outer-app-legacy.jar",
            "identity": {"product": "OuterApp", "version": "9.9.9"},
            "purl": "pkg:maven/a/outer-app@9.9.9",
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": "CVE-2026-0001", "summary": "x", "severityLevel": "CRITICAL", "source": "nvd"}],
        },
        {
            "relativePath": "Data\\cache\\outer-app.jar.bak",
            "identity": {"product": "OuterApp", "version": "9.9.9"},
            "purl": "pkg:maven/a/outer-app@9.9.9",
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": "CVE-2026-0001", "summary": "x", "severityLevel": "CRITICAL", "source": "nvd"}],
        },
    ]}
    rows = build_finding_rows(vuln_matches)

    assert len(rows) == 1
    assert rows[0]["relativePaths"] == [
        "Data\\cache\\outer-app.jar.bak",
        "Program Files\\App\\outer-app.jar",
        "Program Files\\App\\plugins\\outer-app-legacy.jar",
    ]


def test_build_finding_rows_distinct_vulnerabilities_on_same_component_each_get_their_own_files():
    vuln_matches = {"components": [
        {
            "relativePath": "a.jar",
            "identity": {"product": "a", "version": "1.0"},
            "purl": "pkg:maven/a/a@1.0",
            "confidence": "exact-purl",
            "vulnerabilities": [
                {"id": "CVE-2026-0001", "summary": "x", "severityLevel": "CRITICAL", "source": "nvd"},
                {"id": "CVE-2026-0002", "summary": "y", "severityLevel": "HIGH", "source": "nvd"},
            ],
        },
    ]}
    rows = build_finding_rows(vuln_matches)

    assert len(rows) == 2
    assert all(r["relativePaths"] == ["a.jar"] for r in rows)


def test_build_finding_rows_no_relative_path_gives_empty_list():
    vuln_matches = {"components": [
        {
            "relativePath": None,
            "identity": {"product": "a", "version": "1.0"},
            "purl": "pkg:maven/a/a@1.0",
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": "CVE-2023-0001", "summary": "x", "severityLevel": "HIGH", "source": "nvd"}],
        },
    ]}
    rows = build_finding_rows(vuln_matches)
    assert rows[0]["relativePaths"] == []


def test_render_findings_shows_affected_files_column():
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
    report = render_findings(vuln_matches, "TestApp")

    assert "Affected Files" in report
    assert "`a\\outer-app.jar`<br>`b\\outer-app-legacy.jar`" in report


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


def test_render_findings_csv_has_header_and_one_row_per_finding():
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
    csv_text = render_findings_csv(vuln_matches)
    lines = csv_text.strip().splitlines()

    assert lines[0] == "Severity,ID,URL,Component,Version,Summary,Source,Confidence,Affected Files"
    assert len(lines) == 2
    assert "CVE-2023-0001" in lines[1]
    assert "https://nvd.nist.gov/vuln/detail/CVE-2023-0001" in lines[1]
    assert "exact-purl" in lines[1]
    assert "a.jar" in lines[1]


def test_render_findings_csv_joins_multiple_affected_files_with_semicolon():
    vuln_matches = {"components": [
        {
            "relativePath": "a\\outer-app.jar",
            "identity": {"product": "OuterApp", "version": "9.9.9"},
            "cpe": None,
            "purl": "pkg:maven/a/outer-app@9.9.9",
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": "CVE-2026-0001", "summary": "x", "severityLevel": "CRITICAL", "source": "nvd"}],
        },
        {
            "relativePath": "b\\plugins\\outer-app-legacy.jar",
            "identity": {"product": "OuterApp", "version": "9.9.9"},
            "cpe": None,
            "purl": "pkg:maven/a/outer-app@9.9.9",
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": "CVE-2026-0001", "summary": "x", "severityLevel": "CRITICAL", "source": "nvd"}],
        },
    ]}
    csv_text = render_findings_csv(vuln_matches)
    lines = csv_text.strip().splitlines()

    assert len(lines) == 2  # still one row - not one per file
    assert "a\\outer-app.jar; b\\plugins\\outer-app-legacy.jar" in lines[1]


def test_render_findings_csv_empty_when_no_findings():
    vuln_matches = {"components": [
        {"relativePath": "a.jar", "identity": {"product": "a", "version": "1.0"},
         "confidence": "exact-purl", "vulnerabilities": []}
    ]}
    csv_text = render_findings_csv(vuln_matches)
    lines = csv_text.strip().splitlines()
    assert len(lines) == 1  # header only


def test_count_by_severity_counts_each_bucket():
    rows = [
        {"severityLevel": "CRITICAL"},
        {"severityLevel": "CRITICAL"},
        {"severityLevel": "HIGH"},
        {"severityLevel": "MEDIUM"},
        {"severityLevel": "Moderate"},  # OSV/GHSA spelling, folds into MEDIUM
        {"severityLevel": "LOW"},
        {"severityLevel": "LOW"},
        {"severityLevel": "LOW"},
    ]
    assert count_by_severity(rows) == {"CRITICAL": 2, "HIGH": 1, "MEDIUM": 2, "LOW": 3}


def test_count_by_severity_ignores_unknown_and_missing():
    rows = [{"severityLevel": None}, {"severityLevel": "NONE"}, {"severityLevel": "WEIRD"}]
    assert count_by_severity(rows) == {"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0}


def test_count_by_severity_empty_rows():
    assert count_by_severity([]) == {"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0}


def test_diff_finding_rows_detects_new_and_resolved():
    old_rows = [
        {"product": "a", "version": "1.0", "id": "CVE-2020-0001"},
        {"product": "b", "version": "2.0", "id": "CVE-2020-0002"},
    ]
    new_rows = [
        {"product": "b", "version": "2.0", "id": "CVE-2020-0002"},
        {"product": "c", "version": "3.0", "id": "CVE-2020-0003"},
    ]

    diff = diff_finding_rows(old_rows, new_rows)

    assert [r["id"] for r in diff["new_findings"]] == ["CVE-2020-0003"]
    assert [r["id"] for r in diff["resolved_findings"]] == ["CVE-2020-0001"]
    assert diff["unchanged_count"] == 1


def test_diff_finding_rows_same_id_different_version_counts_as_both_changes():
    old_rows = [{"product": "a", "version": "1.0", "id": "CVE-2020-0001"}]
    new_rows = [{"product": "a", "version": "2.0", "id": "CVE-2020-0001"}]

    diff = diff_finding_rows(old_rows, new_rows)

    assert len(diff["new_findings"]) == 1
    assert len(diff["resolved_findings"]) == 1
    assert diff["unchanged_count"] == 0


def test_diff_finding_rows_empty_inputs():
    diff = diff_finding_rows([], [])
    assert diff == {"new_findings": [], "resolved_findings": [], "unchanged_count": 0}


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
