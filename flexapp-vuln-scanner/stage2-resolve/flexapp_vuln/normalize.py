"""Converts a Stage 1 resolved identity into a Package URL (purl) or CPE
2.3 string, where possible.

Only identity methods that map cleanly onto an OSV-supported ecosystem
(Maven, npm, PyPI, NuGet) get a purl. Native/OS components (PE,
string-signature, dotnet-manifest, electron-embedded) get a CPE candidate
instead, via build_cpe_candidate - either a curated cpe-mappings.yaml
override (confidence "mapped-cpe") or an automatic heuristic normalization
(confidence "heuristic", per PLAN.md never to be presented as a confirmed
finding).
"""

from __future__ import annotations

import re
from typing import Any

from packageurl import PackageURL

from .cpe_mappings import CpeMappings

# Identity methods a CPE candidate is even worth attempting for - native/OS
# components with no purl-expressible ecosystem. jar-manifest is excluded:
# a jar with no groupId is still a Java library, not a native/OS component,
# and guessing a CPE for it from MANIFEST.MF alone would be pure noise.
_CPE_ELIGIBLE_METHODS = {
    "pe-version-resource",
    "dotnet-manifest",
    "string-signature",
    "electron-embedded",
}

# Corporate-suffix words stripped during heuristic vendor/product
# normalization - matches the kind of noise CompanyName/ProductName Win32
# resource fields commonly carry (e.g. "Google Inc.", "Acme Corporation").
_CORP_SUFFIXES = re.compile(
    r"\b(inc|incorporated|corp|corporation|llc|ltd|limited|gmbh|co)\b\.?",
    re.IGNORECASE,
)
_NON_ALNUM_RUN = re.compile(r"[^a-z0-9]+")


def _heuristic_normalize(text: str) -> str:
    """Best-effort CPE-vendor/product-shaped string: lowercase, corporate
    suffixes stripped, everything else collapsed to single underscores.
    This is a guess, not a lookup - callers must tag it "heuristic".
    """
    stripped = _CORP_SUFFIXES.sub("", text)
    return _NON_ALNUM_RUN.sub("_", stripped.lower()).strip("_")



# CPE 2.3 formatted-string "special characters" (NIST IR 7695 §6.1.2.4) that
# must be backslash-escaped. Found live: a real Win32 version resource's
# ProductVersion contained raw spaces and a colon ("2.1.23296 git hash:
# e323abb5b08e") - vendor/product go through _heuristic_normalize (which
# already collapses everything non-alnum to underscores) but `version` is
# deliberately kept verbatim for precise matching, so it needs this escaping
# applied directly rather than assuming it's already CPE-safe.
_CPE_SPECIAL_CHARS = re.compile(r'([!"#$%&\'()*+,/:;<=>?@\[\]^`{|}~\\ ])')


def _escape_cpe_component(text: str) -> str:
    return _CPE_SPECIAL_CHARS.sub(r"\\\1", text)

# PyPI purl normalization per the purl spec: lowercase, runs of -._ collapsed to a single -.
_PYPI_NAME_RUN = re.compile(r"[-_.]+")


def _normalize_pypi_name(name: str) -> str:
    return _PYPI_NAME_RUN.sub("-", name).lower()


def _split_npm_scope(name: str) -> tuple[str | None, str]:
    """Splits an npm package name into (namespace, name) for scoped packages.

    "@scope/pkg" -> ("@scope", "pkg"); "pkg" -> (None, "pkg").
    """
    if name.startswith("@") and "/" in name:
        namespace, _, rest = name.partition("/")
        return namespace, rest
    return None, name


def build_purl(identity: dict[str, Any] | None) -> str | None:
    """Returns a purl string for a Stage 1 identity object, or None if this
    identity method doesn't map onto a purl-expressible ecosystem.
    """
    if not identity:
        return None

    method = identity.get("method")
    version = identity.get("version")
    product = identity.get("product")

    if not version or not product:
        return None

    if method == "jar-pom-properties":
        raw = identity.get("raw") or {}
        group_id = raw.get("groupId")
        artifact_id = raw.get("artifactId", product)
        if not group_id or not artifact_id:
            return None
        return PackageURL(type="maven", namespace=group_id, name=artifact_id, version=version).to_string()

    if method == "node-package-json":
        namespace, name = _split_npm_scope(product)
        return PackageURL(type="npm", namespace=namespace, name=name, version=version).to_string()

    if method == "python-dist-info":
        return PackageURL(type="pypi", name=_normalize_pypi_name(product), version=version).to_string()

    if method == "dotnet-deps-json":
        return PackageURL(type="nuget", name=product, version=version).to_string()

    # jar-manifest has no groupId, so no Maven purl can be built reliably.
    # dotnet-manifest, pe-version-resource, string-signature, electron-embedded
    # are native/OS components - not purl-expressible, handled via CPE
    # (build_cpe_candidate, below) instead.
    return None


def build_cpe_candidate(
    identity: dict[str, Any] | None,
    mappings: CpeMappings,
) -> tuple[str | None, str | None]:
    """Returns (cpe23_string, confidence) for a Stage 1 identity, or
    (None, None) if this identity isn't CPE-eligible or lacks a version.

    confidence is "mapped-cpe" when cpe-mappings.yaml has a curated override
    for this vendor/product, or "heuristic" when falling back to automatic
    normalization - callers must not treat these the same way (PLAN.md:
    never present a heuristic match as a confirmed finding).
    """
    if not identity:
        return None, None

    method = identity.get("method")
    if method not in _CPE_ELIGIBLE_METHODS:
        return None, None

    version = identity.get("version")
    if not version:
        return None, None

    mapped = mappings.find(identity)
    if mapped:
        vendor, product = mapped
        confidence = "mapped-cpe"
        version_transform = mappings.find_version_transform(identity)
        if version_transform:
            pattern, group = version_transform
            match = re.match(pattern, str(version))
            # A version that doesn't fit the expected shape falls back to
            # the raw value unchanged - same "don't guess" spirit as the
            # rest of this module; worst case is a CPE that doesn't match
            # anything in NVD (silently zero findings), not a wrong one.
            if match:
                version = match.group(group)
    else:
        vendor_raw = identity.get("vendor") or identity.get("product")
        product_raw = identity.get("product")
        if not product_raw:
            return None, None
        vendor = _heuristic_normalize(vendor_raw or product_raw)
        product = _heuristic_normalize(product_raw)
        if not vendor or not product:
            return None, None
        confidence = "heuristic"

    cpe23 = "cpe:2.3:a:{}:{}:{}:*:*:*:*:*:*:*".format(
        _escape_cpe_component(vendor),
        _escape_cpe_component(product),
        _escape_cpe_component(str(version)),
    )
    return cpe23, confidence
