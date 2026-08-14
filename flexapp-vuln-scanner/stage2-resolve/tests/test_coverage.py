from pathlib import Path

import pytest

from flexapp_vuln.coverage import compute_coverage
from flexapp_vuln.inventory import load_inventory

FIXTURE = Path(__file__).parent / "fixtures" / "sample.inventory.json"


def test_compute_coverage_matches_fixture_exactly():
    inventory = load_inventory(FIXTURE)
    coverage = compute_coverage(inventory)

    # Fixture: 4 files total. kernel32.dll excluded (os-system-path).
    # Of the 3 candidates: outer-app.jar and libcrypto-1_1.dll resolved,
    # unresolved.bin did not.
    assert coverage["totalFilesScanned"] == 4
    assert coverage["excludedCount"] == 1
    assert coverage["excludedByReason"] == {"os-system-path": 1}
    assert coverage["candidateComponents"] == 3
    assert coverage["resolvedComponents"] == 2
    assert coverage["unresolvedComponents"] == 1
    assert coverage["coveragePercent"] == pytest.approx(2 / 3 * 100)
    assert coverage["resolvedByMethod"] == {"jar-pom-properties": 1, "string-signature": 1}
    assert coverage["unresolvedFiles"] == [{
        "relativePath": "Program Files\\App\\unresolved.bin",
        "componentType": "unknown",
        "readError": None,
    }]


def test_compute_coverage_zero_candidates_reports_none_not_error():
    inventory = {
        "schemaVersion": "1.0",
        "package": {},
        "files": [
            {"relativePath": "x", "sizeBytes": 1, "sha256": "x", "excluded": True,
             "exclusionReason": "font-file", "componentType": "unknown", "identity": None, "readError": None}
        ],
    }
    coverage = compute_coverage(inventory)
    assert coverage["candidateComponents"] == 0
    assert coverage["coveragePercent"] is None


def test_compute_coverage_full_resolution_is_100_percent():
    inventory = {
        "schemaVersion": "1.0",
        "package": {},
        "files": [
            {"relativePath": "a.jar", "sizeBytes": 1, "sha256": "x", "excluded": False, "exclusionReason": None,
             "componentType": "jar", "identity": {"method": "jar-pom-properties", "product": "a", "version": "1.0"},
             "readError": None}
        ],
    }
    coverage = compute_coverage(inventory)
    assert coverage["coveragePercent"] == 100.0
