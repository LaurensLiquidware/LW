"""Match confidence levels, per PLAN.md: never present a heuristic match as
a confirmed finding.
"""

from __future__ import annotations

from enum import Enum


class Confidence(str, Enum):
    EXACT_PURL = "exact-purl"
    MAPPED_CPE = "mapped-cpe"
    HEURISTIC = "heuristic"

    def __str__(self) -> str:  # so json.dumps / f-strings show the plain value
        return self.value
