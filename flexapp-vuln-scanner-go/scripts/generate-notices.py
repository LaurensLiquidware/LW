#!/usr/bin/env python3
"""Generates THIRD-PARTY-NOTICES.txt from bom.cdx.json — one line per
component grouped by license, so a reviewer can see at a glance what's
bundled and under what terms without cross-referencing 130 separate
upstream repos. See scripts/generate-sbom.sh (run first) and
scripts/release.sh.
"""
import json
import sys
from collections import defaultdict


def component_license(c: dict) -> str:
    licenses = c.get("licenses") or []
    if licenses:
        lic = licenses[0].get("license", {})
        return lic.get("id") or lic.get("name") or "unknown"
    evidence = c.get("evidence", {}).get("licenses") or []
    if evidence:
        lic = evidence[0].get("license", {})
        return lic.get("id") or lic.get("name") or "unknown"
    return "unknown"


def component_name(c: dict) -> str:
    group = c.get("group")
    name = c["name"]
    return f"{group}/{name}" if group else name


def main() -> None:
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} <bom.cdx.json> <output.txt>", file=sys.stderr)
        sys.exit(1)
    bom_path, out_path = sys.argv[1:3]

    with open(bom_path) as f:
        bom = json.load(f)

    by_license = defaultdict(list)
    for c in bom.get("components", []):
        by_license[component_license(c)].append((component_name(c), c.get("version", "")))

    title = "FlexApp Vulnerability and Security Scanner — Third-Party Notices"
    lines = [
        title,
        "=" * len(title),
        "",
        "This tool bundles the third-party components listed below, generated from",
        "bom.cdx.json (the accompanying CycloneDX 1.6 SBOM). Grouped by license;",
        "consult each project's own repository for the full license text.",
        "",
        "A separate note on a bundled non-code asset not covered by npm/Go module",
        "licensing: the Inter and Material Symbols webfonts used by the frontend UI",
        "(see web/frontend/src/assets/fonts/, both under the OFL-1.1 license). PDF",
        "reports are rendered with go-pdf/fpdf's built-in Helvetica -- no bundled",
        "font file.",
        "",
    ]

    for license_name in sorted(by_license):
        components = sorted(by_license[license_name])
        lines.append(f"{license_name} ({len(components)} component{'s' if len(components) != 1 else ''})")
        lines.append("-" * len(lines[-1]))
        for name, version in components:
            lines.append(f"  {name} {version}")
        lines.append("")

    with open(out_path, "w") as f:
        f.write("\n".join(lines))
        f.write("\n")

    print(f"generate-notices: wrote {sum(len(v) for v in by_license.values())} components across {len(by_license)} license groups to {out_path}")


if __name__ == "__main__":
    main()
