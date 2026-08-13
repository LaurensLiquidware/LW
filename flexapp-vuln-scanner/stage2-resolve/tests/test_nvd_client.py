from unittest.mock import MagicMock

import pytest

from flexapp_vuln.nvd_client import NVDClient


def _mock_response(json_data):
    resp = MagicMock()
    resp.status_code = 200
    resp.json.return_value = json_data
    resp.raise_for_status.side_effect = None
    return resp


class FakeClock:
    """Deterministic, manually-advanced clock + no-op sleep that advances
    the clock by the requested amount - lets rate-limit tests run instantly
    instead of waiting real wall-clock seconds.
    """

    def __init__(self):
        self.t = 0.0

    def now(self):
        return self.t

    def sleep(self, seconds):
        self.t += seconds


@pytest.fixture
def clock():
    return FakeClock()


@pytest.fixture
def session():
    return MagicMock()


def make_client(tmp_path, session, clock, api_key=None):
    return NVDClient(
        cache_dir=tmp_path / "cache",
        api_key=api_key,
        session=session,
        sleep_fn=clock.sleep,
        time_fn=clock.now,
    )


def test_query_cpe_caches(tmp_path, session, clock):
    client = make_client(tmp_path, session, clock)
    session.get.return_value = _mock_response({"vulnerabilities": []})

    first = client.query_cpe("cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*")
    second = client.query_cpe("cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*")

    assert first == second == {"vulnerabilities": []}
    assert session.get.call_count == 1


def test_query_cpe_404_returns_empty_result_not_an_error(tmp_path, session, clock):
    # Documented NVD 2.0 API behavior: a syntactically valid CPE with no
    # matching dictionary entry returns 404, not an empty 200 - this must be
    # treated as "no CVEs known", not raised as a connectivity failure (see
    # cli.py's UnreachableService, which only wraps RequestException).
    client = make_client(tmp_path, session, clock)
    resp = MagicMock()
    resp.status_code = 404
    session.get.return_value = resp

    result = client.query_cpe("cpe:2.3:a:vendor:nonexistent-product:1.0:*:*:*:*:*:*:*")

    assert result == {"vulnerabilities": []}
    resp.raise_for_status.assert_not_called()


def test_query_cpe_404_is_cached(tmp_path, session, clock):
    client = make_client(tmp_path, session, clock)
    resp = MagicMock()
    resp.status_code = 404
    session.get.return_value = resp

    client.query_cpe("cpe:2.3:a:vendor:nonexistent-product:1.0:*:*:*:*:*:*:*")
    client.query_cpe("cpe:2.3:a:vendor:nonexistent-product:1.0:*:*:*:*:*:*:*")

    assert session.get.call_count == 1


def test_no_api_key_sends_no_header(tmp_path, session, clock):
    client = make_client(tmp_path, session, clock, api_key=None)
    session.get.return_value = _mock_response({"vulnerabilities": []})

    client.query_cpe("cpe:2.3:a:zlib:zlib:1.3:*:*:*:*:*:*:*")

    _, kwargs = session.get.call_args
    assert kwargs["headers"] == {}


def test_api_key_sent_as_header(tmp_path, session, clock):
    client = make_client(tmp_path, session, clock, api_key="secret-key")
    session.get.return_value = _mock_response({"vulnerabilities": []})

    client.query_cpe("cpe:2.3:a:zlib:zlib:1.3:*:*:*:*:*:*:*")

    _, kwargs = session.get.call_args
    assert kwargs["headers"] == {"apiKey": "secret-key"}


def test_rate_limit_without_key_throttles_after_five_requests(tmp_path, session, clock):
    client = make_client(tmp_path, session, clock, api_key=None)
    session.get.return_value = _mock_response({"vulnerabilities": []})

    for i in range(5):
        client.query_cpe(f"cpe:2.3:a:vendor:product{i}:1.0:*:*:*:*:*:*:*")
    assert clock.t == 0.0  # first 5 requests are free within the window

    client.query_cpe("cpe:2.3:a:vendor:product5:1.0:*:*:*:*:*:*:*")
    assert clock.t > 0.0  # the 6th had to wait out the 30s window


def test_rate_limit_with_key_allows_fifty(tmp_path, session, clock):
    client = make_client(tmp_path, session, clock, api_key="secret-key")
    session.get.return_value = _mock_response({"vulnerabilities": []})

    for i in range(50):
        client.query_cpe(f"cpe:2.3:a:vendor:product{i}:1.0:*:*:*:*:*:*:*")
    assert clock.t == 0.0

    client.query_cpe("cpe:2.3:a:vendor:product50:1.0:*:*:*:*:*:*:*")
    assert clock.t > 0.0


def test_cached_hits_never_throttle(tmp_path, session, clock):
    client = make_client(tmp_path, session, clock, api_key=None)
    session.get.return_value = _mock_response({"vulnerabilities": []})

    for i in range(5):
        client.query_cpe(f"cpe:2.3:a:vendor:product{i}:1.0:*:*:*:*:*:*:*")

    # Re-querying the same 5 (now cached) CPEs must never sleep, no matter
    # how many times - cache hits bypass the rate limiter entirely.
    for _ in range(20):
        for i in range(5):
            client.query_cpe(f"cpe:2.3:a:vendor:product{i}:1.0:*:*:*:*:*:*:*")
    assert clock.t == 0.0


def test_extract_cves_flattens_response():
    response = {
        "vulnerabilities": [
            {
                "cve": {
                    "id": "CVE-2023-0001",
                    "descriptions": [
                        {"lang": "es", "value": "Descripcion"},
                        {"lang": "en", "value": "English summary"},
                    ],
                    "metrics": {
                        "cvssMetricV31": [
                            {"cvssData": {"baseScore": 9.8, "baseSeverity": "CRITICAL"}}
                        ]
                    },
                }
            }
        ]
    }

    result = NVDClient.extract_cves(response)

    assert result == [{
        "id": "CVE-2023-0001",
        "summary": "English summary",
        "severity": [{"source": "cvssMetricV31", "baseScore": 9.8, "baseSeverity": "CRITICAL"}],
        "severityLevel": "CRITICAL",
    }]


def test_extract_cves_empty_response():
    assert NVDClient.extract_cves({"vulnerabilities": []}) == []
    assert NVDClient.extract_cves({}) == []
