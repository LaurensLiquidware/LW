# Stage 2 — resolution & vulnerability matching

Python 3.11+. Consumes a Stage 1 inventory JSON (see
`../schemas/inventory.schema.json`) and currently implements the first of
three matching/reporting steps from `PLAN.md`'s build order: **OSV.dev
matching**. NVD/CPE matching and the polished `coverage-report.md`/
`findings.md`/`sbom.cdx.json` outputs are later build steps, not yet built.

## Install

```bash
pip install -r ../requirements.txt
```

## Usage

```bash
python -m flexapp_vuln resolve path/to/package.inventory.json --out out/
```

Writes `out/<package>.osv-matches.json`: every non-excluded file from the
inventory, whether a purl could be built for it, and any OSV.dev matches
found (with match confidence, per `PLAN.md` - purl matches are always
`exact-purl`).

Flags:
- `--cache-dir DIR` (default `./cache`) — on-disk cache for OSV.dev
  responses. Once a purl or vulnerability ID is cached, it's never
  re-queried (same policy `PLAN.md` specifies for NVD).
- `--schema PATH` — override the inventory schema location (defaults to
  `../schemas/inventory.schema.json`).

## What "OSV matching" means at this step

Only three of Stage 1's identity methods map cleanly onto a Package URL
(purl), which is what OSV.dev's batch API matches against:

| Identity method | purl type |
|---|---|
| `jar-pom-properties` | `pkg:maven/<groupId>/<artifactId>@<version>` |
| `node-package-json` | `pkg:npm/<name>@<version>` (scoped packages handled) |
| `python-dist-info` | `pkg:pypi/<normalized-name>@<version>` |

Everything else — `jar-manifest` (no groupId available), `pe-version-resource`,
`dotnet-manifest`, `string-signature`, `electron-embedded` — correctly gets
`purl: null` here. Those are native/OS components with no purl-expressible
ecosystem; they're deferred to the NVD/CPE matching step, not silently
dropped. A `null` purl for those methods is the honest, correct answer at
this stage, not a bug.

## Known limitation: `api.osv.dev` reachability

This project was developed in a Linux environment whose network egress
policy **blocks `api.osv.dev`** (confirmed via the proxy status endpoint —
the same category of block hit against `grype.anchore.io` during an earlier,
unrelated Sparks Tool audit in this repo). The OSV client (`osv_client.py`)
is written against OSV's documented, stable public API
(`POST /v1/querybatch`, `GET /v1/vulns/{id}`) and validated with
mocked-HTTP unit tests (`tests/test_osv_client.py`) rather than a live call.

The CLI fails clearly rather than with a raw traceback when it can't reach
`api.osv.dev` — see `_cmd_resolve`'s `requests.exceptions.RequestException`
handling in `cli.py`. This same failure mode is realistic beyond this dev
sandbox too: the checklist this repo's Sparks audits are held to explicitly
calls out that "many customer environments have no outbound internet," so a
clear, non-crashing message here is a real feature, not just a workaround.

**Live end-to-end validation against the real OSV.dev API still needs to
happen in an environment where it's reachable** — that's on you, not
something I could complete from here.

## Tests

```bash
python -m pytest tests/
```

22 tests, all passing, no network required (OSV client tests mock
`requests.Session`). Covers purl construction (Maven/npm incl. scoped
packages/PyPI name normalization/methods with no purl), inventory
loading+schema validation, OSV client caching/batching behavior, and the
CLI's component-assembly logic.
