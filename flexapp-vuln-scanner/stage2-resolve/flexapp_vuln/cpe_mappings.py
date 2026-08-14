"""Loads config/cpe-mappings.yaml and looks up manual vendor/product -> CPE
overrides for a Stage 1 identity.

Per PLAN.md: CPE resolution from PE metadata is lossy and will produce both
false positives and misses, so this is a small, editable override table
rather than an attempt to be clever about automatic normalization. A match
found here is confidence "mapped-cpe"; anything not found here falls back
to automatic heuristic normalization elsewhere (see normalize.py), which is
confidence "heuristic" instead - a strictly lower bar of trust.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml

_DEFAULT_MAPPINGS_PATH = Path(__file__).resolve().parents[1] / "config" / "cpe-mappings.yaml"


class CpeMappings:
    def __init__(self, mappings: list[dict[str, Any]]):
        self._mappings = mappings

    @classmethod
    def load(cls, path: Path | str | None = None) -> "CpeMappings":
        mappings_path = Path(path) if path else _DEFAULT_MAPPINGS_PATH
        if not mappings_path.exists():
            return cls(mappings=[])
        with mappings_path.open("r", encoding="utf-8") as f:
            data = yaml.safe_load(f) or {}
        return cls(mappings=data.get("mappings", []))

    def _find_entry(self, identity: dict[str, Any] | None) -> dict[str, Any] | None:
        if not identity:
            return None

        method = identity.get("method")
        vendor = (identity.get("vendor") or "").lower()
        product = (identity.get("product") or "").lower()

        for entry in self._mappings:
            match = entry.get("match", {})
            # `method` is optional in a mapping entry: found live that a
            # real "zlib" Win32 version resource (method
            # pe-version-resource) missed a mapping written only for the
            # string-signature path, because it required an exact method
            # match. A distinctive product name like "zlib" is unambiguous
            # regardless of which method found it - only scope by method
            # when the entry itself needs that (e.g. "Electron Chromium" is
            # a label that's only ever produced by electron-embedded).
            match_method = match.get("method")
            if match_method is not None and match_method != method:
                continue
            match_vendor = match.get("vendor")
            if match_vendor is not None and match_vendor.lower() != vendor:
                continue
            match_product = match.get("product")
            if match_product is not None and match_product.lower() != product:
                continue

            cpe = entry.get("cpe", {})
            if cpe.get("vendor") and cpe.get("product"):
                return entry

        return None

    def find(self, identity: dict[str, Any] | None) -> tuple[str, str] | None:
        """Returns (cpeVendor, cpeProduct) for the first matching override,
        or None if nothing in the table matches this identity.
        """
        entry = self._find_entry(identity)
        if not entry:
            return None
        cpe = entry["cpe"]
        return cpe["vendor"], cpe["product"]

    def find_version_transform(self, identity: dict[str, Any] | None) -> tuple[str, int] | None:
        """Returns (regexPattern, captureGroup) for the matching entry's
        optional `versionPattern`/`versionGroup`, or None if the matching
        entry has no version transform (or nothing matched at all).

        Exists because a Stage 1 identity's raw version string doesn't
        always match NVD's own version format for that product - found
        live comparing against the real NVD CPE dictionary: FFmpeg reports
        its own git-tag-style version ("n7.1.1", leading "n") where NVD's
        dictionary uses plain "7.1.1"; Qt's Win32 FILEVERSION resource is
        4-part ("6.8.3.0") where NVD's dictionary is 3-part ("6.8.3"). A
        vendor/product fix alone doesn't help if the version itself still
        can't match any real dictionary entry.
        """
        entry = self._find_entry(identity)
        if not entry:
            return None
        cpe = entry.get("cpe", {})
        pattern = cpe.get("versionPattern")
        if not pattern:
            return None
        return pattern, cpe.get("versionGroup", 1)
