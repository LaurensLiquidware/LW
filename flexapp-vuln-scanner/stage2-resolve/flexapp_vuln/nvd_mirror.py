"""Local NVD CVE mirror: bulk-download once, match locally forever after.

Why this exists: `nvd_client.py`'s `NVDClient` makes one live HTTP request
per CPE candidate, rate-limited to 5 req/30s without an API key (50/30s
with one). A single package with a few hundred resolved components can
take tens of minutes, and that doesn't scale to running this against many
customer packages. NVD retired its downloadable JSON/XML CVE feed files in
December 2023 in favor of the API-only model, so "download the feed" now
means: paginate the full CVE list once via the same API 2.0 endpoint (no
`cpeName` filter, `resultsPerPage=2000`), and build a local index from
each CVE's `configurations` (the actual CPE match criteria, including
version ranges) - then match a candidate CPE against that index with zero
further network calls.

This trades a large one-time (or periodic, via `--modified-since-days`)
download for instant, unlimited local matching. As of 2026 the full NVD
CVE dataset is on the order of 260k+ entries; expect the initial mirror
build to take from tens of minutes (with an API key) to a few hours
(without one) and produce an index file in the tens-of-MB range. This is
a real cost, not a bug - run it as a scheduled refresh, not per-scan.

Version-range matching (`versionStartIncluding`/`versionEndExcluding`/etc.)
uses `version_compare.py`'s tokenized comparator, which is best-effort and
NOT authoritative for vendor-specific version schemes (dates, non-numeric
suffixes, epoch-style prefixes) - see that module's docstring. Matches
derived from a range comparison are no less confident than the live NVD
API's own equivalent cpeName lookup (the API performs the same kind of
range match server-side), but are only as good as this comparator.

`configurations[].nodes[].operator` (AND across multiple distinct
products, e.g. "vulnerable only when both product A and library B are
present") is deliberately NOT modeled - every `cpeMatch` entry marked
`vulnerable: true` anywhere in any node is treated as an independent
match. This can occasionally over-match a composite condition as if it
applied unconditionally; documented here rather than silently assumed
away.
"""

from __future__ import annotations

import json
import logging
import re
import time
from pathlib import Path
from typing import Any, Iterator

import requests

from .version_compare import version_in_range

logger = logging.getLogger(__name__)

_DEFAULT_BASE_URL = "https://services.nvd.nist.gov/rest/json/cves/2.0"
_PAGE_SIZE = 2000
_NO_KEY_LIMIT = 5
_WITH_KEY_LIMIT = 50
_WINDOW_SECONDS = 30.0
_MAX_429_RETRIES = 5

# Splits a cpe23 string on unescaped colons only, so an escaped colon
# inside a field (e.g. a version string containing "\:") isn't mistaken
# for a field separator.
_UNESCAPED_COLON_RE = re.compile(r"(?<!\\):")


def parse_cpe23(cpe23: str) -> list[str]:
    """Splits a `cpe:2.3:...` string into its 13 colon-separated fields
    (part, vendor, product, version, update, edition, language, sw_edition,
    target_sw, target_hw, other), leaving each field's own backslash
    escaping untouched.
    """
    return _UNESCAPED_COLON_RE.split(cpe23)


def _throttle(request_times: list[float], limit: int, sleep_fn) -> None:
    now = time.monotonic()
    while request_times and now - request_times[0] > _WINDOW_SECONDS:
        request_times.pop(0)
    if len(request_times) >= limit:
        wait = _WINDOW_SECONDS - (now - request_times[0])
        if wait > 0:
            sleep_fn(wait)
    request_times.append(time.monotonic())


def iter_all_cves(
    api_key: str | None = None,
    base_url: str = _DEFAULT_BASE_URL,
    session: requests.Session | None = None,
    last_mod_start: str | None = None,
    last_mod_end: str | None = None,
    timeout: float = 60.0,
    sleep_fn=time.sleep,
) -> Iterator[dict[str, Any]]:
    """Yields every raw CVE record (the `cve` object, not the wrapping
    `vulnerabilities[]` envelope) from NVD's 2.0 API, paginating with
    `resultsPerPage=2000`. Pass `last_mod_start`/`last_mod_end` (ISO 8601,
    both required together, NVD caps the span at 120 days) for an
    incremental refresh instead of a full rebuild.
    """
    session = session or requests.Session()
    limit = _WITH_KEY_LIMIT if api_key else _NO_KEY_LIMIT
    headers = {"apiKey": api_key} if api_key else {}
    request_times: list[float] = []

    start_index = 0
    total_results = None

    while total_results is None or start_index < total_results:
        params: dict[str, Any] = {"resultsPerPage": _PAGE_SIZE, "startIndex": start_index}
        if last_mod_start and last_mod_end:
            params["lastModStartDate"] = last_mod_start
            params["lastModEndDate"] = last_mod_end

        for attempt in range(_MAX_429_RETRIES + 1):
            _throttle(request_times, limit, sleep_fn)
            response = session.get(base_url, params=params, headers=headers, timeout=timeout)
            if response.status_code == 429:
                if attempt == _MAX_429_RETRIES:
                    response.raise_for_status()
                retry_after = response.headers.get("Retry-After")
                sleep_fn(float(retry_after) if retry_after else _WINDOW_SECONDS)
                continue
            response.raise_for_status()
            break

        data = response.json()
        total_results = data.get("totalResults", 0)
        page = data.get("vulnerabilities", [])
        for item in page:
            cve = item.get("cve")
            if cve:
                yield cve

        start_index += len(page)
        if not page:
            break  # defensive: avoid an infinite loop on an unexpected empty page


def _iter_cpe_matches(cve: dict[str, Any]) -> Iterator[dict[str, Any]]:
    for config in cve.get("configurations", []):
        for node in config.get("nodes", []):
            yield from node.get("cpeMatch", [])


def build_index(cves: Iterator[dict[str, Any]]) -> dict[str, Any]:
    """Builds the local mirror's on-disk structure from a stream of raw
    NVD CVE records: a details table keyed by CVE ID (trimmed to the
    fields `nvd_client.NVDClient.extract_cves` needs), and a
    `vendor:product` -> [criteria...] index for matching.
    """
    cve_details: dict[str, Any] = {}
    cpe_index: dict[str, list[dict[str, Any]]] = {}

    for cve in cves:
        cve_id = cve.get("id")
        if not cve_id:
            continue
        cve_details[cve_id] = {
            "id": cve_id,
            "descriptions": cve.get("descriptions", []),
            "metrics": cve.get("metrics", {}),
        }

        seen_criteria_for_this_cve: set[str] = set()
        for match in _iter_cpe_matches(cve):
            if not match.get("vulnerable", False):
                continue
            criteria = match.get("criteria")
            if not criteria:
                continue
            fields = parse_cpe23(criteria)
            if len(fields) < 6:
                continue
            # cpe:2.3:<part>:<vendor>:<product>:<version>:... - index 0 is
            # the literal "cpe" and 1 is "2.3", so part/vendor/product/
            # version land at 2/3/4/5, not 1/2/3/4.
            vendor, product, version = fields[3], fields[4], fields[5]
            key = f"{vendor}:{product}"

            # A single CVE can list the same vendor:product combination in
            # several configuration nodes (e.g. one per major branch) -
            # collapse to avoid duplicate entries against the same CVE.
            dedup_key = f"{criteria}|{match.get('versionStartIncluding')}|{match.get('versionStartExcluding')}|{match.get('versionEndIncluding')}|{match.get('versionEndExcluding')}"
            if dedup_key in seen_criteria_for_this_cve:
                continue
            seen_criteria_for_this_cve.add(dedup_key)

            cpe_index.setdefault(key, []).append({
                "cveId": cve_id,
                "version": version,
                "versionStartIncluding": match.get("versionStartIncluding"),
                "versionStartExcluding": match.get("versionStartExcluding"),
                "versionEndIncluding": match.get("versionEndIncluding"),
                "versionEndExcluding": match.get("versionEndExcluding"),
            })

    return {"cveDetails": cve_details, "cpeIndex": cpe_index}


def save_mirror(index: dict[str, Any], output_dir: Path | str, generated_utc: str) -> Path:
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    path = output_dir / "nvd-mirror.json"
    with path.open("w", encoding="utf-8") as f:
        json.dump({"generatedUtc": generated_utc, **index}, f)
    return path


def load_mirror(path: Path | str) -> dict[str, Any]:
    with Path(path).open("r", encoding="utf-8") as f:
        return json.load(f)


def merge_index(old: dict[str, Any], new: dict[str, Any], updated_cve_ids: set[str]) -> dict[str, Any]:
    """Merges a freshly-fetched (incremental) index into an existing one.
    A CVE's CPE matches can change between refreshes (a version range gets
    corrected, a new affected product added), so every stale entry for a
    CVE that was actually refetched is dropped before the new entries for
    it are added back - a plain union would leave superseded entries
    behind forever.
    """
    merged_details = dict(old.get("cveDetails", {}))
    merged_details.update(new.get("cveDetails", {}))

    merged_cpe_index: dict[str, list[dict[str, Any]]] = {}
    for key, entries in old.get("cpeIndex", {}).items():
        kept = [e for e in entries if e.get("cveId") not in updated_cve_ids]
        if kept:
            merged_cpe_index[key] = kept

    for key, entries in new.get("cpeIndex", {}).items():
        merged_cpe_index.setdefault(key, []).extend(entries)

    return {"cveDetails": merged_details, "cpeIndex": merged_cpe_index}


class NVDLocalMatcher:
    """Drop-in replacement for `NVDClient` that matches against a
    pre-built local mirror instead of making a live HTTP request per CPE.
    `query_cpe`'s return shape matches `NVDClient.query_cpe`'s, so
    `NVDClient.extract_cves` (a plain staticmethod) works unchanged on
    either source.
    """

    def __init__(self, mirror: dict[str, Any]):
        self._cve_details = mirror.get("cveDetails", {})
        self._cpe_index = mirror.get("cpeIndex", {})

    @classmethod
    def from_path(cls, path: Path | str) -> "NVDLocalMatcher":
        return cls(load_mirror(path))

    def query_cpe(self, cpe23: str) -> dict[str, Any]:
        fields = parse_cpe23(cpe23)
        if len(fields) < 6:
            return {"vulnerabilities": []}
        vendor, product, version = fields[3], fields[4], fields[5]
        key = f"{vendor}:{product}"

        matched_cve_ids: set[str] = set()
        for criteria in self._cpe_index.get(key, []):
            bounds_present = any(
                criteria.get(f) is not None
                for f in (
                    "versionStartIncluding",
                    "versionStartExcluding",
                    "versionEndIncluding",
                    "versionEndExcluding",
                )
            )
            if bounds_present:
                if version_in_range(
                    version,
                    start_including=criteria.get("versionStartIncluding"),
                    start_excluding=criteria.get("versionStartExcluding"),
                    end_including=criteria.get("versionEndIncluding"),
                    end_excluding=criteria.get("versionEndExcluding"),
                ):
                    matched_cve_ids.add(criteria["cveId"])
            elif criteria.get("version") in ("*", "-", version):
                # "*"/"-" with no bounds at all is NVD's shorthand for "any
                # version of this product is vulnerable" - otherwise the
                # CPE's own pinned version must match exactly.
                matched_cve_ids.add(criteria["cveId"])

        vulnerabilities = [
            {"cve": self._cve_details[cve_id]}
            for cve_id in sorted(matched_cve_ids)
            if cve_id in self._cve_details
        ]
        return {"vulnerabilities": vulnerabilities}
