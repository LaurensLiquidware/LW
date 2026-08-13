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
- Stage 2 resolution (`stage2-resolve/`), OSV.dev matching only so far:
  - `flexapp_vuln/inventory.py` — loads and validates a Stage 1 inventory
    JSON against `schemas/inventory.schema.json`.
  - `flexapp_vuln/normalize.py` — builds a Package URL (purl) from a Stage 1
    identity, for the three ecosystems that map cleanly (Maven via
    `jar-pom-properties`, npm via `node-package-json` including scoped
    packages, PyPI via `python-dist-info` with proper name normalization).
    Everything else (native PE, `jar-manifest` with no groupId,
    string-signature/electron-embedded) correctly returns no purl — deferred
    to the NVD/CPE matching step, not silently dropped.
  - `flexapp_vuln/confidence.py` — match confidence levels
    (`exact-purl`/`mapped-cpe`/`heuristic`) per `PLAN.md`.
  - `flexapp_vuln/osv_client.py` — OSV.dev client: batch purl→vuln-ID lookup
    (`/v1/querybatch`) then per-ID detail fetch (`/v1/vulns/{id}`), with an
    on-disk cache that's never re-queried once populated.
  - `flexapp_vuln/cli.py` — `python -m flexapp_vuln resolve <inventory.json>
    --out <dir>`, writing `<package>.osv-matches.json`. Fails with a clear
    message (not a raw traceback) when `api.osv.dev` is unreachable.
  - `tests/`: 22 tests, no network required — purl construction, inventory
    schema validation, OSV client caching/batching (mocked `requests`), and
    CLI component assembly.
  - `requirements.txt` (top-level, pinned): `requests`, `jsonschema`,
    `packageurl-python`, `pytest`.
  - `stage2-resolve/README.md` — usage, what "OSV matching" covers at this
    step, and the `api.osv.dev` reachability caveat below.
- Top-level `README.md` and this `CHANGELOG.md`.

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
- Stage 2: `api.osv.dev` is blocked by this development environment's
  network egress policy (confirmed via the proxy status endpoint — the same
  category of block hit against `grype.anchore.io` during an earlier,
  unrelated Sparks Tool audit in this repo). The OSV client is validated
  against OSV's documented public API via mocked-HTTP tests instead of a
  live call. Live end-to-end validation against the real API still needs an
  environment where it's reachable.
- NVD/CPE matching and the reporting step (`sbom.cdx.json`,
  `coverage-report.md`, `findings.md`) have not been started.
