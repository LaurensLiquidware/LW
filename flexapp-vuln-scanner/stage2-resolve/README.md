# Stage 2 — resolution & vulnerability matching

Python 3.11+. Consumes a Stage 1 inventory JSON (see
`../schemas/inventory.schema.json`) and implements all three matching/
reporting steps from `PLAN.md`'s build order: **OSV.dev matching**, **NVD
2.0 CPE matching**, and **reporting** (`sbom.cdx.json`, `coverage-report.md`,
`findings.md`).

## Install

```bash
pip install -r ../requirements.txt
```

## Usage

Two subcommands, meant to run in sequence:

```bash
# 1. Query OSV.dev + NVD for matches (needs network access to both).
python -m flexapp_vuln resolve path/to/package.inventory.json --out out/

# 2. Generate the SBOM, coverage report, and findings report.
python -m flexapp_vuln report path/to/package.inventory.json --out out/ \
    --vuln-matches out/package.vuln-matches.json
```

**`report` does not require step 1 to have run.** `sbom.cdx.json` and
`coverage-report.md` are computed directly from the inventory JSON alone —
no network access needed — because the headline coverage percentage this
whole PoC exists to measure is about identity resolution, not vulnerability
matching (see `PLAN.md`'s "Goal"). Only `findings.md` needs a
`vuln-matches.json`; without `--vuln-matches`, it says so plainly rather
than rendering something that could be mistaken for "no vulnerabilities
found."

### `resolve`

Writes `out/<package>.vuln-matches.json`: every non-excluded file from the
inventory, whether a purl or CPE could be built for it, and any matches
found from OSV.dev and/or NVD (with match confidence, per `PLAN.md` - purl
matches are always `exact-purl`; CPE matches are `mapped-cpe` when
`config/cpe-mappings.yaml` has a curated override, or `heuristic` when
falling back to automatic normalization).

Flags:
- `--cache-dir DIR` (default `./cache`) — on-disk cache for both OSV.dev and
  NVD responses. Once a purl, vulnerability ID, or CPE is cached, it's never
  re-queried (per `PLAN.md`).
- `--schema PATH` — override the inventory schema location (defaults to
  `../schemas/inventory.schema.json`).
- `--cpe-mappings PATH` — override the CPE mapping table (defaults to
  `config/cpe-mappings.yaml`).
- `--nvd-api-key KEY` — NVD API key (defaults to the `NVD_API_KEY`
  environment variable; unauthenticated requests are limited to 5 per 30
  seconds vs. 50 with a key, per `PLAN.md`).

### `report`

Writes `out/<package>.sbom.cdx.json`, `out/<package>.coverage-report.md`,
and `out/<package>.findings.md`. Flags: `--schema`, `--cpe-mappings` (same
meaning as above), and `--vuln-matches PATH` (optional, see above).

`coverage-report.md` states, per `PLAN.md`'s exact definitions: total files
scanned, files excluded (by reason), candidate components (`excluded:
false`), components resolved (by method), components unresolved (as a
table you can eyeball), and the headline resolution-coverage percentage.

`sbom.cdx.json` is real CycloneDX 1.6 JSON — **validated against the
official schema** (via `cyclonedx-python-lib`'s bundled schema file and
`jsonschema.validate`, not just reasoned about). One entry per distinct
resolved component (deduplicated across files that share the same
purl/CPE), with a `purl` or `cpe` field as available, a SHA-256 hash where
available, and `flexapp-vuln:resolutionMethod`/`flexapp-vuln:confidence`
custom properties for traceability. No `licenses` field is included —
Stage 1 never captures license data, and fabricating one would be worse
than omitting it.

`findings.md` lists matched vulnerabilities, severity-ranked
(CRITICAL → HIGH → MEDIUM/MODERATE → LOW → unknown), split into two
sections: **confirmed matches** (`exact-purl`/`mapped-cpe`) and
**low-confidence matches** (`heuristic`, under an explicit "verify manually
before treating this as a confirmed finding" warning) — per `PLAN.md`'s
rule to never present a heuristic match as confirmed.

## What "matching" means

Only three of Stage 1's identity methods map cleanly onto a Package URL
(purl), which is what OSV.dev's batch API matches against:

| Identity method | purl type |
|---|---|
| `jar-pom-properties` | `pkg:maven/<groupId>/<artifactId>@<version>` |
| `node-package-json` | `pkg:npm/<name>@<version>` (scoped packages handled) |
| `python-dist-info` | `pkg:pypi/<normalized-name>@<version>` |

Everything else that's CPE-eligible — `pe-version-resource`,
`dotnet-manifest`, `string-signature`, `electron-embedded` — gets a CPE 2.3
candidate instead, resolved against NVD:

1. **`config/cpe-mappings.yaml`** is checked first — a small, hand-curated
   vendor/product → CPE override table (e.g. the `OpenSSL` string-signature
   product maps to `cpe:2.3:a:openssl:openssl:...`). A hit here is
   confidence **`mapped-cpe`**.
2. If nothing matches, an automatic heuristic normalization is used instead
   (lowercase, corporate suffixes like "Inc."/"Corporation" stripped,
   everything else collapsed to underscores). This is confidence
   **`heuristic`** — per `PLAN.md`, never presented as a confirmed finding.

`jar-manifest` (no groupId available from `MANIFEST.MF` alone) correctly
gets neither a purl nor a CPE — that's the honest answer for a Java library
resolved with lower-confidence metadata, not something worth guessing a CPE
for. A `null` purl/cpe is a real, expected outcome for some fraction of
components; it's exactly what `coverage-report.md` surfaces as
"unresolved."

## Known limitation: `api.osv.dev` / `services.nvd.nist.gov` reachability

This project was developed in a Linux environment whose network egress
policy **blocks both `api.osv.dev` and `services.nvd.nist.gov`** (confirmed
via the proxy status endpoint — the same category of block hit against
`grype.anchore.io` during an earlier, unrelated Sparks Tool audit in this
repo). Both clients (`osv_client.py`, `nvd_client.py`) are written against
each service's documented, stable public API and validated with
mocked-HTTP unit tests rather than a live call.

The `resolve` command fails clearly rather than with a raw traceback when
either host is unreachable, naming which one — see `UnreachableService` in
`cli.py`. This same failure mode is realistic beyond this dev sandbox too:
the checklist this repo's Sparks audits are held to explicitly calls out
that "many customer environments have no outbound internet," so a clear,
non-crashing message here is a real feature, not just a workaround.
Verified directly: running `resolve` against a purl-able component fails
with a clear `api.osv.dev` message; running it against only a CPE-eligible
component (bypassing OSV entirely) fails with a clear
`services.nvd.nist.gov` message instead — the right host is named in each
case. **The `report` command needs neither host** — see "Usage" above.

**Live end-to-end validation against the real OSV.dev and NVD APIs still
needs to happen in an environment where they're reachable** — that's on
you, not something I could complete from here.

## Tests

```bash
python -m pytest tests/
```

61 tests, all passing, no network required (OSV/NVD client tests mock
`requests.Session`; NVD's rate-limit tests use an injectable fake clock so
they run instantly rather than waiting real wall-clock seconds). Covers
purl/CPE construction, CPE mapping table lookup, inventory loading+schema
validation, OSV/NVD client caching/batching/rate-limiting, coverage
computation (verified against a hand-checked fixture), SBOM structure
(component dedup, purl-vs-CPE, no fabricated license data), findings
rendering (severity sort order, confirmed-vs-heuristic separation), and the
CLI's combined component-assembly logic for both subcommands.
