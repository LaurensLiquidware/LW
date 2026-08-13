# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project is a proof of concept and does not yet follow semantic
versioning releases — entries are grouped under `[Unreleased]` until a first
end-to-end run against a real package justifies cutting a version.

## [Unreleased]

### Changed

- `ExclusionRules.psd1`: renamed the `.xml` extension rule's reason from
  `dotnet-xmldoc-file` to `xml-data-file`. A live test against a real
  Notepad++ package (native C++, zero managed code - all 8 resolved
  identities are `pe-version-resource`, none `dotnet-manifest`) showed the
  same rule correctly excluding 108 files that were never .NET doc
  comments - `autoCompletion\*.xml`, `themes\*.xml`, `stylers.model.xml`,
  `langs.model.xml`. The exclusion was right both times; the old name was
  misleadingly .NET-specific for a rule that generalizes fine to any
  app's XML config/theme/autocomplete data. Same matching behavior, no
  coverage-number change.

### Fixed

- `string-signatures.psd1` (`Electron Chromium`): the bare `Chrome/X.X.X.X`
  pattern was a false-positive magnet - found live on a real Firefox scan,
  where a Chrome-User-Agent-spoofing string inside `browser\omni.ja`
  (site-compatibility overrides) matched it and wrongly attributed
  "Electron Chromium 67.0.3396.87" - and a decade of real Chrome CVEs
  going back to 2012 - to an app that isn't Chromium-based at all
  (Firefox is Gecko-based). A genuine Electron app's own default
  User-Agent always carries an adjacent `Electron/<version>` token right
  after `Chrome/` (e.g. `Chrome/91.0.4472.124 Electron/13.1.7`); tightened
  the pattern to require it, verified this still matches a real Electron
  UA string and correctly no longer matches Firefox's spoof string.

- `Resolve-VersionIdentity.ps1`: after the fix above, re-running the same
  Firefox scan surfaced a second false positive from the exact same file
  via a different signature - `omni.ja`'s arbitrary bundled text also
  matched `Electron Node.js` (`"Node.js v8.11.1"`, likely a
  remote-debugging-protocol or compatibility string), again wrongly
  attributing an embedded Node.js runtime - and its own decade of CVEs -
  to a Gecko-based app with no Node.js at all. Two different signatures
  false-positiving on the same file is evidence the file, not the
  pattern, is the problem: `.ja` (Mozilla's own resource-bundle archive
  format) is real Firefox content but never a vendored native library,
  and packing arbitrary JS/JSON/locale text makes it structurally unsafe
  for last-resort string-signature scanning regardless of which pattern
  is used. Fixed at the dispatcher level - `.ja` files short-circuit to
  a genuinely unresolved identity (not excluded; it's real content, just
  not identifiable this way) before string-signature scanning ever runs.

- `reporting.py` (`render_findings`): rendered one row per
  `(component, vulnerability)` pair with no deduplication by identity,
  unlike `sbom.py` which already dedupes components by purl/CPE. Found
  live on the first real end-to-end `resolve` run to actually surface
  matches (a real Python package): when two physical files share an
  identity (e.g. two copies of the same bundled `sqlite3.dll`), every CVE
  for that identity was rendered once per file. On that package's real
  output this meant 246 table rows for what was actually 146 distinct
  (identity, CVE) findings. Fixed by deduplicating on
  `(purl-or-cpe-or-relativePath, vulnerability id)` before rendering -
  same identity-dedup rule already applied in `sbom.py`, verified against
  the real data both before and after.

- `ExclusionRules.psd1` (`package-manager-path`): matched anything under
  `\chocolatey\`, which wrongly excluded a package's entire real payload
  when Chocolatey installs it as a "portable" package - found live on a
  real Tor Browser scan (0.0% coverage, all 242 files excluded).
  Chocolatey's `lib\<pkg>\tools\` folder is sometimes its own management
  scaffolding (MSI installers, install scripts) and sometimes *is* the
  actual installed application (`firefox.exe`, `nss3.dll`, `softokn3.dll`,
  186 real files here), depending on how that package is authored - a
  folder-wide exclude there can't tell the difference. Narrowed to only
  Chocolatey's own management subfolders
  (`\chocolatey\.chocolatey\`/`\chocolatey\extensions\`/`\chocolatey\logs\`/
  `\ChocolateyHttpCache\`, plus nested `<pkg>.extension\extensions\`
  folders found on re-checking a real Notepad++ scan), and added a
  `package-manager-file` name-pattern rule
  (`*.nuspec`/`*.nupkg`/`chocolatey*.ps1`/`*.ignore`) to still catch
  Chocolatey's own manifest/installer-script files wherever they appear,
  including inside `tools\`. Re-verified against the real Paint.NET and
  Notepad++ inventories that this is not a regression - Paint.NET's
  candidate count is unchanged (315, still 99.0%), Notepad++ gained
  exactly one legitimate candidate (a Chocolatey PATH shim,
  `\chocolatey\bin\notepad++.exe`, left unresolved rather than guessed
  at).

- `Mount-ClassicFlexApp.ps1`: was waiting on a retry loop for Windows to
  auto-assign a drive letter to the mounted VHDX volume - confirmed via a
  live test against a real package on a real Windows 11 host that Windows
  does not reliably do this (the disk/partition come online healthy, just
  without a letter, no matter how long you wait). Now mounts to a scratch
  folder via `Add-PartitionAccessPath` instead, which is deterministic and
  also avoids drive-letter exhaustion on a host scanning many packages.
  `Dismount-ClassicFlexApp`/`Invoke-FlexAppInventory.ps1` updated to clean
  up the access path and scratch folder accordingly.
- `NodeAsar.psm1` (`Get-NodePackageIdentity`, `Get-AsarPackageIdentities`):
  some of OBS Studio's own internal plugin `package.json` files use a bare
  numeric version (`"version": 9`) instead of a string - `ConvertFrom-Json`
  returned that as a PowerShell int, which was passed straight through into
  the identity object, violating the inventory schema's "version is
  string|null" contract and failing `jsonschema.validate` on the Stage 2
  side (caught live: `report` failed on a real Stage-1-produced inventory
  JSON with "9 is not of type 'string', 'null'"). Fixed with a
  `ConvertTo-NullableString` helper, guarding the same
  `$null`-becomes-`""` PowerShell coercion gotcha found earlier.
- `sbom.py`: package.json-derived components with no name field produced
  `"name": null`, which is invalid per the real CycloneDX 1.6 schema
  (confirmed against the official schema file via `jsonschema.validate` on
  the user's actual generated SBOM). Fixed by skipping nameless resolved
  identities when building SBOM components - they still count as
  "resolved" for `coverage.py`'s purposes.
- `normalize.py`'s `_escape_cpe_component`: only escaped backslash and
  colon, on the wrong assumption that vendor/product/version strings were
  "already alnum/underscore/dot by construction." Real Win32
  `ProductVersion` strings disproved that live (`x264`'s
  `"0.164.3106 eaa68fa"`, ANGLE's `"2.1.23296 git hash: e323abb5b08e"`)
  reached the CPE string with raw, unescaped spaces, producing invalid
  CPE 2.3 formatted strings. Replaced with a full CPE 2.3 special-character
  escaper (NIST IR 7695 §6.1.2.4's reserved set).
- `cpe_mappings.py`: `find()` required an exact `method` match, so a real
  `zlib` Win32 version resource (method `pe-version-resource`) missed the
  existing `cpe-mappings.yaml` entry for zlib, which was scoped to
  `method: string-signature`. Made `method` optional in a mapping entry's
  `match` block - a distinctive product name is unambiguous regardless of
  which method found it.
- `VersionResources.psm1` (`Get-DotNetAssemblyIdentity`): every managed
  .NET assembly was silently falling back to `pe-version-resource` instead
  of resolving via `dotnet-manifest` - confirmed live on a real Paint.NET
  package where well-known managed libraries (Json.NET, Mono.Cecil,
  ComputeSharp.Core) all resolved with `raw` shaped like a Win32 version
  resource, never an assembly manifest. Root cause: `GetMetadataReader()`
  is an extension method on the static class
  `System.Reflection.Metadata.PEReaderExtensions`, not an instance method
  on `PEReader` - PowerShell's method binder doesn't resolve extension
  methods via `$peReader.GetMetadataReader()` the way the C# compiler
  does, so the call threw on every single invocation and was swallowed by
  the catch-all. Fixed by calling it as a static method,
  `[System.Reflection.Metadata.PEReaderExtensions]::GetMetadataReader($peReader)`.
  Reproduced and verified directly against a real assembly
  (`Newtonsoft.Json.dll`) before and after the fix.
- `nvd_client.py` (`NVDClient.query_cpe`): a CPE with no matching entry in
  NVD's CPE dictionary returns HTTP 404 - documented NVD 2.0 API behavior,
  not a connectivity problem - but `cli.py`'s `except
  requests.exceptions.RequestException` catches `HTTPError` too, so it was
  misreported as "could not reach services.nvd.nist.gov" and aborted the
  whole `resolve` run on the first non-matching CPE (hit live: a
  `.NET`-runtime-assembly CPE candidate built from raw build metadata,
  e.g. `9,0,1326,6317 @Commit: ...`, that was never going to match any
  real NVD dictionary entry). Fixed by treating a 404 response as "no
  CVEs known for this CPE" (`{"vulnerabilities": []}`, cached like any
  other result), leaving genuine connectivity failures (timeouts, DNS
  errors, 5xx) still raised and reported as unreachable.
- `nvd_client.py` (`NVDClient.query_cpe`): a 429 (Too Many Requests) also
  aborted the whole `resolve` run as if the host were unreachable - hit
  live by re-running `resolve` shortly after a prior invocation, since the
  sliding-window rate limiter only tracks requests made within the current
  process and has no memory of a previous run's requests still counted
  against NVD's server-side window. A 429 is a real, recoverable
  rate-limit signal, not a dead host. Fixed with retry-with-backoff (up to
  5 attempts, honoring a `Retry-After` header when NVD sends one,
  otherwise waiting the full 30s window) before giving up.

### Added

- `ExclusionRules.psd1`: added `flexapp-capture-scaffolding`
  (`\Data\appinstall.cap`/`\Data\printers.bak`/`\Data\DisableShortPaths`/
  `\Data\Suppress.ACL`) after these exact 4 filenames showed up in a
  top-level `\Data\` folder (sibling to `\Volumes\C\...`, not inside it)
  on all 9 real packages tested so far - FlexApp/ProfileUnity's own
  package-capture scaffolding, never app content, always unresolved.
  Scoped to the exact folder+filename combination rather than a bare
  filename or folder-name match, so a real app's own internal folder
  that happens to be named "data" (e.g. OBS Studio's own
  `obs-studio\data\obs-studio\...`) is never touched. Verified against 5
  real inventories that nothing newly-excluded ever had a resolved
  identity.

- `cpe-mappings.yaml`: added FFmpeg, Qt, and Chromium as curated
  `mapped-cpe` overrides, and switched the existing `Electron Chromium`
  entry from a `google:chrome` approximation to the same real Chromium
  CPE. Each was checked against the real NVD CPE dictionary (via web
  search, since `nvd.nist.gov` itself is blocked from direct fetch in this
  dev environment, same as `services.nvd.nist.gov`) before adding -
  confirmed real, dedicated entries: `cpe:2.3:a:ffmpeg:ffmpeg`,
  `cpe:2.3:a:qt:qt` (not `qt6` - Qt5 and Qt6 share the same NVD product),
  `cpe:2.3:a:chromium:chromium` (distinct from Google Chrome). Checked and
  deliberately did NOT add mappings for OBS Studio, x264, CEF, or ANGLE -
  none have a dedicated NVD CPE entry to match against (OBS Studio's own
  product doesn't appear in the dictionary at all; ANGLE/CEF bugs surface
  as Chromium CVEs instead of their own).

  Added a version-transform mechanism (`cpe_mappings.py`'s
  `find_version_transform`, an optional `versionPattern`/`versionGroup`
  on a mapping entry) because a vendor/product fix alone wasn't enough:
  FFmpeg's detected version carries its own git-tag convention
  (`"n7.1.1"`, leading `n`) and Qt's carries a Win32 FILEVERSION
  resource's 4-part format (`"6.8.3.0"`) - neither matches NVD's plain
  3-part dictionary format. A version that doesn't fit the expected shape
  falls back to the raw value unchanged rather than raising or mangling
  the CPE. Verified against real OBS Studio and Chromium data that all
  three produce clean, correctly-shaped CPE strings
  (`cpe:2.3:a:ffmpeg:ffmpeg:7.1.1:...`, `cpe:2.3:a:qt:qt:6.8.3:...`,
  `cpe:2.3:a:chromium:chromium:147.0.7727.102:...`).

- `nvd_mirror.py` + `mirror-nvd` CLI subcommand + `resolve --nvd-mirror`:
  a local NVD CVE mirror, to answer scale concerns raised live once the
  429-retry fix above made a real timing problem visible - without an
  API key, live-querying NVD per CPE candidate at 5 req/30s means a
  single few-hundred-component package takes 20-30+ minutes, which
  doesn't hold up across many customer packages. NVD retired its
  downloadable JSON/XML CVE feed files in December 2023 in favor of an
  API-only model, so this bulk-paginates the same 2.0 API once (no
  `cpeName` filter, `resultsPerPage=2000`) into a local index built from
  each CVE's real CPE match criteria (including version ranges), then
  matches locally with zero further network calls. `--modified-since-days`
  supports a cheaper incremental refresh that merges into an existing
  mirror rather than rebuilding from scratch. Version-range matching uses
  a new best-effort tokenized comparator (`version_compare.py`,
  RPM/dpkg-style) - documented as non-authoritative, same spirit as this
  project's `heuristic` confidence tier. The live per-CPE `resolve` path
  is unchanged and remains the default; the mirror is opt-in.
- `ExclusionRules.psd1`: added `report-design-file` (`*.jasper*`
  wildcarded on both sides) and `report-design-folder` (`\TClickRapporten\`),
  found live on a real Jonker ERP package (Remix-H1-DROOG): 868 of 968
  "unresolved" files (90%) were JasperReports compiled report designs and
  their locale-variant `.properties` sidecars, all under one folder - not
  components. The wildcarded name pattern also catches date-stamped
  backup copies found live (`Bon.jasper.2013-07-18`,
  `Bon.jasper_20221129`) that a plain `.jasper` extension rule would miss,
  since `GetExtension()` only returns the last dot-segment. Verified
  against the real inventory JSON that none of the 868 newly-excluded
  files had a resolved identity - moved that package's honest resolution
  coverage from 8.9% to **48.7%**.
- `ExclusionRules.psd1`: added three more categories from the same
  Remix-H1-DROOG package's remaining unresolved files: `changelog-folder`
  (`\Wijzigingen\` - Dutch for "changes", plain-text release notes, 35
  files), `log-folder` (`\Log\<module>\`, runtime log output, 28 files),
  and `jre-telemetry` (`\.oracle_jre_usage\`, Oracle's own JRE
  usage-tracking files - not part of the packaged app at all, an artifact
  of any machine that's ever run a JRE, 2 files). Verified against the
  real inventory JSON that none had a resolved identity - moved that
  package's honest resolution coverage from 48.7% to **73.1%**.
- `ExclusionRules.psd1`: added `.ini`/`.pak`/`.effect` as noise categories
  (`config-file`/`resource-pack-file`/`shader-effect-file`), found by a live
  test against a real OBS Studio package - 90% of that package's
  "unresolved" files were `.ini` config/locale files alone, none of them
  third-party components. Moved that package's honest resolution coverage
  from 3.7% to 53.0%.
- `ExclusionRules.psd1`: added `\Lang\`/`\Locale\`/`\Locales\`/`\i18n\`/
  `\l10n\` as a new `localization-file` path rule, found by a second live
  test against a real 7-Zip package - 92% of that package's "unresolved"
  files were per-language translation `.txt` files under a `\Lang\`
  folder. Moved that package's honest resolution coverage from 5.6% to
  42.9%.
- `ExclusionRules.psd1`: added four more noise categories from the
  Paint.NET live test, after the `dotnet-manifest` fix made the real
  unresolved-files list visible for the first time - that package was
  installed via Chocolatey on the capture machine, so Chocolatey's own
  package-manager footprint (nuspec/nupkg, install-state sentinel files,
  cached HTTP API responses, logs, helper scripts) got swept into the
  VHDX alongside the app itself: `package-manager-path`
  (`\chocolatey\`/`\ChocolateyHttpCache\`, ~65 files),
  `dotnet-xmldoc-file` (`.xml` IntelliSense doc-comment files paired 1:1
  with each assembly, 16 files), `dotnet-resource-data`
  (`.resources`/`.resx` uncompiled localization data, 38 files),
  `readme-license-text` (bundled-plugin `License.txt`/`Readme.txt`/
  `Third Party Notices.txt`/`VERIFICATION.txt`, 12 files), and
  `shell-shortcut` (`.lnk`, 1 file). Verified against the real inventory
  JSON that none of the 126 newly-excluded files had ever had a resolved
  identity - moved that package's honest resolution coverage from 70.6%
  to **98.7%**.
- `ExclusionRules.psd1`: added `dotnet-runtimeconfig` (`*.runtimeconfig.json`
  - pure .NET runtime target-framework config, never a component). Moved
  that package's coverage from 98.7% to 99.0% (312/315). Deliberately did
  NOT exclude the sibling `*.deps.json` - see PLAN.md's new "candidate
  follow-up" section: it's a real dependency lockfile worth parsing for
  identity resolution, not noise to discard.
- `cpe-mappings.yaml`: added a second `curl` mapping entry for
  `"The curl library"` - curl.dll's actual Win32 `ProductName` (confirmed
  live), distinct from the string-signature path's `"libcurl"` label.
  Deliberately did NOT add mappings for OBS Studio/FFmpeg/Qt6/x264/
  Chromium/ANGLE/CEF (seen fragmenting into multiple CPEs across binaries
  with different vendor-string variants, e.g. "OBS Project" vs. "OBS") -
  guessing their canonical NVD CPE dictionary vendor string without live
  verification would present a guess as `mapped-cpe` confidence, no more
  trustworthy than `heuristic` but labeled as if it were.

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
- **All five build-order steps from PLAN.md are now complete, and both
  Stage 1 and Stage 2's `report` command have been live-validated end to
  end against two different real packages** on a real Windows host - OBS
  Studio (classic VHDX and FlexApp One, byte-identical results, 2170
  files, 53.0% coverage after fixes) and 7-Zip (112 files, 42.9% coverage
  after fixes). Reviewing the actual generated output files (not just
  summary numbers) from both runs caught five real bugs total, all fixed
  and re-verified against the user's real files - see the "Fixed"/"Added"
  entries above. What remains is network access for live OSV.dev/NVD
  queries in the `resolve` command, which this dev environment still
  can't provide.
