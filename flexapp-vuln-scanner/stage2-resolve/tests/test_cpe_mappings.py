from flexapp_vuln.cpe_mappings import CpeMappings


def test_find_exact_match():
    mappings = CpeMappings(mappings=[
        {
            "match": {"method": "string-signature", "product": "OpenSSL"},
            "cpe": {"vendor": "openssl", "product": "openssl"},
        }
    ])
    identity = {"method": "string-signature", "product": "OpenSSL", "version": "1.1.1w"}
    assert mappings.find(identity) == ("openssl", "openssl")


def test_find_case_insensitive():
    mappings = CpeMappings(mappings=[
        {
            "match": {"method": "string-signature", "product": "openssl"},
            "cpe": {"vendor": "openssl", "product": "openssl"},
        }
    ])
    identity = {"method": "string-signature", "product": "OpenSSL", "version": "1.1.1w"}
    assert mappings.find(identity) == ("openssl", "openssl")


def test_find_no_match_returns_none():
    mappings = CpeMappings(mappings=[
        {
            "match": {"method": "string-signature", "product": "OpenSSL"},
            "cpe": {"vendor": "openssl", "product": "openssl"},
        }
    ])
    identity = {"method": "string-signature", "product": "zlib", "version": "1.3"}
    assert mappings.find(identity) is None


def test_find_method_mismatch():
    mappings = CpeMappings(mappings=[
        {
            "match": {"method": "electron-embedded", "product": "OpenSSL"},
            "cpe": {"vendor": "openssl", "product": "openssl"},
        }
    ])
    identity = {"method": "string-signature", "product": "OpenSSL", "version": "1.1.1w"}
    assert mappings.find(identity) is None


def test_find_matches_regardless_of_method_when_unscoped():
    # Found live: a real "zlib" Win32 version resource (method
    # pe-version-resource) should still hit a mapping written without a
    # method constraint, the same way a string-signature-sourced "zlib"
    # would.
    mappings = CpeMappings(mappings=[
        {
            "match": {"product": "zlib"},
            "cpe": {"vendor": "zlib", "product": "zlib"},
        }
    ])
    for method in ("string-signature", "pe-version-resource", "dotnet-manifest"):
        identity = {"method": method, "product": "zlib", "version": "1.3.1"}
        assert mappings.find(identity) == ("zlib", "zlib")


def test_find_vendor_only_match():
    mappings = CpeMappings(mappings=[
        {
            "match": {"method": "pe-version-resource", "vendor": "Google Inc."},
            "cpe": {"vendor": "google", "product": "chrome"},
        }
    ])
    identity = {"method": "pe-version-resource", "vendor": "Google Inc.", "product": "Google Chrome", "version": "120.0"}
    assert mappings.find(identity) == ("google", "chrome")


def test_none_identity():
    assert CpeMappings(mappings=[]).find(None) is None


def test_load_real_config_file_parses():
    mappings = CpeMappings.load()
    assert len(mappings._mappings) > 0
    identity = {"method": "string-signature", "product": "OpenSSL", "version": "1.1.1w"}
    assert mappings.find(identity) == ("openssl", "openssl")


def test_find_version_transform_returns_pattern_and_group():
    mappings = CpeMappings(mappings=[
        {
            "match": {"product": "FFmpeg"},
            "cpe": {"vendor": "ffmpeg", "product": "ffmpeg", "versionPattern": r"^n?(\d+\.\d+\.\d+)", "versionGroup": 1},
        }
    ])
    identity = {"method": "pe-version-resource", "product": "FFmpeg", "version": "n7.1.1"}
    assert mappings.find_version_transform(identity) == (r"^n?(\d+\.\d+\.\d+)", 1)


def test_find_version_transform_none_when_entry_has_no_pattern():
    mappings = CpeMappings(mappings=[
        {"match": {"product": "OpenSSL"}, "cpe": {"vendor": "openssl", "product": "openssl"}},
    ])
    identity = {"method": "string-signature", "product": "OpenSSL", "version": "1.1.1w"}
    assert mappings.find_version_transform(identity) is None


def test_find_version_transform_none_when_nothing_matches():
    mappings = CpeMappings(mappings=[
        {"match": {"product": "OpenSSL"}, "cpe": {"vendor": "openssl", "product": "openssl", "versionPattern": r"(\d+)"}},
    ])
    identity = {"method": "string-signature", "product": "zlib", "version": "1.3"}
    assert mappings.find_version_transform(identity) is None


def test_load_missing_file_returns_empty():
    mappings = CpeMappings.load("/nonexistent/path.yaml")
    assert mappings.find({"method": "string-signature", "product": "OpenSSL"}) is None
