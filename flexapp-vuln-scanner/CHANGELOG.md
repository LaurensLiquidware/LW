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
- Top-level `README.md` and this `CHANGELOG.md`.

### Notes

- Everything above except the actual `Mount-DiskImage`/`Dismount-DiskImage`
  calls and a real FlexApp One package invocation has been functionally
  validated: real `.package.xml` samples, real jars built with the JDK's
  `jar` tool (including a naturally line-wrapped `MANIFEST.MF`), a real
  `app.asar` built with the `asar` npm package, hashes cross-checked against
  `sha256sum`, and full inventory output validated against
  `schemas/inventory.schema.json`. One bug was found and fixed this way: a
  `[string]`-typed parameter was silently coercing `$null` to `""` instead
  of preserving JSON `null` for a nested `.asar` entry's hash field.
- Stage 2 (OSV.dev matching, NVD matching, SBOM/coverage/findings reporting)
  has not been started.
