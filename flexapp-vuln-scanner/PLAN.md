# PLAN.md — FlexApp Package Composition & Vulnerability PoC

Status: **DRAFT — awaiting confirmation before any code is written.**

## Goal (restated)

Answer one question honestly: for a real enterprise FlexApp package, what
percentage of the third-party components inside it can be resolved to a
version identity precise enough to match against a vulnerability database?
Everything below is designed to produce that number defensibly, not to
produce a polished report.

## Module structure

```
flexapp-vuln-scanner/
  stage1-extract/                      # PowerShell 7, runs on/near the package store
    Invoke-FlexAppInventory.ps1        # entry point: dispatches by extension
    Mount-ClassicFlexApp.ps1           # Mount-DiskImage -ReadOnly + guaranteed dismount
    Expand-FlexAppOne.ps1              # runs <Package>.exe --extract --skipico into a temp dir,
                                        # then hands the extracted .vhdx + .xml to the same
                                        # classic-mount path — see resolved assumption 3 below
    Get-FileInventory.ps1              # filesystem walk -> per-file record
    Resolve-VersionIdentity.ps1        # dispatch table: PE / .NET / jar / node / python / string-scan
    Read-PackageMetadataXml.ps1        # FlexApp package XML parser
    ExclusionRules.psd1                # noise-filter rules (path/name heuristics), inspectable
    Modules/
      VersionResources.psm1            # FileVersionInfo + .NET assembly manifest reading
      JavaManifest.psm1                # MANIFEST.MF / pom.properties / nested-jar recursion
      NodeAsar.psm1                    # package.json incl. inside app.asar; Electron string scan
      PythonDist.psm1                  # dist-info/METADATA, egg-info/PKG-INFO
      StringSignatures.psm1            # banner-pattern scan, patterns loaded from config
    config/
      string-signatures.yaml           # OpenSSL/zlib/libcurl/SQLite banner regexes (editable, not hardcoded)
    tests/
      *.Tests.ps1                      # Pester tests, incl. synthetic fixtures for weird-filesystem cases

  stage2-resolve/                      # Python 3.11+, consumes stage 1 JSON
    flexapp_vuln/
      __init__.py
      inventory.py                     # load + validate stage1 JSON (jsonschema)
      normalize.py                     # vendor/product string normalization -> candidate CPE/purl
      cpe_mappings.py                  # loads cpe-mappings.yaml overrides
      osv_client.py                    # OSV.dev batch query (purl-based), on-disk cache
      nvd_client.py                    # NVD 2.0 CPE query, rate-limit aware, on-disk cache, NVD_API_KEY
      confidence.py                    # match confidence levels: exact-purl / mapped-cpe / heuristic
      sbom.py                          # CycloneDX 1.6 SBOM builder
      coverage.py                      # resolution coverage computation + breakdown by method
      reporting.py                     # coverage-report.md and findings.md renderers
      cli.py                           # `flexapp-vuln resolve <inventory.json> --out <dir>`
    config/
      cpe-mappings.yaml                # manual vendor/product -> CPE overrides
    cache/                             # on-disk NVD/OSV response cache (gitignored)
    tests/
      test_*.py                        # pytest, incl. fixtures with synthetic inventory JSON

  schemas/
    inventory.schema.json              # JSON Schema for the stage1 -> stage2 contract

  README.md
  CHANGELOG.md
  requirements.txt                     # pinned, stage 2 only (stage 1 has no external deps beyond flexappone.exe)
  .gitignore                           # cache/, *.vhdx, extracted temp dirs, etc.
```

Rationale: stage 1 stays dependency-free PowerShell (no external modules to
install on a production package-store host); stage 2 is a normal pip-installable
package so it's easy to iterate on and unit test off-box using saved JSON
fixtures, without needing Windows or real packages.

## JSON schema: stage 1 → stage 2 contract

Top-level `inventory.schema.json` (informal shape shown; real file will be
strict JSON Schema draft 2020-12):

```jsonc
{
  "schemaVersion": "1.0",
  "package": {
    "sourcePath": "D:\\FlexAppShare\\Chrome_120.vhdx",
    "packageType": "classic-vhdx | flexapp-one",
    "flexAppXml": {                     // null if no sidecar .package.xml found; real schema confirmed 2026-08-12 from a live winscp_*.package.xml sample
      "uuid": "37d9eb95-78ae-4ceb-a8fa-77be405c4fe8",
      "displayName": "winscp",          // raw capture-folder name, NOT a clean product name — do not treat as normalized
      "packageType": "Vhd",             // classic FlexApp; FlexApp One packages will need a sample to confirm this field's value
      "sizeInGb": 10,
      "actualSizeInBytes": 306184192,
      "dateCreated": "2026-07-30T16:06:52.2678033+02:00",
      "dateModified": "2026-07-30T16:09:34.3895346+02:00",
      "historyRaw": [                   // free-text audit trail, NOT structured fields — see note below
        "2026-07-30T16:09:34.3895346+02:00",
        "   create package by admin.lvh",
        "   Version 6.9.5.9678, 6.9.5.9678 Wed 07/01/2026+2c00aca1f87b2c3f690ec2e94a38fcd923ab779e"
      ],
      "versionMajorMinorBuildRevision": null,   // was 0.0.0.0 in the sample — present in the schema but not populated; do not rely on it
      "shortcutTargets": [               // from <Links><Link><Target> — the actual installed executable path(s)
        "C:\\Program Files (x86)\\WinSCP\\WinSCP.exe"
      ],
      "installerIds": [                  // regex-extracted from <Installers>, e.g. winget "--id X" — omitted if none found
        "OBSProject.OBSStudio"
      ]
      // <Icon>, <license>, <CallToHome> are read but deliberately NEVER copied into this JSON —
      // Icon is a large irrelevant image blob; <license> carries a named Liquidware contact's
      // email/phone (PII unrelated to the packaged app) and a signature/serial. Both are dropped
      // at the parse boundary, not filtered later, so they can never leak downstream.
    },
    "scanStartedUtc": "2026-08-12T18:00:00Z",
    "scanFinishedUtc": "2026-08-12T18:04:12Z",
    "toolVersion": "0.1.0"
  },
  "files": [
    {
      "relativePath": "Program Files\\Google\\Chrome\\Application\\120.0.6099.129\\chrome.dll",
      "sizeBytes": 123456789,
      "sha256": "…",
      "excluded": false,
      "exclusionReason": null,          // set + excluded=true when filtered (see ExclusionRules.psd1)
      "componentType": "pe-native | dotnet-assembly | jar | node-package | python-dist | native-string-scan | unknown",
      "identity": {                     // null if unresolved
        "method": "pe-version-resource | dotnet-manifest | jar-pom-properties | jar-manifest | node-package-json | electron-embedded | python-dist-info | string-signature",
        "vendor": "Google Inc.",
        "product": "Google Chrome",
        "version": "120.0.6099.129",
        "raw": { }                      // method-specific raw fields kept for audit/debug
      },
      "readError": null                 // populated instead of crashing, e.g. "access denied", "zero-byte file"
    }
  ]
}
```

Every file that was walked appears exactly once in `files[]`, whether
excluded, resolved, or unresolved. Stage 2 never re-derives inclusion/exclusion
— stage 1 already decided it, transparently, via `exclusionReason`.

## Definition of "resolution coverage"

This is the number the whole PoC exists to produce, so pinning down
numerator/denominator precisely:

- **Denominator — "candidate components"**: files with `excluded: false`
  AND `componentType != "unknown"` is *not* how we'll count it, because that
  would let us shrink the denominator by classifying hard files as unknown.
  Instead: denominator = every file with `excluded: false`. If stage 1 saw it,
  didn't filter it as noise, and it's a real file (not a directory), it is a
  candidate component and must be accounted for.
- **Numerator — "resolved"**: files with `excluded: false` AND
  `identity != null` (i.e., some method produced vendor/product/version,
  regardless of confidence level). Confidence (exact-purl / mapped-cpe /
  heuristic) is tracked separately for the *vulnerability-matching* stage,
  not for the coverage percentage — resolving "OpenSSL 1.1.1w" via string
  scan still counts as resolved identity even though the later CPE match
  against it will be heuristic-confidence.
- **Unresolved**: `excluded: false` AND `identity == null`. These go in
  `coverage-report.md`'s unresolved table with `readError` surfaced if that's
  why (locked/zero-byte/permission), and componentType if known but the
  specific method failed (e.g., a `.dll` with no version resource at all).
- **Excluded files** are reported (count + reason breakdown) but are outside
  both numerator and denominator — they're not claimed to be "coverage" in
  either direction, since the whole point is not hiding hard cases inside
  the excluded bucket. This is why the exclusion ruleset must stay small,
  in a **inspectable** config, and biased toward "well-known OS/noise file"
  rather than "any file we don't know how to parse."
- **Headline number** = numerator / denominator, reported per-package and
  also as file-count vs distinct-component-count if those differ meaningfully
  (a component can appear as many files — resource DLLs referencing the same
  product/version — planned as a stretch aggregation view, not required for v1).

Per-method breakdown (numerator split by `identity.method`) is reported
alongside the headline number, since the task explicitly wants the hit rate
per method as a finding in its own right.

## Assumptions about FlexApp internals — please verify before I build

1. **Classic FlexApp (.vhdx) layout**: I'm assuming the VHDX contains a
   standard NTFS volume with the captured application installed under a
   `Program Files` / `Program Files (x86)` / app-specific root, mountable
   read-only with `Mount-DiskImage -Access ReadOnly` on Windows 10/11 or
   Server, no BitLocker/encryption on the VHDX itself. Confirm this holds for
   your real test packages, and tell me if any are encrypted or require a
   specific FlexApp/ProfileUnity component installed to mount.
2. **FlexApp package XML metadata — RESOLVED**, confirmed from a real
   sample (`winscp_20260730160821.package.xml`) on 2026-08-12:
   - It's a sidecar file named `<vhdx-basename>.package.xml`, sitting next to
     the `.vhdx` in the same folder (`<FilePath>\\ld-lw01\apps\FlexApp\winscp_20260730160821`,
     `<FileName>winscp_20260730160821.vhdx`) — a folder-per-package UNC share
     layout. `Read-PackageMetadataXml.ps1` will look for `<vhdx-name>.package.xml`
     next to the given `.vhdx` path.
   - Real element names: `Uuid`, `DisplayName`, `PackageType` (`Vhd` for
     classic FlexApp — still need a FlexApp One sample to confirm its value),
     `FilePath`/`FileName`, `SizeInGb`/`ActualSizeInBytes`, `DateCreated`/
     `DateModified`, `History` (array of loose strings), `Links` (array of
     shortcut descriptors), `VersionMajor`/`Minor`/`Build`/`Revision`.
   - **`DisplayName` is not a clean product name** — it's the raw capture
     folder name (`winscp`, lowercase). Don't feed it to CPE/purl matching
     without normalization; it's closer to a slug than `<ProductName>`.
   - **`VersionMajor`/`Minor`/`Build`/`Revision` are populated inconsistently
     across packages** — all `0` in the winscp sample, but a second sample
     (`OBSStudio_20260625190629.package.xml`, also `PackageType: Vhd`) has
     `32.1.2.0`, which correctly matches OBS Studio's real version. Revised
     rule: use these fields when non-zero (treat as a package-level version
     signal), fall back to regex-extracting `History` when they're all zero.
     Neither is fully dependable alone — this is exactly the kind of
     per-method hit-rate variance the brief wants tracked, so record which
     path resolved it (`xml-version-fields` vs. `xml-history-regex`) in the
     `raw` field like any other identity method.
   - **`History`'s format is not consistent between packages either** — the
     winscp sample has 3 loose strings including a `Version X.Y.Z.W` line;
     the OBS sample has one string, `"Package created on:06/25/2026 19:07
     by:DESKTOP-PPDVIVG\Install with:1.6.1.9217"`, which contains no app
     version at all — `1.6.1.9217` there is the FlexApp packaging console's
     own version, not OBS Studio's. The regex extractor needs to specifically
     match a `Version ([\d.]+)` pattern and not just grab the first
     version-shaped number in the string, or it will misattribute the
     packaging tool's version to the packaged app.
   - **`Installers` is a high-value field when present**: the OBS sample has
     `cmd.exe /c winget install --id OBSProject.OBSStudio --silent ...`. A
     `--id <PackageID>` extracted from a winget/choco/msiexec-style install
     command is a clean, structured identifier — arguably better than
     anything derivable from `DisplayName`. Plan: add an `xml-installer-id`
     resolution signal that regex-extracts `--id ([\w.-]+)` (winget) or
     equivalent patterns for other package managers, used the same way as
     the `Links[].Target` signal — package-level corroboration for the
     primary component's identity, not a per-file resolution method.
   - **`Links[].Target` remains the most useful field for pinpointing the
     primary component**: absolute installed executable path(s)
     (`C:\Program Files\obs-studio\bin\64bit\obs64.exe` in the OBS sample).
     Captured as `shortcutTargets`, as before.
   - **`Icon` is a large embedded base64 PNG** — irrelevant to component
     resolution. `Read-PackageMetadataXml.ps1` will explicitly skip/drop this
     field rather than pass it through to the JSON inventory.
   - **New finding, not in the original plan — `<license>` block contains
     real PII and must never be emitted.** The OBS sample's `.package.xml`
     includes a `<license>` element with Liquidware's own FlexApp One
     product-license metadata: `contactName`, `contactEmail`,
     `contactNumber` (a named individual's email and phone number), plus a
     license `signature` blob and `serial`. This has nothing to do with the
     packaged application (OBS Studio) — it's licensing metadata for the
     FlexApp One packaging tool itself, sitting in the same sidecar file.
     **`Read-PackageMetadataXml.ps1` must explicitly exclude `<license>` and
     `<CallToHome>` from anything written to the inventory JSON** — same
     treatment as `<Icon>`, but for a privacy reason rather than a size
     reason. This needs to be called out plainly since the whole point of
     stage 1 is "nothing leaves the machine except this JSON," and right now
     the source XML would otherwise carry a person's contact details straight
     into that JSON.
   - Confirmed both samples are `PackageType: Vhd` (classic FlexApp) — still
     no confirmation on whether FlexApp One packages use the same
     `.package.xml` sidecar format; see point 3 below, which resolves the
     FlexApp One extraction question a different way (self-extracting exe),
     which may mean FlexApp One doesn't produce this sidecar at all.
3. **FlexApp One (.flexapp/.exe) format — RESOLVED**, from the doc text you
   pasted (2026-08-12, "Package Extraction & Inspection" section) plus your
   `OBS-Studio.exe --extract C:\FA1` test. This is a bigger simplification
   than I expected:
   - There's no separate `flexappone.exe` tool. **Each FlexApp One package
     is itself a self-extracting executable** (`<PackageName>.exe`) built by
     the FlexApp Packaging Console.
   - `--extract <path>` pulls out exactly **VHDX, ICO, and XML** — i.e. a
     FlexApp One package is a classic FlexApp capture (VHDX + `.package.xml`
     sidecar) plus an icon, wrapped in a self-extracting shell. This answers
     the open question from point 2 above: **it's very likely the same
     `.package.xml` schema**, since the doc says the extracted XML can be
     edited "in the Packaging Console" and re-imported to build a new
     package — that's describing round-tripping the same authoring format
     already parsed in point 2, not a distinct FlexApp One-only schema. I'll
     confirm this the moment a real extracted sample is available, but it
     changes the plan now rather than waiting: `Read-PackageMetadataXml.ps1`
     does not need a second branch for FlexApp One — one parser, two entry
     paths.
   - **Pipeline simplification**: `Expand-FlexAppOne.ps1` runs
     `& $PackagePath --extract $TempDir --skipico` (the target path must
     already exist — the doc is explicit about this, so the wrapper creates
     it first) to skip the icon we don't need, finds the resulting `.vhdx`
     + `.xml` in `$TempDir`, and hands them to the *same*
     `Mount-ClassicFlexApp.ps1` + `Read-PackageMetadataXml.ps1` path used for
     a classic `.vhdx` input. FlexApp One stops being a separate extraction
     strategy and becomes "one extra unwrap step before the classic path" —
     `Invoke-FlexAppInventory.ps1`'s dispatch is now: `.exe`/`.flexapp` →
     unwrap via `--extract --skipico` → fall into the same `.vhdx` handling
     as a direct classic input.
   - Noted but not used for v1: `--skipdisk` (XML/ICO only, no VHDX — not
     useful here since the VHDX is exactly what we need to scan),
     `--skipxml` (opposite of what we need), `--mapicons` (per-shortcut
     icons — irrelevant), `--diskpath` (overrides the default VHDX *mount*
     path, `C:\ProgramData\Liquidware\Flexapp\Shadows` — relevant to
     `Mount-ClassicFlexApp.ps1` if that default path ever has insufficient
     space on a scanning host, worth a config knob but not a v1 requirement).
   - **Exit codes are unconfirmed** — you don't have that documented either.
     `Expand-FlexAppOne.ps1` will capture the process's stdout/stderr and
     actual exit code for the log regardless, but the real failure signal is
     checking whether `$TempDir` actually contains a `.vhdx` and `.xml`
     after the call — presence/absence of the expected output files, not the
     exit code, is what decides success in the wrapper. If a 0/non-zero
     contract turns out to be reliable once we're testing against real
     packages, I'll tighten this, but the file-presence check is the safe
     default given an undocumented contract.
   - **Safety constraint — this executable is far more than an extractor,
     and the wrapper must never let it be anything else.** The full CLI
     reference you pasted shows the *same* `<PackageName>.exe` also
     installs/uninstalls the FlexApp service and driver (`--install`,
     `--uninstall`, requires elevation), mounts and launches the packaged
     app (`--index`), deletes the package from disk (`--remove`), replaces a
     running package with a new version (`--replace`), and several other
     state-changing operations. `Expand-FlexAppOne.ps1` will invoke this
     executable with **exactly and only** `--extract <path> --skipico` —
     the argument list is hardcoded in the wrapper, never built from
     concatenation, never accepts pass-through arguments, and there is no
     code path anywhere in this project that constructs any other flag
     combination. This is a stage-1 hard rule, not a suggestion: the doc's
     own "non-destructive" callout applies specifically to `--extract`, and
     only to `--extract` — the same binary has an entire other mode that is
     very much destructive (`--stop`, `--clean`, `--remove`), and mounting
     read-only inventory tooling must never be able to drift into calling
     any of it, even accidentally.
   - Also present in the reference but irrelevant to this project and not
     wired up anywhere: `--admin`, `--install`/`--upgrade`/`--uninstall`,
     `--startup`, `--index`/`--ctl`/`--addtostart`, `--stop`/`--replace`/
     `--clean`/`--remove`, `--sync`, `--reg`/`--system`, `--skipactivation`/
     `--assoc`, `--persist`, `--sessionisolate`, `--outofband`, and the
     runtime-tuning flags (`--blockcachesize`, `--authtimeout`,
     `--linktimeout`, `--priorityboost`). `--printshortcuts` (with
     `--skipactivation`) and `--debug`/`--console`/`--logpath` could be
     useful for *our own* troubleshooting while developing the wrapper, but
     are not part of the shipped tool's behavior — noting them here so
     they're not mistaken for something already planned.
4. **Scale — RESOLVED.** Confirmed range: **100 MB to 15 GB** actual content
   size across your real packages. That's a >100x spread, so stage 1 can't
   assume the winscp sample's ~292 MB is typical — the 15 GB end needs
   `Get-FileInventory.ps1`'s hashing pass to not choke:
   - Hash files with a streaming `SHA256` computation (fixed-size buffer,
     not `Get-FileHash` on a fully-buffered read) so memory use doesn't
     scale with individual file size — some vendored binaries (embedded
     Chromium, large native libraries) can be hundreds of MB on their own.
   - Run the walk/hash/identity-resolution pass with bounded parallelism
     (e.g. `ForEach-Object -Parallel` with a throttle limit, or a simple
     runspace pool) rather than serially, given the top end of this range —
     v1 will include this from the start rather than treating it as a later
     optimization, since 15 GB serially over a UNC path could be slow enough
     to be impractical for iterating on the tool itself.
   - Log elapsed time and file/byte throughput in the inventory JSON's
     `package` block (`scanStartedUtc`/`scanFinishedUtc` already covers
     this) so a slow run is visible in the coverage report rather than a
     silent surprise.
5. **Execution environment for stage 1**: assuming this runs interactively
   or via scheduled task directly on a machine with access to the package
   store (UNC path or local), with permissions to mount VHDX images. Confirm
   whether stage 1 needs to run inside ProfileUnity/FlexApp server context
   specifically, or any Windows host is fine.
6. **flexappone.exe availability for this PoC**: I don't have this binary in
   this environment and can't test the extraction path against it directly.
   Stage 1 will be written to the documented interface but the real
   validation happens on your side with real binaries/packages — flagging
   this now rather than after the fact.

None of these block writing the code — reasonable defaults are documented
above and the code will be defensive either way (never crash on a single bad
file, log and continue) — but getting real answers before Stage 1 is
"finished" avoids rework.

## Build order (pause after each, per your instructions)

1. Stage 1 extraction (mount/extract + file walk + hashing + XML metadata),
   no identity resolution yet — just produces the inventory JSON with
   `componentType` guesses and `identity: null` everywhere.
2. Identity resolution methods, in the priority order from the brief (PE →
   .NET → Java → Node/Electron → Python → string-scan), plus the exclusion
   ruleset.
3. OSV.dev matching (purl-based) + confidence tagging + on-disk cache.
4. NVD 2.0 CPE matching + cpe-mappings.yaml + rate-limit handling + cache.
5. Reporting: `sbom.cdx.json`, `coverage-report.md`, `findings.md`.

At each pause I'll show what was built plus any real output from local
tests (synthetic fixtures where I can't run against Windows/real packages
directly from this environment), before moving to the next stage.

## Open items I'm not deciding unilaterally

- Whether `coverage-report.md`/`findings.md` should be per-package files or
  aggregated across a batch of packages in one run — brief says "one per
  package" for the SBOM but is silent on the reports. Defaulting to
  per-package for all three outputs unless you'd rather have a batch rollup
  too.
- Dependency CVE flag: once `requirements.txt` is pinned in step 3/4, I'll
  separately check those pinned versions against known CVEs and report that
  to you directly (not as part of `findings.md`, which is about the scanned
  FlexApp package contents, not this tool's own dependencies).

---
**Waiting for confirmation on the plan and answers to the assumptions above
before writing any code.**
