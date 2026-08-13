from flexapp_vuln.cpe_mappings import CpeMappings
from flexapp_vuln.normalize import build_cpe_candidate, build_purl


def test_maven_pom_properties():
    identity = {
        "method": "jar-pom-properties",
        "vendor": "com.acme",
        "product": "outer-app",
        "version": "9.9.9",
        "raw": {"groupId": "com.acme", "artifactId": "outer-app", "version": "9.9.9"},
    }
    assert build_purl(identity) == "pkg:maven/com.acme/outer-app@9.9.9"


def test_npm_unscoped():
    identity = {"method": "node-package-json", "product": "lodash", "version": "4.17.21"}
    assert build_purl(identity) == "pkg:npm/lodash@4.17.21"


def test_npm_scoped():
    identity = {"method": "node-package-json", "product": "@angular/core", "version": "17.0.0"}
    assert build_purl(identity) == "pkg:npm/%40angular/core@17.0.0"


def test_pypi_name_normalization():
    identity = {"method": "python-dist-info", "product": "Requests_Toolbelt", "version": "1.0.0"}
    assert build_purl(identity) == "pkg:pypi/requests-toolbelt@1.0.0"


def test_jar_manifest_has_no_purl():
    # No groupId available from MANIFEST.MF alone - correctly returns None
    # rather than guessing one.
    identity = {"method": "jar-manifest", "product": "legacy-widget", "version": "4.5.6", "raw": {}}
    assert build_purl(identity) is None


def test_native_pe_has_no_purl():
    identity = {"method": "pe-version-resource", "vendor": "Acme", "product": "Acme Widget", "version": "1.0.0"}
    assert build_purl(identity) is None


def test_string_signature_has_no_purl():
    identity = {"method": "string-signature", "product": "OpenSSL", "version": "1.1.1w"}
    assert build_purl(identity) is None


def test_none_identity():
    assert build_purl(None) is None


def test_missing_version_or_product():
    assert build_purl({"method": "node-package-json", "product": "x", "version": None}) is None
    assert build_purl({"method": "node-package-json", "product": None, "version": "1.0.0"}) is None


# -- build_cpe_candidate --------------------------------------------------

_EMPTY_MAPPINGS = CpeMappings(mappings=[])
_OPENSSL_MAPPINGS = CpeMappings(mappings=[
    {
        "match": {"method": "string-signature", "product": "OpenSSL"},
        "cpe": {"vendor": "openssl", "product": "openssl"},
    }
])


def test_cpe_mapped_override_is_mapped_cpe_confidence():
    identity = {"method": "string-signature", "product": "OpenSSL", "version": "1.1.1w"}
    cpe, confidence = build_cpe_candidate(identity, _OPENSSL_MAPPINGS)
    assert cpe == "cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*"
    assert confidence == "mapped-cpe"


def test_cpe_heuristic_fallback_strips_corp_suffix_and_confidence_is_heuristic():
    identity = {
        "method": "pe-version-resource",
        "vendor": "Acme Corporation",
        "product": "Acme Widget",
        "version": "1.2.3",
    }
    cpe, confidence = build_cpe_candidate(identity, _EMPTY_MAPPINGS)
    assert cpe == "cpe:2.3:a:acme:acme_widget:1.2.3:*:*:*:*:*:*:*"
    assert confidence == "heuristic"


def test_cpe_purl_expressible_methods_return_none():
    identity = {"method": "jar-pom-properties", "product": "outer-app", "version": "9.9.9"}
    assert build_cpe_candidate(identity, _EMPTY_MAPPINGS) == (None, None)


def test_cpe_no_version_returns_none():
    identity = {"method": "string-signature", "product": "OpenSSL", "version": None}
    assert build_cpe_candidate(identity, _EMPTY_MAPPINGS) == (None, None)


def test_cpe_none_identity_returns_none():
    assert build_cpe_candidate(None, _EMPTY_MAPPINGS) == (None, None)


def test_cpe_escapes_colon_in_normalized_component():
    # Heuristic normalization strips non-alnum to underscores, so a raw
    # colon can't actually reach the CPE string via that path - this
    # confirms the escaper itself behaves if ever fed one directly (e.g. a
    # mapped override authored with one by mistake).
    from flexapp_vuln.normalize import _escape_cpe_component

    assert _escape_cpe_component("foo:bar") == "foo\\:bar"


def test_cpe_version_with_raw_space_and_colon_is_escaped():
    # Found live: a real Win32 ProductVersion string
    # ("2.1.23296 git hash: e323abb5b08e") reached the CPE unescaped,
    # producing an invalid CPE 2.3 formatted string. `version` is kept
    # verbatim (not heuristic-normalized like vendor/product), so the
    # escaper itself must handle spaces and colons directly.
    identity = {
        "method": "pe-version-resource",
        "vendor": "Google LLC",
        "product": "ANGLE libEGL Dynamic Link Library",
        "version": "2.1.23296 git hash: e323abb5b08e",
    }
    cpe, confidence = build_cpe_candidate(identity, _EMPTY_MAPPINGS)
    assert cpe == (
        "cpe:2.3:a:google:angle_libegl_dynamic_link_library:"
        "2.1.23296\\ git\\ hash\\:\\ e323abb5b08e:*:*:*:*:*:*:*"
    )
    assert confidence == "heuristic"
