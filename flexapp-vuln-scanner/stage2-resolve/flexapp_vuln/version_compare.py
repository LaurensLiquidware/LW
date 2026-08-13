"""Loose version comparison for matching a component version against an
NVD CPE match range (versionStartIncluding/versionEndExcluding/etc.).

NVD version strings are not semver - they're whatever the vendor's own
scheme happens to be (dotted numbers, dates, dotted-numbers-with-suffix,
...). This is a best-effort, RPM/dpkg-style tokenized comparator: split
into alternating digit/non-digit runs, compare numeric runs as integers
and everything else as strings. It gets common cases right (1.2.10 >
1.2.9) but is NOT a CPE-spec or semver-compliant comparator - treat any
match derived from a range comparison as best-effort, same spirit as this
project's `heuristic` confidence tier elsewhere.
"""

from __future__ import annotations

import re

_TOKEN_RE = re.compile(r"\d+|\D+")


def _tokenize(version: str) -> list[str]:
    return _TOKEN_RE.findall(version)


def compare_versions(a: str, b: str) -> int:
    """Returns -1/0/1 for a<b, a==b, a>b, comparing token-by-token."""
    a_tokens = _tokenize(a)
    b_tokens = _tokenize(b)

    for a_tok, b_tok in zip(a_tokens, b_tokens):
        if a_tok == b_tok:
            continue
        a_is_num = a_tok.isdigit()
        b_is_num = b_tok.isdigit()
        if a_is_num and b_is_num:
            a_num, b_num = int(a_tok), int(b_tok)
            if a_num != b_num:
                return -1 if a_num < b_num else 1
            continue
        # A numeric run sorts before a non-numeric run at the same
        # position (matches common "1.0" < "1.0-beta" expectations).
        if a_is_num != b_is_num:
            return -1 if a_is_num else 1
        return -1 if a_tok < b_tok else 1

    if len(a_tokens) == len(b_tokens):
        return 0
    return -1 if len(a_tokens) < len(b_tokens) else 1


def version_in_range(
    version: str,
    start_including: str | None = None,
    start_excluding: str | None = None,
    end_including: str | None = None,
    end_excluding: str | None = None,
) -> bool:
    """True if `version` satisfies every provided bound. No bounds at all
    means "unbounded" - matches everything, matching NVD's own semantics
    for a cpeMatch entry that only pins an exact version (handled by the
    caller before ever calling this).
    """
    if start_including is not None and compare_versions(version, start_including) < 0:
        return False
    if start_excluding is not None and compare_versions(version, start_excluding) <= 0:
        return False
    if end_including is not None and compare_versions(version, end_including) > 0:
        return False
    if end_excluding is not None and compare_versions(version, end_excluding) >= 0:
        return False
    return True
