# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project is a proof of concept and does not yet follow semantic
versioning releases — entries are grouped under `[Unreleased]` until a first
end-to-end run against a real package justifies cutting a version.

## [Unreleased]

### Added

- `PLAN.md`: design specification for the two-stage pipeline (PowerShell
  extraction/inventory, Python resolution/matching), the Stage 1 → Stage 2
  JSON schema contract, the exact resolution-coverage definition
  (numerator/denominator), and the build order.
- `schemas/inventory.schema.json`: JSON Schema (draft 2020-12) for the
  Stage 1 → Stage 2 contract.
- Stage 1 extraction (`stage1-extract/`):
  - `Invoke-FlexAppInventory.ps1` — entry point; dispatches classic `.vhdx`
    vs. FlexApp One `.exe`/`.flexapp`, writes one inventory JSON per package.
  - `Mount-ClassicFlexApp.ps1` — read-only VHDX mount/dismount.
  - `Expand-FlexAppOne.ps1` — unwraps a FlexApp One package via the
    hardcoded `--extract <path> --skipico` (and only that — see the safety
    note in `PLAN.md`'s resolved assumption 3 on why no other flag is ever
    reachable, given the same executable can also install/uninstall the
    FlexApp service, replace, or delete packages).
  - `Read-PackageMetadataXml.ps1` — parses the `.package.xml` sidecar.
    Deliberately never reads `<Icon>`, `<license>`, or `<CallToHome>` —
    `<license>` was found (from a real sample) to carry a named individual's
    email and phone number, unrelated to the packaged application.
  - `Get-FileInventory.ps1` — file walk with streaming SHA-256 hashing and
    bounded parallelism, long-path/UNC safe.
  - `ExclusionRules.psd1` + `Test-FileExclusion` (in
    `Resolve-VersionIdentity.ps1`) — transparent path/name noise filtering
    (OS system paths, resource-only assemblies, debug symbols, fonts/icons/
    media, satellite culture resources). Excluded files are still reported
    with a reason, never silently dropped.
  - `Resolve-VersionIdentity.ps1` and `Modules/`: identity resolution in
    priority order —
    - `VersionResources.psm1`: .NET assembly manifest (checked first, via
      `System.Reflection.Metadata`, metadata-only) then Win32 version
      resource for native PE.
    - `JavaManifest.psm1`: `META-INF/maven/*/*/pom.properties` (highest
      confidence) then `MANIFEST.MF`, recursing into nested jars (fat
      jars/WARs) to arbitrary depth.
    - `NodeAsar.psm1`: `package.json` directly or inside `app.asar` (a
      from-scratch reader for Electron's Pickle-framed archive format).
    - `PythonDist.psm1`: `dist-info/METADATA` and `egg-info/PKG-INFO`.
    - `StringSignatures.psm1` + `config/string-signatures.psd1`: last-resort
      banner-string scanning for vendored native libs (OpenSSL, zlib,
      libcurl, SQLite) and Electron's embedded Chromium/Node versions.
      A `.psd1` config was used instead of the originally-planned YAML, to
      avoid adding a parsing dependency to a stage that's meant to stay
      dependency-free PowerShell.
    - Nested components (fat-jar dependencies, `app.asar` entries) surface
      as synthetic `files[]` entries using a `<real path>!/<inner path>`
      convention.
  - `stage1-extract/README.md` — usage, what each step does, known
    limitations.
- Stage 2 resolution (`stage2-resolve/`), OSV.dev and NVD/CPE matching:
  - `flexapp_vuln/inventory.py` — loads and validates a Stage 1 inventory
    JSON against `schemas/inventory.schema.json`.
  - `flexapp_vuln/normalize.py` — `build_purl` for the three OSV ecosystems
    that map cleanly (Maven via `jar-pom-properties`, npm via
    `node-package-json` including scoped packages, PyPI via
    `python-dist-info` with proper name normalization); `build_cpe_candidate`
    for native/OS methods (`pe-version-resource`, `dotnet-manifest`,
    `string-signature`, `electron-embedded`), via a curated
    `cpe-mappings.yaml` override (confidence `mapped-cpe`) or automatic
    heuristic normalization (confidence `heuristic` — corporate suffixes
    like "Inc."/"Corporation" stripped, never presented as a confirmed
    finding). `jar-manifest` (no groupId) correctly gets neither a purl nor
    a CPE.
  - `flexapp_vuln/cpe_mappings.py` — loads and looks up
    `config/cpe-mappings.yaml`.
  - `config/cpe-mappings.yaml` — curated overrides for OpenSSL, zlib,
    libcurl, SQLite (string-signature) and Electron's embedded
    Chromium/Node.js (with a note on the Chromium-vs-Chrome approximation).
  - `flexapp_vuln/confidence.py` — match confidence levels
    (`exact-purl`/`mapped-cpe`/`heuristic`) per `PLAN.md`.
  - `flexapp_vuln/osv_client.py` — OSV.dev client: batch purl→vuln-ID lookup
    (`/v1/querybatch`) then per-ID detail fetch (`/v1/vulns/{id}`), with an
    on-disk cache that's never re-queried once populated.
  - `flexapp_vuln/nvd_client.py` — NVD 2.0 client: CPE-based CVE lookup
    (`GET /rest/json/cves/2.0`), on-disk cache, and a sliding-window rate
    limiter (5 req/30s without `NVD_API_KEY`, 50 with).
  - `flexapp_vuln/cli.py` — `resolve` subcommand:
    `python -m flexapp_vuln resolve <inventory.json> --out <dir>`, writing
    `<package>.vuln-matches.json` combining both sources. Fails with a clear
    message naming which host is unreachable (`api.osv.dev` or
    `services.nvd.nist.gov`) instead of a raw traceback.
  - `requirements.txt` (top-level, pinned): `requests`, `jsonschema`,
    `packageurl-python`, `PyYAML`, `pytest`.
- Stage 2 reporting, completing the build order:
  - `flexapp_vuln/coverage.py` — computes PLAN.md's exact resolution
    coverage definition (denominator = non-excluded files, numerator = those
    with a resolved identity) from an inventory JSON alone, independent of
    OSV/NVD matching or network access, since the headline coverage number
    is about identity resolution, not vulnerability matching.
  - `flexapp_vuln/sbom.py` — builds a CycloneDX 1.6 JSON SBOM directly from
    the inventory (recomputing purl/CPE via `normalize.py`, so it never
    needs `resolve` to have run first). One deduplicated component per
    distinct purl/CPE, with a SHA-256 hash where available and
    `flexapp-vuln:resolutionMethod`/`flexapp-vuln:confidence` custom
    properties for traceability. No `licenses` field - Stage 1 never
    captures license data, and fabricating one would be worse than omitting
    it. **Validated against the real, official CycloneDX 1.6 JSON schema**
    (via `cyclonedx-python-lib`'s bundled schema file, `jsonschema.validate`).
  - `flexapp_vuln/reporting.py` — renders `coverage-report.md` (total
    files/exclusions-by-reason/resolved-by-method/unresolved table/headline
    percentage) and `findings.md` (severity-sorted, split into "confirmed"
    exact-purl/mapped-cpe vs. "low-confidence" heuristic sections under an
    explicit verify-manually warning, per PLAN.md's rule to never present a
    heuristic match as confirmed). `findings.md` says plainly when no
    vuln-matches data was supplied, rather than looking like "no
    vulnerabilities found."
  - `flexapp_vuln/cli.py` — new `report` subcommand:
    `python -m flexapp_vuln report <inventory.json> --out <dir>
    [--vuln-matches <path>]`. Writes all three outputs; `--vuln-matches` is
    optional (only `findings.md` needs it).
  - `tests/`: 61 tests total (16 new this round), no network required.
  - `stage2-resolve/README.md` and top-level `README.md`/`CHANGELOG.md`
    updated for the full three-command pipeline.

### Notes

- Stage 1: everything except the actual `Mount-DiskImage`/
  `Dismount-DiskImage` calls and a real FlexApp One package invocation has
  been functionally validated: real `.package.xml` samples, real jars built
  with the JDK's `jar` tool (including a naturally line-wrapped
  `MANIFEST.MF`), a real `app.asar` built with the `asar` npm package,
  hashes cross-checked against `sha256sum`, and full inventory output
  validated against `schemas/inventory.schema.json`. One bug was found and
  fixed this way: a `[string]`-typed parameter was silently coercing `$null`
  to `""` instead of preserving JSON `null` for a nested `.asar` entry's
  hash field.
- Stage 2: `api.osv.dev` and `services.nvd.nist.gov` are both blocked by
  this development environment's network egress policy (confirmed via the
  proxy status endpoint — the same category of block hit against
  `grype.anchore.io` during an earlier, unrelated Sparks Tool audit in this
  repo). Both clients are validated against each service's documented
  public API via mocked-HTTP tests instead of a live call, and the
  `resolve` command's graceful-failure path was confirmed against the real
  blocked network for both hosts separately. The `report` command needs
  neither host - confirmed with a real, fully-offline end-to-end run
  producing all three outputs. Live end-to-end validation of `resolve`
  against the real APIs still needs an environment where they're reachable.
- **All five build-order steps from PLAN.md are now complete.** What
  remains is real-world validation this environment can't do: a Windows
  host for Stage 1's VHDX-mounting functions, and network access for live
  OSV.dev/NVD queries.
