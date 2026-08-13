from pathlib import Path

from flexapp_vuln.cpe_mappings import CpeMappings
from flexapp_vuln.inventory import load_inventory
from flexapp_vuln.sbom import build_sbom

FIXTURE = Path(__file__).parent / "fixtures" / "sample.inventory.json"

_TEST_MAPPINGS = CpeMappings(mappings=[
    {
        "match": {"method": "string-signature", "product": "OpenSSL"},
        "cpe": {"vendor": "openssl", "product": "openssl"},
    }
])


def test_build_sbom_is_valid_cyclonedx_1_6_shape():
    inventory = load_inventory(FIXTURE)
    sbom = build_sbom(inventory, cpe_mappings=_TEST_MAPPINGS)

    assert sbom["bomFormat"] == "CycloneDX"
    assert sbom["specVersion"] == "1.6"
    assert sbom["serialNumber"].startswith("urn:uuid:")
    assert "timestamp" in sbom["metadata"]
    assert "component" in sbom["metadata"]


def test_build_sbom_includes_only_resolved_non_excluded_components():
    inventory = load_inventory(FIXTURE)
    sbom = build_sbom(inventory, cpe_mappings=_TEST_MAPPINGS)

    # Fixture has 2 resolved components (jar + string-signature); the
    # excluded kernel32.dll and the unresolved.bin never appear.
    assert len(sbom["components"]) == 2
    names = {c["name"] for c in sbom["components"]}
    assert names == {"outer-app", "OpenSSL"}


def test_build_sbom_jar_component_has_purl_and_hash():
    inventory = load_inventory(FIXTURE)
    sbom = build_sbom(inventory, cpe_mappings=_TEST_MAPPINGS)

    jar = next(c for c in sbom["components"] if c["name"] == "outer-app")
    assert jar["purl"] == "pkg:maven/com.acme/outer-app@9.9.9"
    assert "cpe" not in jar
    assert jar["hashes"][0]["alg"] == "SHA-256"
    assert jar["hashes"][0]["content"] == "bc70a2ea1dea659dd82d351ab4f0a9ef9d387ffd3b84491cb4d60cd8cc9bea36"


def test_build_sbom_native_component_has_cpe_not_purl():
    inventory = load_inventory(FIXTURE)
    sbom = build_sbom(inventory, cpe_mappings=_TEST_MAPPINGS)

    native = next(c for c in sbom["components"] if c["name"] == "OpenSSL")
    assert "purl" not in native
    assert native["cpe"] == "cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*"


def test_build_sbom_no_license_field_fabricated():
    inventory = load_inventory(FIXTURE)
    sbom = build_sbom(inventory, cpe_mappings=_TEST_MAPPINGS)
    for component in sbom["components"]:
        assert "licenses" not in component


def test_build_sbom_dedupes_identical_components():
    inventory = {
        "schemaVersion": "1.0",
        "package": {},
        "files": [
            {"relativePath": "a/lib.jar", "sizeBytes": 1, "sha256": "aaa", "excluded": False,
             "exclusionReason": None, "componentType": "jar",
             "identity": {"method": "jar-pom-properties", "product": "x", "version": "1.0",
                          "raw": {"groupId": "g", "artifactId": "x", "version": "1.0"}},
             "readError": None},
            {"relativePath": "b/lib-copy.jar", "sizeBytes": 1, "sha256": "bbb", "excluded": False,
             "exclusionReason": None, "componentType": "jar",
             "identity": {"method": "jar-pom-properties", "product": "x", "version": "1.0",
                          "raw": {"groupId": "g", "artifactId": "x", "version": "1.0"}},
             "readError": None},
        ],
    }
    sbom = build_sbom(inventory, cpe_mappings=CpeMappings(mappings=[]))
    assert len(sbom["components"]) == 1


def test_build_sbom_empty_inventory_has_empty_components():
    inventory = {"schemaVersion": "1.0", "package": {}, "files": []}
    sbom = build_sbom(inventory, cpe_mappings=CpeMappings(mappings=[]))
    assert sbom["components"] == []
