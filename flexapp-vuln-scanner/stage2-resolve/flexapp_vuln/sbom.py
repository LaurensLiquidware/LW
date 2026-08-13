"""Builds a CycloneDX 1.6 JSON SBOM from a Stage 1 inventory.

Deliberately independent of OSV/NVD matching (and therefore of network
access) - purl/CPE are recomputed here directly via normalize.py, the same
way the `resolve` command does, so an SBOM can always be produced from an
inventory JSON alone.

No license data is included - Stage 1 never captures license information
for any component, and CycloneDX allows omitting the `licenses` field
entirely. Claiming a license here would be fabricating data this pipeline
never actually collected.
"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from typing import Any

from .cpe_mappings import CpeMappings
from .inventory import iter_non_excluded_files
from .normalize import build_cpe_candidate, build_purl


def _dedup_key(purl: str | None, cpe: str | None, identity: dict[str, Any]) -> str:
    if purl:
        return f"purl:{purl}"
    if cpe:
        return f"cpe:{cpe}"
    # Only reached for resolved-but-neither-purl-nor-cpe identities
    # (currently just jar-manifest) - dedupe by the raw identity triple.
    return f"raw:{identity.get('method')}:{identity.get('product')}:{identity.get('version')}"


def build_sbom(
    inventory: dict[str, Any],
    cpe_mappings: CpeMappings | None = None,
) -> dict[str, Any]:
    cpe_mappings = cpe_mappings or CpeMappings.load()

    package = inventory.get("package", {})
    flex_xml = package.get("flexAppXml") or {}
    app_name = flex_xml.get("displayName") or package.get("sourcePath", "unknown-package")
    app_version = flex_xml.get("versionMajorMinorBuildRevision") or "0.0.0.0"

    seen: dict[str, dict[str, Any]] = {}

    for record in iter_non_excluded_files(inventory):
        identity = record.get("identity")
        if not identity:
            continue

        purl = build_purl(identity)
        cpe, cpe_confidence = (None, None) if purl else build_cpe_candidate(identity, cpe_mappings)
        confidence = "exact-purl" if purl else cpe_confidence

        key = _dedup_key(purl, cpe, identity)
        if key in seen:
            continue

        component: dict[str, Any] = {
            "type": "library",
            "name": identity.get("product"),
            "version": identity.get("version"),
            "bom-ref": key,
            "properties": [
                {"name": "flexapp-vuln:resolutionMethod", "value": identity.get("method")},
                {"name": "flexapp-vuln:confidence", "value": confidence or "unresolved"},
            ],
        }
        if purl:
            component["purl"] = purl
        if cpe:
            component["cpe"] = cpe
        if record.get("sha256"):
            component["hashes"] = [{"alg": "SHA-256", "content": record["sha256"]}]

        seen[key] = component

    return {
        "bomFormat": "CycloneDX",
        "specVersion": "1.6",
        "serialNumber": f"urn:uuid:{uuid.uuid4()}",
        "version": 1,
        "metadata": {
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "component": {
                "type": "application",
                "name": app_name,
                "version": app_version,
            },
        },
        "components": list(seen.values()),
    }
