import json
from unittest.mock import MagicMock

import pytest

from flexapp_vuln.osv_client import OSVClient


def _mock_response(json_data, status_code=200):
    resp = MagicMock()
    resp.status_code = status_code
    resp.json.return_value = json_data
    resp.raise_for_status.side_effect = None
    return resp


@pytest.fixture
def session():
    return MagicMock()


@pytest.fixture
def client(tmp_path, session):
    return OSVClient(cache_dir=tmp_path / "cache", session=session, batch_size=2)


def test_query_batch_single_call(client, session):
    session.post.return_value = _mock_response({
        "results": [
            {"vulns": [{"id": "GHSA-aaaa", "modified": "2024-01-01"}]},
            {"vulns": []},
        ]
    })

    result = client.query_batch(["pkg:npm/lodash@4.17.15", "pkg:npm/left-pad@1.3.0"])

    assert result == {
        "pkg:npm/lodash@4.17.15": ["GHSA-aaaa"],
        "pkg:npm/left-pad@1.3.0": [],
    }
    assert session.post.call_count == 1


def test_query_batch_caches_and_skips_network_on_second_call(client, session):
    session.post.return_value = _mock_response({
        "results": [{"vulns": [{"id": "GHSA-aaaa"}]}],
    })

    first = client.query_batch(["pkg:npm/lodash@4.17.15"])
    second = client.query_batch(["pkg:npm/lodash@4.17.15"])

    assert first == second == {"pkg:npm/lodash@4.17.15": ["GHSA-aaaa"]}
    assert session.post.call_count == 1  # second call was served entirely from cache


def test_query_batch_splits_by_batch_size(client, session):
    # batch_size=2 (see fixture), 3 purls -> 2 HTTP calls (2 + 1)
    session.post.side_effect = [
        _mock_response({"results": [{"vulns": []}, {"vulns": []}]}),
        _mock_response({"results": [{"vulns": []}]}),
    ]

    result = client.query_batch(["pkg:npm/a@1", "pkg:npm/b@1", "pkg:npm/c@1"])

    assert set(result.keys()) == {"pkg:npm/a@1", "pkg:npm/b@1", "pkg:npm/c@1"}
    assert session.post.call_count == 2


def test_query_batch_mixed_cache_hit_and_miss(client, session):
    session.post.return_value = _mock_response({"results": [{"vulns": [{"id": "GHSA-aaaa"}]}]})
    client.query_batch(["pkg:npm/lodash@4.17.15"])
    assert session.post.call_count == 1

    session.post.return_value = _mock_response({"results": [{"vulns": []}]})
    result = client.query_batch(["pkg:npm/lodash@4.17.15", "pkg:npm/new-pkg@1.0.0"])

    # Only the uncached purl should have triggered a second network call.
    assert session.post.call_count == 2
    assert result["pkg:npm/lodash@4.17.15"] == ["GHSA-aaaa"]
    assert result["pkg:npm/new-pkg@1.0.0"] == []


def test_get_vulnerability_caches(client, session):
    session.get.return_value = _mock_response({"id": "GHSA-aaaa", "summary": "A bad thing"})

    first = client.get_vulnerability("GHSA-aaaa")
    second = client.get_vulnerability("GHSA-aaaa")

    assert first == second == {"id": "GHSA-aaaa", "summary": "A bad thing"}
    assert session.get.call_count == 1


def test_resolve_combines_batch_and_detail_lookup(client, session):
    session.post.return_value = _mock_response({
        "results": [{"vulns": [{"id": "GHSA-aaaa"}]}],
    })
    session.get.return_value = _mock_response({"id": "GHSA-aaaa", "summary": "A bad thing", "severity": []})

    result = client.resolve(["pkg:npm/lodash@4.17.15"])

    assert result == {
        "pkg:npm/lodash@4.17.15": [{"id": "GHSA-aaaa", "summary": "A bad thing", "severity": []}]
    }


def test_resolve_empty_purl_list(client, session):
    assert client.resolve([]) == {}
    session.post.assert_not_called()
    session.get.assert_not_called()


def test_resolve_reports_progress_per_vuln_id(client, session):
    session.post.return_value = _mock_response({
        "results": [{"vulns": [{"id": "GHSA-aaaa"}, {"id": "GHSA-bbbb"}]}],
    })
    session.get.return_value = _mock_response({"id": "x", "summary": "s", "severity": []})

    calls = []
    client.resolve(["pkg:npm/lodash@4.17.15"], on_progress=lambda done, total: calls.append((done, total)))

    assert calls == [(1, 2), (2, 2)]


def test_resolve_empty_purl_list_never_calls_on_progress(client, session):
    calls = []
    client.resolve([], on_progress=lambda done, total: calls.append((done, total)))
    assert calls == []


def test_cache_persists_across_client_instances(tmp_path, session):
    cache_dir = tmp_path / "cache"
    client1 = OSVClient(cache_dir=cache_dir, session=session)
    session.post.return_value = _mock_response({"results": [{"vulns": [{"id": "GHSA-aaaa"}]}]})
    client1.query_batch(["pkg:npm/lodash@4.17.15"])

    fresh_session = MagicMock()
    client2 = OSVClient(cache_dir=cache_dir, session=fresh_session)
    result = client2.query_batch(["pkg:npm/lodash@4.17.15"])

    assert result == {"pkg:npm/lodash@4.17.15": ["GHSA-aaaa"]}
    fresh_session.post.assert_not_called()
