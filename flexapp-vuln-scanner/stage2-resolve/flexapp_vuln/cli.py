"""CLI entry point for Stage 2.

Currently implements the "resolve" step of PLAN.md's build order: load a
Stage 1 inventory JSON, build purls for every OSV-expressible identity, and
query OSV.dev for matches. Writes an intermediate osv-matches.json - the
polished coverage-report.md/findings.md/sbom.cdx.json outputs are a later
build step (NVD matching comes first, per PLAN.md).
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
from .inventory import iter_non_excluded_files, load_inventory
from .normalize import build_purl
from .osv_client import OSVClient

logger = logging.getLogger(__name__)


def _utc_now_iso() -> str:
    # Stamped by the caller at invocation time - this is a live CLI run, not
    # a workflow script, so real wall-clock time is fine here.
    from datetime import datetime, timezone

    return datetime.now(timezone.utc).isoformat()


def resolve_osv_matches(inventory: dict[str, Any], cache_dir: Path) -> dict[str, Any]:
    candidates = []
    purl_set: set[str] = set()

    for record in iter_non_excluded_files(inventory):
        identity = record.get("identity")
        purl = build_purl(identity)
        if purl:
            purl_set.add(purl)
        candidates.append({
            "relativePath": record.get("relativePath"),
            "identity": identity,
            "purl": purl,
        })

    client = OSVClient(cache_dir=cache_dir)
    matches = client.resolve(sorted(purl_set)) if purl_set else {}

    components = []
    for candidate in candidates:
        purl = candidate["purl"]
        vulns = matches.get(purl, []) if purl else []
        components.append({
            "relativePath": candidate["relativePath"],
            "identity": candidate["identity"],
            "purl": purl,
            "confidence": Confidence.EXACT_PURL.value if purl else None,
            "vulnerabilities": [
                {
                    "id": v.get("id"),
                    "summary": v.get("summary"),
                    "severity": v.get("severity", []),
                    "aliases": v.get("aliases", []),
                }
                for v in vulns
            ],
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
    try:
        result = resolve_osv_matches(inventory, cache_dir=Path(args.cache_dir))
    except requests.exceptions.RequestException as exc:
        print(
            "ERROR: could not reach api.osv.dev "
            f"({exc.__class__.__name__}: {exc}).\n"
            "This is a network/proxy connectivity problem, not a code bug - "
            "OSV.dev matching requires outbound HTTPS access to api.osv.dev. "
            "If you're behind an egress-restricted proxy/firewall, run this "
            "step from a machine with that access, or check the proxy's "
            "allow-list.",
            file=sys.stderr,
        )
        return 1

    out_base = inventory_path.stem.removesuffix(".inventory")
    out_path = out_dir / f"{out_base}.osv-matches.json"
    with out_path.open("w", encoding="utf-8") as f:
        json.dump(result, f, indent=2)

    total = len(result["components"])
    with_purl = sum(1 for c in result["components"] if c["purl"])
    with_vulns = sum(1 for c in result["components"] if c["vulnerabilities"])
    print(f"Wrote {out_path}")
    print(f"  {total} candidate components, {with_purl} purl-expressible, {with_vulns} with OSV matches")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="flexapp-vuln")
    subparsers = parser.add_subparsers(dest="command", required=True)

    resolve_parser = subparsers.add_parser("resolve", help="Resolve OSV.dev matches for a Stage 1 inventory JSON")
    resolve_parser.add_argument("inventory", help="Path to a Stage 1 <package>.inventory.json file")
    resolve_parser.add_argument("--out", required=True, help="Output directory")
    resolve_parser.add_argument("--cache-dir", default="cache", help="On-disk cache directory (default: ./cache)")
    resolve_parser.add_argument("--schema", default=None, help="Override path to inventory.schema.json")
    resolve_parser.set_defaults(func=_cmd_resolve)

    return parser


def main(argv: list[str] | None = None) -> int:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")
    parser = build_parser()
    args = parser.parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
