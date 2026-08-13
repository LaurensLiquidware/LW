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

    def find(self, identity: dict[str, Any] | None) -> tuple[str, str] | None:
        """Returns (cpeVendor, cpeProduct) for the first matching override,
        or None if nothing in the table matches this identity.
        """
        if not identity:
            return None

        method = identity.get("method")
        vendor = (identity.get("vendor") or "").lower()
        product = (identity.get("product") or "").lower()

        for entry in self._mappings:
            match = entry.get("match", {})
            if match.get("method") != method:
                continue
            match_vendor = match.get("vendor")
            if match_vendor is not None and match_vendor.lower() != vendor:
                continue
            match_product = match.get("product")
            if match_product is not None and match_product.lower() != product:
                continue

            cpe = entry.get("cpe", {})
            cpe_vendor = cpe.get("vendor")
            cpe_product = cpe.get("product")
            if cpe_vendor and cpe_product:
                return cpe_vendor, cpe_product

        return None
