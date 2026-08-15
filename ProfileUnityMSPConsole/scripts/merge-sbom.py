#!/usr/bin/env python3
"""Merges a Go SBOM (cyclonedx-gomod) and an npm SBOM (cyclonedx-npm) into
one CycloneDX 1.6 document describing the whole application — a single
self-contained binary that embeds the built frontend, so its SBOM should
be one file covering both dependency trees, not two separate ones a
reviewer has to reconcile by hand. See scripts/generate-sbom.sh.
"""
import datetime
import json
import sys
import uuid


def normalize_purl(purl: str) -> str:
    """cyclonedx-gomod builds a purl's query qualifiers from a Go map, whose
    iteration order is randomized per-process -- so two runs over an
    unchanged dependency tree can emit "type=module&goos=linux&goarch=amd64"
    one time and "goarch=amd64&goos=linux&type=module" the next. That's
    non-determinism in a string CI diffs byte-for-byte, not a real change.
    Sort the qualifiers so the same component always serializes the same way.
    """
    if not purl or "?" not in purl:
        return purl
    base, _, rest = purl.partition("?")
    query, _, fragment = rest.partition("#")
    qualifiers = sorted(q for q in query.split("&") if q)
    normalized = base + "?" + "&".join(qualifiers)
    if fragment:
        normalized += "#" + fragment
    return normalized


def main() -> None:
    if len(sys.argv) != 5:
        print(f"usage: {sys.argv[0]} <go-sbom.json> <npm-sbom.json> <version> <output.json>", file=sys.stderr)
        sys.exit(1)
    go_path, npm_path, version, out_path = sys.argv[1:5]

    with open(go_path) as f:
        go_sbom = json.load(f)
    with open(npm_path) as f:
        npm_sbom = json.load(f)

    timestamp = go_sbom.get("metadata", {}).get("timestamp") or datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    merged = {
        "bomFormat": "CycloneDX",
        "specVersion": "1.6",
        "serialNumber": f"urn:uuid:{uuid.uuid4()}",
        "version": 1,
        "metadata": {
            "timestamp": timestamp,
            "component": {
                "type": "application",
                "name": "profileunity-msp-console",
                "version": version,
            },
        },
        "components": [],
    }

    seen = set()
    for src in (go_sbom, npm_sbom):
        for c in src.get("components", []):
            if "purl" in c:
                c["purl"] = normalize_purl(c["purl"])
            key = (c.get("name"), c.get("version"), c.get("purl"))
            if key in seen:
                continue
            seen.add(key)
            merged["components"].append(c)

    with open(out_path, "w") as f:
        json.dump(merged, f, indent=2)
        f.write("\n")

    print(f"merge-sbom: {len(go_sbom.get('components', []))} Go + {len(npm_sbom.get('components', []))} npm -> {len(merged['components'])} merged components")


if __name__ == "__main__":
    main()
