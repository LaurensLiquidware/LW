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
from .coverage import compute_coverage
from .cpe_mappings import CpeMappings
from .inventory import iter_non_excluded_files, load_inventory
from .normalize import build_cpe_candidate, build_purl
from .nvd_client import NVDClient
from .nvd_mirror import NVDLocalMatcher, build_index, iter_all_cves, load_mirror, merge_index, save_mirror
from .osv_client import OSVClient
from .pdf_report import render_pdf_report
from .reporting import render_coverage_report, render_findings
from .sbom import build_sbom

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
    nvd_mirror_path: Path | None = None,
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

    # A local mirror (see nvd_mirror.py, built via `mirror-nvd`) answers
    # every CPE candidate from an in-memory index with zero network calls -
    # skips the live API's 5-50 req/30s rate limit entirely, which matters
    # once a package's resolved-component count runs into the hundreds.
    nvd_matches: dict[str, list[dict[str, Any]]] = {}
    if nvd_mirror_path is not None:
        local_matcher = NVDLocalMatcher.from_path(nvd_mirror_path)
        for cpe23 in sorted(cpe_set):
            response = local_matcher.query_cpe(cpe23)
            nvd_matches[cpe23] = NVDClient.extract_cves(response)
    else:
        nvd_client = NVDClient(cache_dir=cache_dir, api_key=nvd_api_key)
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
                {
                    "id": v.get("id"),
                    "summary": v.get("summary"),
                    "severity": v.get("severity", []),
                    # GHSA-sourced OSV entries commonly carry this; many
                    # other ecosystems don't - None is an honest "unknown",
                    # not something to guess at.
                    "severityLevel": (v.get("database_specific") or {}).get("severity"),
                    "source": "osv",
                }
                for v in osv_matches.get(purl, [])
            ]
        elif cpe:
            confidence = candidate["cpeConfidence"]
            vulnerabilities = [
                {
                    "id": v.get("id"),
                    "summary": v.get("summary"),
                    "severity": v.get("severity", []),
                    "severityLevel": v.get("severityLevel"),
                    "source": "nvd",
                }
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
            nvd_mirror_path=Path(args.nvd_mirror) if args.nvd_mirror else None,
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


def _cmd_mirror_nvd(args: argparse.Namespace) -> int:
    from datetime import datetime, timedelta, timezone

    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)
    mirror_path = out_dir / "nvd-mirror.json"

    last_mod_start = last_mod_end = None
    existing = None
    if args.modified_since_days is not None:
        if mirror_path.exists():
            existing = load_mirror(mirror_path)
        now = datetime.now(timezone.utc)
        last_mod_start = (now - timedelta(days=args.modified_since_days)).strftime("%Y-%m-%dT%H:%M:%S.000")
        last_mod_end = now.strftime("%Y-%m-%dT%H:%M:%S.000")
        print(f"Incremental refresh: fetching CVEs modified between {last_mod_start} and {last_mod_end}")
    else:
        print("Full mirror rebuild: fetching the entire NVD CVE dataset - this can take "
              "a long time (hours without an API key, tens of minutes with one)")

    cves = list(iter_all_cves(
        api_key=args.nvd_api_key,
        last_mod_start=last_mod_start,
        last_mod_end=last_mod_end,
    ))
    print(f"Fetched {len(cves)} CVE records")

    new_index = build_index(iter(cves))
    if existing is not None:
        updated_ids = {cve["id"] for cve in cves if cve.get("id")}
        index = merge_index(existing, new_index, updated_ids)
    else:
        index = new_index

    generated_utc = datetime.now(timezone.utc).isoformat()
    path = save_mirror(index, out_dir, generated_utc)
    print(f"Wrote {path} ({len(index['cveDetails'])} CVEs, {len(index['cpeIndex'])} distinct vendor:product keys)")
    return 0


def _package_display_name(inventory: dict[str, Any]) -> str:
    package = inventory.get("package", {})
    flex_xml = package.get("flexAppXml") or {}
    return flex_xml.get("displayName") or Path(package.get("sourcePath", "unknown-package")).stem


def _cmd_report(args: argparse.Namespace) -> int:
    inventory_path = Path(args.inventory)
    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)

    inventory = load_inventory(inventory_path, schema_path=args.schema)
    cpe_mappings = CpeMappings.load(args.cpe_mappings)
    package_name = _package_display_name(inventory)
    out_base = inventory_path.stem.removesuffix(".inventory")

    vuln_matches = None
    if args.vuln_matches:
        with Path(args.vuln_matches).open("r", encoding="utf-8") as f:
            vuln_matches = json.load(f)

    coverage = compute_coverage(inventory)
    sbom = build_sbom(inventory, cpe_mappings=cpe_mappings)
    coverage_report_md = render_coverage_report(coverage, package_name)
    findings_md = render_findings(vuln_matches, package_name)

    sbom_path = out_dir / f"{out_base}.sbom.cdx.json"
    coverage_path = out_dir / f"{out_base}.coverage-report.md"
    findings_path = out_dir / f"{out_base}.findings.md"

    with sbom_path.open("w", encoding="utf-8") as f:
        json.dump(sbom, f, indent=2)
    coverage_path.write_text(coverage_report_md, encoding="utf-8")
    findings_path.write_text(findings_md, encoding="utf-8")

    print(f"Wrote {sbom_path} ({len(sbom['components'])} components)")
    pct = coverage["coveragePercent"]
    pct_str = f"{pct:.1f}%" if pct is not None else "N/A"
    print(f"Wrote {coverage_path} (resolution coverage: {pct_str})")
    if vuln_matches is None:
        print(f"Wrote {findings_path} (no vuln-matches.json supplied - no findings data)")
    else:
        print(f"Wrote {findings_path}")

    if args.pdf:
        pdf_path = out_dir / f"{out_base}.report.pdf"
        package = inventory.get("package", {})
        package_meta = {**package, **(package.get("flexAppXml") or {})}
        render_pdf_report(
            pdf_path,
            package_name=package_name,
            package_meta=package_meta,
            coverage=coverage,
            vuln_matches=vuln_matches,
        )
        print(f"Wrote {pdf_path}")

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
    resolve_parser.add_argument(
        "--nvd-mirror",
        default=None,
        help="Path to a local nvd-mirror.json built by `mirror-nvd` - answers every CPE "
             "candidate locally with zero network calls instead of live-querying NVD "
             "(default: query the live API)",
    )
    resolve_parser.set_defaults(func=_cmd_resolve)

    mirror_parser = subparsers.add_parser(
        "mirror-nvd", help="Bulk-download the full NVD CVE dataset into a local mirror for `resolve --nvd-mirror`"
    )
    mirror_parser.add_argument("--out", required=True, help="Output directory for nvd-mirror.json")
    mirror_parser.add_argument(
        "--nvd-api-key",
        default=None,
        help="NVD API key (default: NVD_API_KEY env var). Strongly recommended - a full "
             "mirror build is ~260k+ CVEs, which takes hours at the unauthenticated 5 "
             "req/30s limit vs. tens of minutes at 50 req/30s with a key.",
    )
    mirror_parser.add_argument(
        "--modified-since-days",
        type=int,
        default=None,
        help="Incremental refresh: only fetch CVEs modified in the last N days, merging "
             "into an existing nvd-mirror.json in --out if one exists (default: full "
             "rebuild from scratch)",
    )
    mirror_parser.set_defaults(func=_cmd_mirror_nvd)

    report_parser = subparsers.add_parser(
        "report", help="Generate sbom.cdx.json + coverage-report.md + findings.md"
    )
    report_parser.add_argument("inventory", help="Path to a Stage 1 <package>.inventory.json file")
    report_parser.add_argument("--out", required=True, help="Output directory")
    report_parser.add_argument("--schema", default=None, help="Override path to inventory.schema.json")
    report_parser.add_argument(
        "--cpe-mappings", default=None, help="Override path to cpe-mappings.yaml (default: config/cpe-mappings.yaml)"
    )
    report_parser.add_argument(
        "--vuln-matches",
        default=None,
        help="Path to a <package>.vuln-matches.json from `resolve` (optional - "
             "coverage-report.md and sbom.cdx.json don't need it; findings.md does)",
    )
    report_parser.add_argument(
        "--pdf",
        action="store_true",
        help="Also write <package>.report.pdf - a single polished document combining "
             "the coverage summary and vulnerability findings, for a human reader "
             "rather than another tool",
    )
    report_parser.set_defaults(func=_cmd_report)

    return parser


def main(argv: list[str] | None = None) -> int:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")
    parser = build_parser()
    args = parser.parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
