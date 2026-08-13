# PLAN.md — FlexApp Package Composition & Vulnerability PoC

Status: **All five build-order steps complete, and Stage 1 validated live
against a real package on 2026-08-13.** You ran both classic VHDX and
FlexApp One against a real OBS Studio capture on a real Windows host —
both produced byte-identical results (2170 files, zero crashes, zero read
errors), which incidentally confirms the FlexApp One unwrap reconstructs
the exact same underlying package as its classic-VHDX counterpart. That
run caught and fixed one real bug (`Mount-ClassicFlexApp.ps1` waiting for
a drive letter that Windows never assigns — see "Assumptions," point 1,
addendum) and drove one real improvement to the exclusion ruleset (see
below). Still open: live network validation of the `resolve` command
against the real OSV.dev/NVD APIs, which this dev environment's egress
policy blocks - that's on you, not something I could complete from here.

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

1. **Classic FlexApp (.vhdx) layout — CONFIRMED, with one real bug fixed,
   from a live test on 2026-08-13.** A real OBS Studio VHDX mounted cleanly
   via `Mount-DiskImage -Access ReadOnly` (NTFS, no BitLocker, no special
   FlexApp/ProfileUnity component needed to mount it) - but the disk and
   partition came online healthy while **Windows never assigned a drive
   letter**, no matter how long `Mount-ClassicFlexApp.ps1`'s original
   retry-and-wait loop waited. This wasn't a timing issue - no letter was
   ever coming. Fixed by mounting to a scratch folder via
   `Add-PartitionAccessPath` instead of depending on/racing with automatic
   drive-letter assignment, which also sidesteps drive-letter exhaustion on
   a host scanning many packages. Confirmed working after the fix, on both
   classic VHDX and FlexApp One (which unwraps to the same VHDX format) -
   both produced byte-identical inventory output for the same package.
   - **New discovery**: the VHDX's internal path layout isn't a flat
     `Program Files\...` root - files sit under a wrapper like
     `<capture-timestamp>-<name>-1\Volumes\C\Program Files\...`. This
     doesn't affect anything today (nothing currently parses `relativePath`
     structurally), but it means `flexAppXml.shortcutTargets` (a raw
     `C:\Program Files\...` string from the package XML) can't be
     naively string-matched against `relativePath` if that's ever wanted -
     it would need the wrapper prefix stripped first.
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
5. **Execution environment for stage 1 — RESOLVED for v1.** Any Windows
   host with network access to the package store (UNC path) and permission
   to mount VHDX images is sufficient — no dependency on running inside the
   ProfileUnity/FlexApp server context itself.
   - You floated a nicer version of this: logging into the ProfileUnity
     console and querying it for package locations, rather than being
     handed a path by hand. That's a real improvement (it would turn "here's
     a path" into "point this at your ProfileUnity server and it enumerates
     every package"), but it's explicitly out of scope for this PoC per the
     brief's own non-goals list ("No ProfileUnity console integration, no
     Stratusphere correlation"). I'm treating your "ideally" as a noted
     future direction, not a v1 requirement — flagging the tension so it's
     an explicit choice rather than something that slips in by drift. If you
     want to fold ProfileUnity-console-driven path discovery into this PoC
     after all, say so and I'll adjust the plan (it's a fairly small
     addition — a `Get-FlexAppPackagePaths.ps1` that queries the console's
     API/DB instead of the caller supplying a path — but it's still a scope
     change from what was originally asked for, so it should be a deliberate
     yes, not something I add unprompted).
   - v1, as scoped: `Invoke-FlexAppInventory.ps1` takes an explicit
     `.vhdx`/`.exe`/`.flexapp` path (or a folder to enumerate), supplied by
     you or a simple wrapper script, not discovered via ProfileUnity.
6. **flexappone.exe availability for this PoC**: resolved as part of point 3
   above — there's no separate `flexappone.exe`; the package `.exe` itself
   is the extraction tool. I still don't have one of these binaries or a
   real package in this environment to test the extraction path against
   directly. Stage 1 will be written to the documented `--extract --skipico`
   interface, but the real validation happens on your side with real
   packages — flagging this now rather than after the fact.

All six items above are now resolved with real answers rather than
assumptions, except the two flagged as validated on your side once you have
a real binary/package in hand (point 6) — the code itself will still be
defensive regardless (never crash on a single bad file, log and continue).

## Build order (pause after each, per your instructions)

1. **Done.** Stage 1 extraction (mount/extract + file walk + hashing + XML
   metadata).
2. **Done, and live-validated.** Identity resolution methods, in the
   priority order from the brief (PE → .NET → Java → Node/Electron →
   Python → string-scan), plus the exclusion ruleset. Syntax-checked and
   functionally smoke-tested on Linux PowerShell 7.6 before the live test -
   real `.package.xml` samples, real jars built with `jar`, a real
   `app.asar` built with the `asar` npm package, hashes cross-checked
   against `sha256sum`, and the full pipeline's output validated against
   `schemas/inventory.schema.json`.

   **Live test against a real OBS Studio package (2026-08-13)**: 2170
   files, zero crashes, zero read errors. Real Win32 version resources
   resolved cleanly and with real vendor/product/version strings - FFmpeg
   DLLs, `libcurl.dll`, and notably `libcef.dll` (Chromium Embedded
   Framework), whose own version resource already embeds the Chromium
   version (`127.145.7+g2b7d20b+chromium-127.0.6533.120`) with no
   string-signature guessing needed. Raw coverage on that package looked
   low (3.7%) until digging into *why*: 1656 of 1826 "unresolved" files
   were `.ini` config/locale files alone (90%), plus `.pak`/`.effect` data
   files - none of them third-party components. Added all three as new
   `ExclusionRules.psd1` categories (`config-file`/`resource-pack-file`/
   `shader-effect-file`), which moved that package's honest coverage number
   to **53.0%** - much closer to the range that would make this concept
   worth pursuing, and a good demonstration of why "excluded, but
   transparently" matters: the fix was a legitimate exclusion, not hiding
   anything, since config/data files were never real candidates.

   Real gaps this run surfaced, left as-is rather than guessed at: a
   handful of genuine third-party libraries (`lua51.dll`, `librist.dll`,
   `srt.dll`, `datachannel.dll`) have neither a Win32 version resource nor
   a matching `string-signatures.psd1` pattern - expanding the pattern set
   further would mean guessing at byte content I can't inspect directly,
   so this stays an honest "unresolved," not a fabricated match.

   Also found a real schema-contract violation this way: some of OBS
   Studio's own internal plugin `package.json` files use a bare numeric
   version (`"version": 9`, not `"9"`) - `ConvertFrom-Json` returned that as
   a PowerShell int, which `Get-NodePackageIdentity`/
   `Get-AsarPackageIdentities` passed straight through, violating the
   schema's "version is string|null" contract and failing
   `jsonschema.validate` on the Stage 2 side. Fixed by explicitly coercing
   both name/version to string (guarding the `$null`-becomes-`""` PowerShell
   coercion gotcha found earlier) rather than trusting `package.json`'s
   fields to already be the type the spec expects.

   **Confirmed end to end after the fix**: full re-run on the same real
   package produced a schema-valid inventory JSON, and Stage 2's `report`
   command generated a valid 14-component `sbom.cdx.json` and a
   `coverage-report.md` stating **53.0% resolution coverage** - the exact
   number predicted from analyzing the raw JSON directly, now confirmed via
   the actual reporting pipeline rather than a hand calculation.

   **Second live test, a real 7-Zip package (2026-08-13)**: much smaller
   (112 files), and surfaced the same class of finding from a different
   angle. Raw coverage looked very low (5.6%) until digging in: 93 of 101
   "unresolved" files (92%) were per-language translation `.txt` files
   under a `\Lang\` folder - not components. Added `\Lang\`/`\Locale\`/
   `\Locales\`/`\i18n\`/`\l10n\` as a new `localization-file`
   `ExclusionRules.psd1` path rule (broader than just 7-Zip's specific
   folder name, to generalize to other apps' localization conventions),
   which moved the honest coverage number to **42.9%**. The 6 resolved
   files (`7z.exe`, `7z.dll`, `7zFM.exe`, `7zG.exe`, `7-zip.dll`,
   `7-zip32.dll`) all correctly attributed to the same real component
   (Igor Pavlov / 7-Zip / 26.01) and deduplicated to one `sbom.cdx.json`
   entry, exactly as designed.

   Also found and fixed, reviewing this run's actual output files rather
   than just the numbers: (a) `sbom.py` was emitting `"name": null` for
   package.json-derived components with no name field, which is invalid
   per the real CycloneDX 1.6 schema (confirmed via `jsonschema.validate`
   against the official schema) - fixed by skipping nameless components
   from the SBOM (they still count as "resolved" for coverage purposes);
   (b) `normalize.py`'s CPE escaper only handled backslash/colon, on the
   wrong assumption that version strings were "already CPE-safe" - real
   Win32 ProductVersion strings (`x264`'s `"0.164.3106 eaa68fa"`, ANGLE's
   `"2.1.23296 git hash: e323abb5b08e"`) proved that false, producing
   invalid CPE 2.3 strings with raw unescaped spaces; fixed with a proper
   CPE-spec reserved-character escaper; (c) `cpe_mappings.py` required an
   exact `method` match, so a real `zlib` Win32 version resource (method
   `pe-version-resource`) missed the existing mapping written only for
   the `string-signature` path - made `method` optional in a mapping
   entry. Deliberately did NOT add guessed mappings for OBS
   Studio/FFmpeg/Qt6/x264/Chromium/ANGLE/CEF (seen fragmenting into
   multiple CPEs across binaries with different vendor-string variants) -
   without live NVD access to verify the real dictionary entry, that would
   present a guess as `mapped-cpe` confidence, no more trustworthy than
   `heuristic` but labeled as if it were.

   **Third live test, a real Paint.NET package (2026-08-13)**: 482 files,
   40 excluded, 311/442 candidates resolved - **70.4% coverage**, well
   into the range that makes the concept worth pursuing. But the
   coverage report's "resolved, by method" breakdown showed *100%*
   `pe-version-resource` and *zero* `dotnet-manifest`, which was wrong on
   its face - several of the resolved names are well-known managed .NET
   libraries (Json.NET, Mono.Cecil + its `.Mdb`/`.Pdb`/`.Rocks` variants,
   ComputeSharp.Core), which should have resolved via the
   higher-priority `dotnet-manifest` path. Reproduced directly against a
   real assembly (`Newtonsoft.Json.dll`, borrowed from the local `pwsh`
   install) and confirmed a real bug in `Get-DotNetAssemblyIdentity`:
   `GetMetadataReader()` is an *extension* method on the static class
   `System.Reflection.Metadata.PEReaderExtensions`, not an instance
   method on `PEReader` - PowerShell's method binder doesn't resolve
   extension methods via `$peReader.GetMetadataReader()` dot-syntax the
   way the C# compiler does, so the call threw
   `MethodException: ... does not contain a method named
   'GetMetadataReader'` on *every single invocation*, silently swallowed
   by the surrounding catch-all, falling through to the
   `pe-version-resource` path every time. Fixed by calling it as a static
   method instead: `[System.Reflection.Metadata.PEReaderExtensions]::GetMetadataReader($peReader)`.
   Verified against the same real assembly post-fix - now correctly
   returns `method: dotnet-manifest`, `product: Newtonsoft.Json`,
   `version: 13.0.0.0`, plus a real public key token and the Win32
   file-version cross-check. This means every earlier live-test coverage
   number for a managed/mixed package undercounted `dotnet-manifest`
   precision in favor of the coarser `pe-version-resource` method (the
   raw *coverage percentage* is unaffected - both methods count as
   "resolved" - but confidence/precision of individual .NET component
   identities was silently downgraded). Re-running Paint.NET (or any
   .NET-heavy package) after this fix should now show real
   `dotnet-manifest` entries in the method breakdown.

   **First live attempt at the `resolve` step (2026-08-13)**, now that a
   real Windows host had working network access: surfaced two more real
   bugs in `nvd_client.py`, both the same shape - a documented, recoverable
   NVD API response (404 for "no dictionary match", 429 for
   "rate-limited") was caught by `cli.py`'s blanket
   `except requests.exceptions.RequestException` and misreported as
   "could not reach services.nvd.nist.gov", aborting the whole run. Fixed
   by treating 404 as an empty, cacheable result, and 429 with
   retry-with-backoff (honoring `Retry-After`). That in turn surfaced a
   real **scale** problem, not a bug: without an API key, NVD's 5 req/30s
   limit means a package with a few hundred resolved components takes
   20-30+ minutes for the NVD side alone - not viable across many customer
   packages. Added a local NVD mirror (`nvd_mirror.py`, `mirror-nvd` CLI
   subcommand, `resolve --nvd-mirror`) that bulk-paginates NVD's full CVE
   dataset once (NVD retired downloadable feed files in December 2023, so
   this is the API-only equivalent) and matches locally afterward with
   zero network calls per scan. The live per-CPE path remains the default;
   the mirror is opt-in. Still pending: an actual timed `resolve` run
   against a real package, live-vs-mirror, to confirm the real-world
   speedup and check the mirror's version-range matching against genuine
   NVD data.

   **Full live `resolve` + `report` run against Paint.NET (2026-08-13)**:
   completed cleanly with no errors (confirming the 404/429 fixes hold up
   live), 442 candidates, 312 CPE-expressible, 0 purl-expressible
   (expected - `dotnet-manifest`/`pe-version-resource` don't map to a
   purl), 0 vulnerability matches. The coverage report's method breakdown
   now shows real `dotnet-manifest` resolution working
   (271 `dotnet-manifest` / 41 `pe-version-resource`, confirming the fix
   above), and the zero-matches result is plausible rather than
   suspicious: most of those 312 CPEs are `heuristic`-confidence
   auto-normalized strings with no curated `cpe-mappings.yaml` entry,
   unlikely to match a real NVD dictionary entry exactly - a real,
   honestly-labeled limitation of this pipeline, not a bug.

   The now-visible unresolved-files list (130 files) surfaced a new noise
   pattern: this package was installed via Chocolatey on the capture
   machine, so Chocolatey's own package-manager footprint got captured
   into the VHDX alongside Paint.NET itself. Added four more
   `ExclusionRules.psd1` categories - `package-manager-path`
   (`\chocolatey\`/`\ChocolateyHttpCache\`), `dotnet-xmldoc-file` (`.xml`
   doc-comment files), `dotnet-resource-data` (`.resources`/`.resx`),
   `readme-license-text`, and `shell-shortcut` (`.lnk`) - verified against
   the real inventory JSON that none of the 126 newly-excluded files ever
   had a resolved identity. Moved that package's honest resolution
   coverage from **70.6% to 98.7%**.

   Deliberately deferred rather than implemented: `paintdotnet.deps.json`
   (currently excluded as noise) actually lists every NuGet dependency
   name + exact resolved version - a real lockfile, same idea as the
   `jar-pom-properties` highest-confidence path already in the pipeline.
   Parsing it would give exact-version identities instead of guessing from
   a DLL's PE resource, but that's a new identity-resolution capability,
   not an exclusion-rule tweak - scoped as a candidate follow-up, not done
   here.
3. **Done.** OSV.dev matching (purl-based) + confidence tagging + on-disk
   cache. Purls built for `jar-pom-properties`/`node-package-json`/
   `python-dist-info` only (the three OSV-supported ecosystems this
   pipeline's identity methods map onto cleanly); everything else correctly
   gets `purl: null` here, deferred to step 4. `api.osv.dev` is blocked by
   this dev environment's network policy (same category as the
   `grype.anchore.io` block from the earlier Sparks audit) - the client is
   validated with 22 passing mocked-HTTP unit tests instead of a live call,
   and the CLI fails with a clear message rather than a raw traceback when
   it can't reach the API. Live end-to-end validation against the real
   OSV.dev API still needs an environment where it's reachable.
4. **Done.** NVD 2.0 CPE matching + `cpe-mappings.yaml` + rate-limit
   handling + cache. CPE candidates built for `pe-version-resource`/
   `dotnet-manifest`/`string-signature`/`electron-embedded` only —
   `jar-manifest` (no groupId) correctly gets neither a purl nor a CPE,
   an honest "unresolved with lower-confidence metadata" outcome, not a gap
   to paper over. A hit in the curated `cpe-mappings.yaml` override table
   is confidence `mapped-cpe`; falling back to automatic heuristic
   normalization is confidence `heuristic` and is never presented as a
   confirmed finding. Rate limiting (5 req/30s without `NVD_API_KEY`, 50
   with) implemented as a sliding window, validated with an injectable fake
   clock so tests run instantly instead of waiting real seconds.
   `services.nvd.nist.gov` is blocked by this dev environment's network
   policy too (same category as OSV/Grype) - validated with mocked-HTTP
   unit tests, and confirmed the CLI's real (blocked-network) failure path
   correctly names `services.nvd.nist.gov` specifically when only a
   CPE-eligible component is present (bypassing OSV entirely). Live
   end-to-end validation against the real NVD API still needs an
   environment where it's reachable. 45 tests passing total across both
   matching steps.
5. **Done.** Reporting: `sbom.cdx.json`, `coverage-report.md`, `findings.md`.
   Coverage computation and SBOM generation are deliberately independent of
   OSV/NVD network access - they only need the Stage 1 inventory JSON, since
   the headline coverage percentage is about identity resolution, not
   vulnerability matching. This means the flagship number this whole PoC
   exists to produce (see "Goal," top of this document) can always be
   generated, even fully offline. `findings.md` is the one report that
   genuinely needs a `vuln-matches.json` from the `resolve` step; without
   one it says so plainly rather than rendering something that could be
   mistaken for "no vulnerabilities found." The SBOM was validated against
   the real, official CycloneDX 1.6 JSON schema (via the `cyclonedx-python-lib`
   package's bundled schema file, `jsonschema.validate`) - not just reasoned
   about. `findings.md` separates confirmed matches (`exact-purl`/
   `mapped-cpe`) from `heuristic` ones under an explicit "verify manually"
   warning, per the confidence-tagging rule. 61 tests passing total.

At each pause I'll show what was built plus any real output from local
tests (synthetic fixtures where I can't run against Windows/real packages
directly from this environment), before moving to the next stage.

6. **Candidate follow-up, not started: parse `*.deps.json` for exact
   dependency identities.** Surfaced by the Paint.NET live test - after
   the exclusion-rule additions above, `paintdotnet.deps.json` is one of
   only 4 files left unresolved on that package, currently discarded as
   noise (`dotnet-runtimeconfig`'s sibling). It shouldn't be: a .NET
   Core/5+ `.deps.json` is a real dependency lockfile - it lists every
   NuGet package name and exact resolved version the app was actually
   built against, in a `.libraries` section keyed
   `"<PackageName>/<Version>"`. That's the same shape of "highest
   confidence available" signal `jar-pom-properties` already provides for
   Java fat jars, and it would give exact identities for managed
   dependencies that only show up today as a single top-level assembly's
   `dotnet-manifest` result (or not at all, if the dependency is IL-linked/
   trimmed away and its DLL never shipped, but its `.deps.json` entry
   still names it).

   Scoped design, not yet implemented: a new `Get-DepsJsonIdentities`
   function (new `DepsJson.psm1` module, same shape as
   `NodeAsar.psm1`/`PythonDist.psm1`) parsing `.libraries` into one
   synthetic component per entry (skip the `type: "project"` entry for the
   app itself), dispatched in `Resolve-VersionIdentity.ps1` alongside the
   existing `package.json`/`METADATA` special-cased filenames. Would need
   a purl mapping in `stage2-resolve/normalize.py` too (NuGet packages are
   `pkg:nuget/<name>@<version>`, not currently an OSV-supported ecosystem
   this pipeline maps to - would need checking OSV's NuGet ecosystem
   support before assuming it resolves there). Not started - flagging the
   design here rather than guessing at NuGet-ecosystem OSV support without
   verifying it first.

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
