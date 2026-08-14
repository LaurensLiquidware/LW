"""OSV.dev client: batch purl -> vuln ID lookup, then per-ID detail fetch.

Per PLAN.md: OSV.dev needs no API key and matching against a purl is exact,
not fuzzy - this is expected to be the reliable path. All responses are
cached to disk and never re-queried once cached (same policy PLAN.md
specifies for NVD, applied consistently here too).

NOTE: api.osv.dev is blocked by this development environment's network
egress policy (confirmed via the proxy status endpoint - same category as
the grype.anchore.io block hit during the FlexAppOneDownloadMonitor Sparks
audit). This client is written against OSV's documented, stable public API
and validated with mocked-HTTP unit tests (see tests/test_osv_client.py),
but has not been exercised against the real endpoint from this environment.
"""

from __future__ import annotations

import hashlib
import json
import logging
from pathlib import Path
from typing import Any, Callable

import requests

logger = logging.getLogger(__name__)

_DEFAULT_BASE_URL = "https://api.osv.dev"
_DEFAULT_BATCH_SIZE = 100


def _cache_key(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


class OSVClient:
    def __init__(
        self,
        cache_dir: Path | str,
        base_url: str = _DEFAULT_BASE_URL,
        batch_size: int = _DEFAULT_BATCH_SIZE,
        session: requests.Session | None = None,
        timeout: float = 30.0,
    ) -> None:
        self.cache_dir = Path(cache_dir)
        self.purl_cache_dir = self.cache_dir / "osv-purl"
        self.vuln_cache_dir = self.cache_dir / "osv-vuln"
        self.purl_cache_dir.mkdir(parents=True, exist_ok=True)
        self.vuln_cache_dir.mkdir(parents=True, exist_ok=True)

        self.base_url = base_url.rstrip("/")
        self.batch_size = batch_size
        self.session = session or requests.Session()
        self.timeout = timeout

    # -- caching helpers -----------------------------------------------

    def _read_cache(self, cache_dir: Path, key: str) -> Any | None:
        path = cache_dir / f"{_cache_key(key)}.json"
        if not path.exists():
            return None
        with path.open("r", encoding="utf-8") as f:
            return json.load(f)

    def _write_cache(self, cache_dir: Path, key: str, value: Any) -> None:
        path = cache_dir / f"{_cache_key(key)}.json"
        with path.open("w", encoding="utf-8") as f:
            json.dump(value, f)

    # -- batch purl -> vuln ID lookup ------------------------------------

    def query_batch(self, purls: list[str]) -> dict[str, list[str]]:
        """Returns {purl: [vuln_id, ...]} for every purl given.

        Cached purls never hit the network again. Uncached purls are sent
        to /v1/querybatch in groups of self.batch_size.
        """
        results: dict[str, list[str]] = {}
        uncached: list[str] = []

        for purl in purls:
            cached = self._read_cache(self.purl_cache_dir, purl)
            if cached is not None:
                results[purl] = cached
            else:
                uncached.append(purl)

        for i in range(0, len(uncached), self.batch_size):
            batch = uncached[i : i + self.batch_size]
            batch_results = self._query_batch_uncached(batch)
            for purl, vuln_ids in batch_results.items():
                self._write_cache(self.purl_cache_dir, purl, vuln_ids)
                results[purl] = vuln_ids

        return results

    def _query_batch_uncached(self, purls: list[str]) -> dict[str, list[str]]:
        if not purls:
            return {}

        body = {"queries": [{"package": {"purl": p}} for p in purls]}
        response = self.session.post(
            f"{self.base_url}/v1/querybatch",
            json=body,
            timeout=self.timeout,
        )
        response.raise_for_status()
        payload = response.json()

        results_list = payload.get("results", [])
        out: dict[str, list[str]] = {}
        for purl, result in zip(purls, results_list):
            vulns = result.get("vulns", []) if result else []
            out[purl] = [v["id"] for v in vulns if "id" in v]
        return out

    # -- per-ID full vulnerability detail --------------------------------

    def get_vulnerability(self, vuln_id: str) -> dict[str, Any]:
        cached = self._read_cache(self.vuln_cache_dir, vuln_id)
        if cached is not None:
            return cached

        response = self.session.get(
            f"{self.base_url}/v1/vulns/{vuln_id}",
            timeout=self.timeout,
        )
        response.raise_for_status()
        data = response.json()
        self._write_cache(self.vuln_cache_dir, vuln_id, data)
        return data

    # -- combined convenience API -----------------------------------------

    def resolve(
        self,
        purls: list[str],
        on_progress: Callable[[int, int], None] | None = None,
    ) -> dict[str, list[dict[str, Any]]]:
        """Returns {purl: [full_vuln_dict, ...]} for every purl given.

        on_progress(done, total), if given, is called after each of the
        per-ID detail fetches below (the sequential, potentially-slow part
        for a package with many distinct vuln IDs) - not the batch lookup,
        which is a single request regardless of purl count.
        """
        purl_to_ids = self.query_batch(purls)
        unique_ids = sorted({vid for ids in purl_to_ids.values() for vid in ids})

        vuln_by_id: dict[str, dict[str, Any]] = {}
        for i, vuln_id in enumerate(unique_ids, start=1):
            try:
                vuln_by_id[vuln_id] = self.get_vulnerability(vuln_id)
            except requests.HTTPError as exc:
                logger.warning("Failed to fetch vulnerability details for %s: %s", vuln_id, exc)
            if on_progress:
                on_progress(i, len(unique_ids))

        return {
            purl: [vuln_by_id[vid] for vid in ids if vid in vuln_by_id]
            for purl, ids in purl_to_ids.items()
        }
