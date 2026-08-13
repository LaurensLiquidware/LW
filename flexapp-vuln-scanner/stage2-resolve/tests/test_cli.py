from pathlib import Path
from unittest.mock import patch

from flexapp_vuln.cli import resolve_osv_matches
from flexapp_vuln.inventory import load_inventory

FIXTURE = Path(__file__).parent / "fixtures" / "sample.inventory.json"


def test_resolve_osv_matches_builds_purls_and_matches(tmp_path):
    inventory = load_inventory(FIXTURE)

    with patch("flexapp_vuln.cli.OSVClient") as mock_client_cls:
        mock_client = mock_client_cls.return_value
        mock_client.resolve.return_value = {
            "pkg:maven/com.acme/outer-app@9.9.9": [
                {"id": "GHSA-aaaa", "summary": "Something bad", "severity": [], "aliases": []}
            ]
        }

        result = resolve_osv_matches(inventory, cache_dir=tmp_path)

    # 3 non-excluded files in the fixture (kernel32.dll is excluded).
    assert len(result["components"]) == 3

    by_path = {c["relativePath"]: c for c in result["components"]}

    jar = by_path["Program Files\\App\\outer-app.jar"]
    assert jar["purl"] == "pkg:maven/com.acme/outer-app@9.9.9"
    assert jar["confidence"] == "exact-purl"
    assert jar["vulnerabilities"][0]["id"] == "GHSA-aaaa"

    # string-signature identity has no purl - correctly unresolved for OSV,
    # not silently dropped from the components list.
    native = by_path["Program Files\\App\\lib\\libcrypto-1_1.dll"]
    assert native["purl"] is None
    assert native["confidence"] is None
    assert native["vulnerabilities"] == []

    unresolved = by_path["Program Files\\App\\unresolved.bin"]
    assert unresolved["identity"] is None
    assert unresolved["purl"] is None


def test_resolve_osv_matches_with_no_purls_skips_osv_call(tmp_path):
    inventory = {
        "schemaVersion": "1.0",
        "package": {},
        "files": [
            {
                "relativePath": "a.bin",
                "sizeBytes": 1,
                "sha256": "x",
                "excluded": False,
                "exclusionReason": None,
                "componentType": "unknown",
                "identity": None,
                "readError": None,
            }
        ],
    }

    with patch("flexapp_vuln.cli.OSVClient") as mock_client_cls:
        result = resolve_osv_matches(inventory, cache_dir=tmp_path)
        mock_client_cls.return_value.resolve.assert_not_called()

    assert result["components"][0]["purl"] is None
