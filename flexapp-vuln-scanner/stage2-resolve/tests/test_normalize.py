from flexapp_vuln.normalize import build_purl


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
