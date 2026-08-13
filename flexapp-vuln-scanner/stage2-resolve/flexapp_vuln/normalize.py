"""Converts a Stage 1 resolved identity into a Package URL (purl), where possible.

Only identity methods that map cleanly onto an OSV-supported ecosystem
(Maven, npm, PyPI - NuGet is deliberately NOT attempted here, see below) get
a purl. Everything else (native PE, string-signature, dotnet-manifest) is
left for the NVD/CPE path (PLAN.md's next build step) - returning None here
is the correct, honest answer for those, not a bug to paper over.
"""

from __future__ import annotations

import re
from typing import Any

from packageurl import PackageURL

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

    # jar-manifest has no groupId, so no Maven purl can be built reliably.
    # dotnet-manifest, pe-version-resource, string-signature, electron-embedded
    # are native/OS components - not purl-expressible, handled via CPE (next
    # build step) instead.
    return None
