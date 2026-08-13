from pathlib import Path
from unittest.mock import patch

from flexapp_vuln.cli import resolve_vuln_matches
from flexapp_vuln.cpe_mappings import CpeMappings
from flexapp_vuln.inventory import load_inventory

FIXTURE = Path(__file__).parent / "fixtures" / "sample.inventory.json"

_TEST_MAPPINGS = CpeMappings(mappings=[
    {
        "match": {"method": "string-signature", "product": "OpenSSL"},
        "cpe": {"vendor": "openssl", "product": "openssl"},
    }
])


def test_resolve_vuln_matches_builds_purls_and_matches(tmp_path):
    inventory = load_inventory(FIXTURE)

    with patch("flexapp_vuln.cli.OSVClient") as mock_osv_cls, patch("flexapp_vuln.cli.NVDClient") as mock_nvd_cls:
        mock_osv_cls.return_value.resolve.return_value = {
            "pkg:maven/com.acme/outer-app@9.9.9": [
                {"id": "GHSA-aaaa", "summary": "Something bad", "severity": []}
            ]
        }
        mock_nvd_cls.return_value.query_cpe.return_value = {"fake": "response"}
        mock_nvd_cls.extract_cves.return_value = [
            {"id": "CVE-2023-0001", "summary": "OpenSSL issue", "severity": []}
        ]

        result = resolve_vuln_matches(inventory, cache_dir=tmp_path, cpe_mappings=_TEST_MAPPINGS)

    # 3 non-excluded files in the fixture (kernel32.dll is excluded).
    assert len(result["components"]) == 3
    by_path = {c["relativePath"]: c for c in result["components"]}

    jar = by_path["Program Files\\App\\outer-app.jar"]
    assert jar["purl"] == "pkg:maven/com.acme/outer-app@9.9.9"
    assert jar["cpe"] is None
    assert jar["confidence"] == "exact-purl"
    assert jar["vulnerabilities"][0] == {
        "id": "GHSA-aaaa", "summary": "Something bad", "severity": [], "source": "osv",
    }

    # OpenSSL string-signature identity: no purl, but a mapped CPE via the
    # curated override table above.
    native = by_path["Program Files\\App\\lib\\libcrypto-1_1.dll"]
    assert native["purl"] is None
    assert native["cpe"] == "cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*"
    assert native["confidence"] == "mapped-cpe"
    assert native["vulnerabilities"][0] == {
        "id": "CVE-2023-0001", "summary": "OpenSSL issue", "severity": [], "source": "nvd",
    }

    unresolved = by_path["Program Files\\App\\unresolved.bin"]
    assert unresolved["identity"] is None
    assert unresolved["purl"] is None
    assert unresolved["cpe"] is None
    assert unresolved["confidence"] is None
    assert unresolved["vulnerabilities"] == []


def test_resolve_vuln_matches_with_nothing_expressible_skips_both_clients(tmp_path):
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

    with patch("flexapp_vuln.cli.OSVClient") as mock_osv_cls, patch("flexapp_vuln.cli.NVDClient") as mock_nvd_cls:
        result = resolve_vuln_matches(inventory, cache_dir=tmp_path, cpe_mappings=CpeMappings(mappings=[]))
        mock_osv_cls.return_value.resolve.assert_not_called()
        mock_nvd_cls.return_value.query_cpe.assert_not_called()

    assert result["components"][0]["purl"] is None
    assert result["components"][0]["cpe"] is None


def test_resolve_vuln_matches_heuristic_cpe_when_no_mapping(tmp_path):
    inventory = {
        "schemaVersion": "1.0",
        "package": {},
        "files": [
            {
                "relativePath": "AcmeWidget.exe",
                "sizeBytes": 1,
                "sha256": "x",
                "excluded": False,
                "exclusionReason": None,
                "componentType": "pe-native",
                "identity": {
                    "method": "pe-version-resource",
                    "vendor": "Acme Corporation",
                    "product": "Acme Widget",
                    "version": "1.2.3",
                    "raw": {},
                },
                "readError": None,
            }
        ],
    }

    with patch("flexapp_vuln.cli.OSVClient") as mock_osv_cls, patch("flexapp_vuln.cli.NVDClient") as mock_nvd_cls:
        mock_nvd_cls.return_value.query_cpe.return_value = {"vulnerabilities": []}
        mock_nvd_cls.extract_cves.return_value = []

        result = resolve_vuln_matches(inventory, cache_dir=tmp_path, cpe_mappings=CpeMappings(mappings=[]))
        mock_osv_cls.return_value.resolve.assert_not_called()

    component = result["components"][0]
    assert component["cpe"] == "cpe:2.3:a:acme:acme_widget:1.2.3:*:*:*:*:*:*:*"
    assert component["confidence"] == "heuristic"
