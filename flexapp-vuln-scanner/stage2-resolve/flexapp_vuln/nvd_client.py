"""NVD 2.0 API client: CPE-based CVE lookup, rate-limit aware, on-disk cache.

Per PLAN.md: 5 requests per 30 seconds without an API key, 50 with one
(NVD_API_KEY env var). Every response is cached to disk and a cached CPE is
never re-queried.

NOTE: services.nvd.nist.gov is blocked by this development environment's
network egress policy (confirmed via the proxy status endpoint - same
category as the api.osv.dev and grype.anchore.io blocks hit elsewhere in
this project). This client is written against NVD's documented 2.0 API and
validated with mocked-HTTP unit tests (see tests/test_nvd_client.py), not a
live call. Live end-to-end validation still needs an environment where
services.nvd.nist.gov is reachable.
"""

from __future__ import annotations

import collections
import hashlib
import json
import logging
import os
import time
from pathlib import Path
from typing import Any

import requests

logger = logging.getLogger(__name__)

_DEFAULT_BASE_URL = "https://services.nvd.nist.gov/rest/json/cves/2.0"
_NO_KEY_LIMIT = 5
_WITH_KEY_LIMIT = 50
_WINDOW_SECONDS = 30.0


def _cache_key(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


class NVDClient:
    def __init__(
        self,
        cache_dir: Path | str,
        api_key: str | None = None,
        base_url: str = _DEFAULT_BASE_URL,
        session: requests.Session | None = None,
        timeout: float = 30.0,
        # Injectable for deterministic tests - real callers never pass these.
        sleep_fn=time.sleep,
        time_fn=time.monotonic,
    ) -> None:
        self.cache_dir = Path(cache_dir) / "nvd-cpe"
        self.cache_dir.mkdir(parents=True, exist_ok=True)

        self.api_key = api_key if api_key is not None else os.environ.get("NVD_API_KEY")
        self.base_url = base_url
        self.session = session or requests.Session()
        self.timeout = timeout
        self._sleep = sleep_fn
        self._now = time_fn

        self._limit = _WITH_KEY_LIMIT if self.api_key else _NO_KEY_LIMIT
        self._request_times: collections.deque[float] = collections.deque(maxlen=self._limit)

    def _read_cache(self, cpe23: str) -> Any | None:
        path = self.cache_dir / f"{_cache_key(cpe23)}.json"
        if not path.exists():
            return None
        with path.open("r", encoding="utf-8") as f:
            return json.load(f)

    def _write_cache(self, cpe23: str, value: Any) -> None:
        path = self.cache_dir / f"{_cache_key(cpe23)}.json"
        with path.open("w", encoding="utf-8") as f:
            json.dump(value, f)

    def _throttle(self) -> None:
        """Blocks (via self._sleep) until making another request stays
        within the sliding rate-limit window. Cached hits never call this.
        """
        now = self._now()
        while self._request_times and now - self._request_times[0] > _WINDOW_SECONDS:
            self._request_times.popleft()

        if len(self._request_times) >= self._limit:
            wait = _WINDOW_SECONDS - (now - self._request_times[0])
            if wait > 0:
                self._sleep(wait)

        self._request_times.append(self._now())

    def query_cpe(self, cpe23: str) -> dict[str, Any]:
        """Returns the raw NVD 2.0 API response for a CPE 2.3 string.
        Cached forever once fetched - a cache hit never touches the network
        or the rate limiter.
        """
        cached = self._read_cache(cpe23)
        if cached is not None:
            return cached

        self._throttle()

        headers = {"apiKey": self.api_key} if self.api_key else {}
        response = self.session.get(
            self.base_url,
            params={"cpeName": cpe23},
            headers=headers,
            timeout=self.timeout,
        )
        if response.status_code == 404:
            # Documented NVD 2.0 API behavior: a cpeName with no matching
            # entry in NVD's CPE dictionary returns 404, not an empty 200
            # result. That's a real "no CVEs known for this CPE" answer, not
            # a connectivity failure - must not be conflated with the
            # RequestException handling in cli.py that reports unreachable
            # hosts, or every non-matching CPE would wrongly abort the run.
            data: dict[str, Any] = {"vulnerabilities": []}
            self._write_cache(cpe23, data)
            return data
        response.raise_for_status()
        data = response.json()
        self._write_cache(cpe23, data)
        return data

    @staticmethod
    def extract_cves(nvd_response: dict[str, Any]) -> list[dict[str, Any]]:
        """Flattens an NVD 2.0 response into a simple list of
        {id, summary, severity} dicts, matching the shape used for OSV
        results so downstream reporting can treat both sources uniformly.
        """
        out = []
        for item in nvd_response.get("vulnerabilities", []):
            cve = item.get("cve", {})
            descriptions = cve.get("descriptions", [])
            summary = next((d["value"] for d in descriptions if d.get("lang") == "en"), None)

            severities = []
            metrics = cve.get("metrics", {})
            for metric_key in ("cvssMetricV31", "cvssMetricV30", "cvssMetricV2"):
                for metric in metrics.get(metric_key, []):
                    cvss_data = metric.get("cvssData", {})
                    severities.append({
                        "source": metric_key,
                        "baseScore": cvss_data.get("baseScore"),
                        "baseSeverity": cvss_data.get("baseSeverity") or metric.get("baseSeverity"),
                    })

            severity_level = next((s["baseSeverity"] for s in severities if s.get("baseSeverity")), None)

            out.append({
                "id": cve.get("id"),
                "summary": summary,
                "severity": severities,
                "severityLevel": severity_level.upper() if severity_level else None,
            })
        return out
