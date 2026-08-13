from unittest.mock import MagicMock

from flexapp_vuln.nvd_client import NVDClient
from flexapp_vuln.nvd_mirror import (
    NVDLocalMatcher,
    build_index,
    iter_all_cves,
    merge_index,
    parse_cpe23,
)


def _cve(cve_id, cpe_matches, descriptions=None):
    return {
        "id": cve_id,
        "descriptions": descriptions or [{"lang": "en", "value": f"{cve_id} summary"}],
        "metrics": {},
        "configurations": [
            {"nodes": [{"cpeMatch": cpe_matches}]}
        ],
    }


def test_parse_cpe23_splits_on_unescaped_colons():
    fields = parse_cpe23("cpe:2.3:a:vendor:product:1.0:*:*:*:*:*:*:*")
    assert fields[:6] == ["cpe", "2.3", "a", "vendor", "product", "1.0"]


def test_parse_cpe23_preserves_escaped_colon_within_field():
    fields = parse_cpe23(r"cpe:2.3:a:vendor:product:1.0\:beta:*:*:*:*:*:*")
    assert fields[5] == r"1.0\:beta"


def test_exact_version_match():
    cve = _cve("CVE-2024-0001", [{
        "criteria": "cpe:2.3:a:zlib:zlib:1.2.11:*:*:*:*:*:*:*",
        "vulnerable": True,
    }])
    matcher = NVDLocalMatcher(build_index(iter([cve])))

    result = matcher.query_cpe("cpe:2.3:a:zlib:zlib:1.2.11:*:*:*:*:*:*:*")
    cves = NVDClient.extract_cves(result)

    assert [c["id"] for c in cves] == ["CVE-2024-0001"]


def test_exact_version_mismatch_no_match():
    cve = _cve("CVE-2024-0001", [{
        "criteria": "cpe:2.3:a:zlib:zlib:1.2.11:*:*:*:*:*:*:*",
        "vulnerable": True,
    }])
    matcher = NVDLocalMatcher(build_index(iter([cve])))

    result = matcher.query_cpe("cpe:2.3:a:zlib:zlib:1.2.13:*:*:*:*:*:*:*")

    assert result == {"vulnerabilities": []}


def test_version_range_match():
    cve = _cve("CVE-2024-0002", [{
        "criteria": "cpe:2.3:a:openssl:openssl:*:*:*:*:*:*:*:*",
        "versionStartIncluding": "1.1.0",
        "versionEndExcluding": "1.1.1w",
        "vulnerable": True,
    }])
    matcher = NVDLocalMatcher(build_index(iter([cve])))

    in_range = NVDClient.extract_cves(matcher.query_cpe("cpe:2.3:a:openssl:openssl:1.1.1a:*:*:*:*:*:*:*"))
    out_of_range = NVDClient.extract_cves(matcher.query_cpe("cpe:2.3:a:openssl:openssl:1.0.0:*:*:*:*:*:*:*"))

    assert [c["id"] for c in in_range] == ["CVE-2024-0002"]
    assert out_of_range == []


def test_non_vulnerable_entry_is_excluded():
    cve = _cve("CVE-2024-0003", [{
        "criteria": "cpe:2.3:a:vendor:product:1.0:*:*:*:*:*:*:*",
        "vulnerable": False,
    }])
    matcher = NVDLocalMatcher(build_index(iter([cve])))

    result = matcher.query_cpe("cpe:2.3:a:vendor:product:1.0:*:*:*:*:*:*:*")

    assert result == {"vulnerabilities": []}


def test_wildcard_version_matches_any_version():
    cve = _cve("CVE-2024-0004", [{
        "criteria": "cpe:2.3:a:vendor:product:*:*:*:*:*:*:*:*",
        "version": "*",
        "vulnerable": True,
    }])
    matcher = NVDLocalMatcher(build_index(iter([cve])))

    result = NVDClient.extract_cves(matcher.query_cpe("cpe:2.3:a:vendor:product:9.9.9:*:*:*:*:*:*:*"))

    assert [c["id"] for c in result] == ["CVE-2024-0004"]


def test_different_vendor_product_no_match():
    cve = _cve("CVE-2024-0005", [{
        "criteria": "cpe:2.3:a:vendor:product:1.0:*:*:*:*:*:*:*",
        "vulnerable": True,
    }])
    matcher = NVDLocalMatcher(build_index(iter([cve])))

    result = matcher.query_cpe("cpe:2.3:a:othervendor:product:1.0:*:*:*:*:*:*:*")

    assert result == {"vulnerabilities": []}


def test_merge_index_drops_stale_entries_for_refetched_cve():
    old_cve = _cve("CVE-2024-0006", [{
        "criteria": "cpe:2.3:a:vendor:product:1.0:*:*:*:*:*:*:*",
        "vulnerable": True,
    }])
    old_index = build_index(iter([old_cve]))

    # Same CVE refetched with a corrected/expanded version range.
    updated_cve = _cve("CVE-2024-0006", [{
        "criteria": "cpe:2.3:a:vendor:product:2.0:*:*:*:*:*:*:*",
        "vulnerable": True,
    }])
    new_index = build_index(iter([updated_cve]))

    merged = merge_index(old_index, new_index, updated_cve_ids={"CVE-2024-0006"})
    matcher = NVDLocalMatcher(merged)

    stale = matcher.query_cpe("cpe:2.3:a:vendor:product:1.0:*:*:*:*:*:*:*")
    fresh = NVDClient.extract_cves(matcher.query_cpe("cpe:2.3:a:vendor:product:2.0:*:*:*:*:*:*:*"))

    assert stale == {"vulnerabilities": []}
    assert [c["id"] for c in fresh] == ["CVE-2024-0006"]


def test_merge_index_keeps_untouched_entries():
    untouched_cve = _cve("CVE-2024-0007", [{
        "criteria": "cpe:2.3:a:vendor:other-product:1.0:*:*:*:*:*:*:*",
        "vulnerable": True,
    }])
    old_index = build_index(iter([untouched_cve]))
    new_index = build_index(iter([]))

    merged = merge_index(old_index, new_index, updated_cve_ids={"CVE-2024-9999"})
    matcher = NVDLocalMatcher(merged)

    result = NVDClient.extract_cves(matcher.query_cpe("cpe:2.3:a:vendor:other-product:1.0:*:*:*:*:*:*:*"))
    assert [c["id"] for c in result] == ["CVE-2024-0007"]


def _page_response(vulnerabilities, total_results):
    resp = MagicMock()
    resp.status_code = 200
    resp.json.return_value = {"vulnerabilities": vulnerabilities, "totalResults": total_results}
    return resp


def test_iter_all_cves_paginates():
    session = MagicMock()
    page1 = _page_response([{"cve": _cve("CVE-2024-0001", [])}, {"cve": _cve("CVE-2024-0002", [])}], total_results=3)
    page2 = _page_response([{"cve": _cve("CVE-2024-0003", [])}], total_results=3)
    session.get.side_effect = [page1, page2]

    results = list(iter_all_cves(session=session, sleep_fn=lambda s: None))

    assert [cve["id"] for cve in results] == ["CVE-2024-0001", "CVE-2024-0002", "CVE-2024-0003"]
    assert session.get.call_count == 2


def test_iter_all_cves_retries_on_429():
    session = MagicMock()
    rate_limited = MagicMock()
    rate_limited.status_code = 429
    rate_limited.headers = {}
    ok = _page_response([{"cve": _cve("CVE-2024-0001", [])}], total_results=1)
    session.get.side_effect = [rate_limited, ok]

    results = list(iter_all_cves(session=session, sleep_fn=lambda s: None))

    assert [cve["id"] for cve in results] == ["CVE-2024-0001"]
    assert session.get.call_count == 2
