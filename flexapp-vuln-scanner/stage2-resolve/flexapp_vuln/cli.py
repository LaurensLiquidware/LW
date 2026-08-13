"""CLI entry point for Stage 2.

Implements the "resolve" step of PLAN.md's build order through OSV.dev and
NVD/CPE matching: load a Stage 1 inventory JSON, build a purl for anything
OSV-expressible (Maven/npm/PyPI) and a CPE candidate for anything else
CPE-eligible (native PE, .NET, string-signature, electron-embedded), query
both sources, and write a combined <package>.vuln-matches.json. The polished
coverage-report.md/findings.md/sbom.cdx.json outputs are the next build step.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from pathlib import Path
from typing import Any

import requests

from .confidence import Confidence
from .cpe_mappings import CpeMappings
from .inventory import iter_non_excluded_files, load_inventory
from .normalize import build_cpe_candidate, build_purl
from .nvd_client import NVDClient
from .osv_client import OSVClient

logger = logging.getLogger(__name__)


class UnreachableService(Exception):
    """Raised when a required vulnerability-database host can't be reached,
    so the CLI can print one clear message instead of a raw traceback.
    """

    def __init__(self, host: str, original: Exception):
        super().__init__(f"could not reach {host}: {original}")
        self.host = host
        self.original = original


def _utc_now_iso() -> str:
    # Stamped by the caller at invocation time - this is a live CLI run, not
    # a workflow script, so real wall-clock time is fine here.
    from datetime import datetime, timezone

    return datetime.now(timezone.utc).isoformat()


def resolve_vuln_matches(
    inventory: dict[str, Any],
    cache_dir: Path,
    cpe_mappings: CpeMappings | None = None,
    nvd_api_key: str | None = None,
) -> dict[str, Any]:
    cpe_mappings = cpe_mappings or CpeMappings.load()

    candidates = []
    purl_set: set[str] = set()
    cpe_set: set[str] = set()

    for record in iter_non_excluded_files(inventory):
        identity = record.get("identity")
        purl = build_purl(identity)
        cpe, cpe_confidence = (None, None) if purl else build_cpe_candidate(identity, cpe_mappings)

        if purl:
            purl_set.add(purl)
        if cpe:
            cpe_set.add(cpe)

        candidates.append({
            "relativePath": record.get("relativePath"),
            "identity": identity,
            "purl": purl,
            "cpe": cpe,
            "cpeConfidence": cpe_confidence,
        })

    osv_client = OSVClient(cache_dir=cache_dir)
    try:
        osv_matches = osv_client.resolve(sorted(purl_set)) if purl_set else {}
    except requests.exceptions.RequestException as exc:
        raise UnreachableService("api.osv.dev", exc) from exc

    nvd_client = NVDClient(cache_dir=cache_dir, api_key=nvd_api_key)
    nvd_matches: dict[str, list[dict[str, Any]]] = {}
    try:
        for cpe23 in sorted(cpe_set):
            response = nvd_client.query_cpe(cpe23)
            nvd_matches[cpe23] = NVDClient.extract_cves(response)
    except requests.exceptions.RequestException as exc:
        raise UnreachableService("services.nvd.nist.gov", exc) from exc

    components = []
    for candidate in candidates:
        purl = candidate["purl"]
        cpe = candidate["cpe"]

        if purl:
            confidence = Confidence.EXACT_PURL.value
            vulnerabilities = [
                {"id": v.get("id"), "summary": v.get("summary"), "severity": v.get("severity", []), "source": "osv"}
                for v in osv_matches.get(purl, [])
            ]
        elif cpe:
            confidence = candidate["cpeConfidence"]
            vulnerabilities = [
                {"id": v.get("id"), "summary": v.get("summary"), "severity": v.get("severity", []), "source": "nvd"}
                for v in nvd_matches.get(cpe, [])
            ]
        else:
            confidence = None
            vulnerabilities = []

        components.append({
            "relativePath": candidate["relativePath"],
            "identity": candidate["identity"],
            "purl": purl,
            "cpe": cpe,
            "confidence": confidence,
            "vulnerabilities": vulnerabilities,
        })

    return {
        "generatedUtc": _utc_now_iso(),
        "package": inventory.get("package", {}),
        "components": components,
    }


def _cmd_resolve(args: argparse.Namespace) -> int:
    inventory_path = Path(args.inventory)
    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)

    inventory = load_inventory(inventory_path, schema_path=args.schema)
    cpe_mappings = CpeMappings.load(args.cpe_mappings)

    try:
        result = resolve_vuln_matches(
            inventory,
            cache_dir=Path(args.cache_dir),
            cpe_mappings=cpe_mappings,
            nvd_api_key=args.nvd_api_key,
        )
    except UnreachableService as exc:
        print(
            f"ERROR: could not reach {exc.host} "
            f"({exc.original.__class__.__name__}: {exc.original}).\n"
            "This is a network/proxy connectivity problem, not a code bug. "
            "If you're behind an egress-restricted proxy/firewall, run this "
            "step from a machine with access to that host, or check the "
            "proxy's allow-list.",
            file=sys.stderr,
        )
        return 1

    out_base = inventory_path.stem.removesuffix(".inventory")
    out_path = out_dir / f"{out_base}.vuln-matches.json"
    with out_path.open("w", encoding="utf-8") as f:
        json.dump(result, f, indent=2)

    total = len(result["components"])
    with_purl = sum(1 for c in result["components"] if c["purl"])
    with_cpe = sum(1 for c in result["components"] if c["cpe"])
    with_vulns = sum(1 for c in result["components"] if c["vulnerabilities"])
    print(f"Wrote {out_path}")
    print(
        f"  {total} candidate components, {with_purl} purl-expressible (OSV), "
        f"{with_cpe} CPE-expressible (NVD), {with_vulns} with matches"
    )
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="flexapp-vuln")
    subparsers = parser.add_subparsers(dest="command", required=True)

    resolve_parser = subparsers.add_parser(
        "resolve", help="Resolve OSV.dev + NVD matches for a Stage 1 inventory JSON"
    )
    resolve_parser.add_argument("inventory", help="Path to a Stage 1 <package>.inventory.json file")
    resolve_parser.add_argument("--out", required=True, help="Output directory")
    resolve_parser.add_argument("--cache-dir", default="cache", help="On-disk cache directory (default: ./cache)")
    resolve_parser.add_argument("--schema", default=None, help="Override path to inventory.schema.json")
    resolve_parser.add_argument(
        "--cpe-mappings", default=None, help="Override path to cpe-mappings.yaml (default: config/cpe-mappings.yaml)"
    )
    resolve_parser.add_argument(
        "--nvd-api-key",
        default=None,
        help="NVD API key (default: NVD_API_KEY env var, or unauthenticated 5 req/30s if unset)",
    )
    resolve_parser.set_defaults(func=_cmd_resolve)

    return parser


def main(argv: list[str] | None = None) -> int:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")
    parser = build_parser()
    args = parser.parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
