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

   **Live test against a real Notepad++ package (2026-08-13)**: a native
   C++ app (all 8 resolved identities are `pe-version-resource`, none
   `dotnet-manifest`) - correctly identified the main exe, context-menu
   shell extension, uninstaller, WinGup auto-updater, and 3 plugins
   (NppExport, mimeTools, NppConverter), 53.3% coverage on a much smaller
   package (194 files). Also caught a labeling bug: the `.xml` exclusion
   rule correctly excluded 108 files, but they were Notepad++'s own
   `autoCompletion`/`themes`/`langs.model` config XML, not .NET doc
   comments - renamed the reason from `dotnet-xmldoc-file` to the honest,
   general `xml-data-file` (same matching behavior). Noted, not
   implemented: `contextMenu\NppShell.msix` is an MSIX (zip container with
   a real `AppxManifest.xml`) sitting unresolved - a candidate future
   container-identity method, same idea as jar/asar.

   **First real vulnerability findings, against a real Python package
   (2026-08-13)**: the milestone this whole PoC exists to prove out -
   confirmed `mapped-cpe` CVE matches for bundled zlib/OpenSSL/SQLite, and
   `exact-purl` GHSA/PYSEC matches for `pip` via OSV.dev (the first live
   exercise of `python-dist-info` + the PyPI purl path). Also caught a
   real rendering bug in `findings.md`: `render_findings` emitted one row
   per `(component, vulnerability)` pair with no identity dedup, unlike
   `sbom.py` which already dedupes by purl/CPE - when two physical files
   share an identity (two copies of the same bundled `sqlite3.dll`), every
   CVE was rendered once per file. On this package's real output that was
   246 rows for what was actually 146 distinct findings. Fixed by
   deduplicating on `(purl-or-cpe-or-relativePath, vulnerability id)`
   before rendering, verified against the real data before and after.

   **Live test against a real Jonker ERP package, Remix-H1-DROOG
   (2026-08-13)**: a large, old, real-world Java business application (a
   Dutch dry-mortar/concrete company's bespoke ERP suite) - the best
   stress test yet of the Java identity path. Confirmed real, well-known
   CVEs entirely via `jar-pom-properties` + `exact-purl`: Commons
   Collections deserialization, iText command injection, Logback
   deserialization, JasperReports RCE, HttpClient cert validation - zero
   heuristic-confidence noise in the findings. Also correctly left 24 real
   third-party jars (Apache Ant launcher, Batik JS, RXTX comm, JSch,
   JPedal, JTidy, JUnit, SwingX, ...) honestly unresolved rather than
   guessing - old-style jars with neither `pom.properties` nor usable
   `MANIFEST.MF` identity attributes, a real metadata gap, not a bug.

   Raw coverage looked very low (8.9%) until digging in: 868 of 968
   "unresolved" files (90%) were JasperReports compiled report designs
   (`.jasper`) and their locale-variant `.properties` sidecars
   (`Aannemer.properties`/`_en.properties`/`_bam.properties`), all inside
   one `TClickRapporten\` folder - this app's own report templates, not
   components. Added `report-design-file` (`*.jasper*`, wildcarded to
   also catch date-stamped backup copies like `Bon.jasper.2013-07-18`
   that a plain extension rule would miss) and `report-design-folder`
   (`\TClickRapporten\`, needed for the `.properties` sidecars, which
   aren't identifiable by extension alone). Verified against the real
   inventory JSON that none of the newly-excluded files had a resolved
   identity - moved that package's honest resolution coverage from **8.9%
   to 48.7%**.

   The remaining unresolved files pointed at three more noise categories:
   a `\Wijzigingen\` (Dutch: "changes") folder of plain-text release
   notes (35 files), a `\Log\<module>\` runtime log-output folder (28
   files), and Oracle's own JRE usage-telemetry files
   (`\.oracle_jre_usage\`, 2 files - not part of the app at all, an
   artifact of any machine that has ever run a JRE). Added all three;
   verified none had a resolved identity - moved that package's coverage
   from 48.7% to **73.1%**. Also noticed `appinstall.cap`/`printers.bak`
   recurring for the third package in a row now, always in a top-level
   `\Data\` folder that sits alongside `\Volumes\C\...` rather than inside
   it - increasingly looks like FlexApp/ProfileUnity's own capture
   scaffolding rather than app content, but scoping a rule precisely
   (without also catching a real app-internal folder that happens to be
   named "Data") needs more evidence - held off adding a rule for it.

   **Live test against a real Tor Browser package (2026-08-13)**:
   surfaced a real, serious bug rather than a noise-tuning opportunity -
   **0.0% coverage**, all 242 files excluded. Tor Browser was captured as
   a "portable" Chocolatey package, where Chocolatey's `lib\<pkg>\tools\`
   folder *is* the actual installed application
   (`\chocolatey\lib\tor-browser\tools\tor-browser\Browser\firefox.exe`,
   `nss3.dll`, `softokn3.dll`, ...) rather than separate management
   scaffolding next to a real install elsewhere - the blanket
   `package-manager-path` rule from the Paint.NET scan couldn't
   distinguish that from Chocolatey's own housekeeping and excluded the
   whole package. Narrowed the path rule to only Chocolatey's own known
   management subfolders, and added a `package-manager-file` name-pattern
   rule (`*.nuspec`/`*.nupkg`/`chocolatey*.ps1`/`*.ignore`) to still catch
   Chocolatey's own manifest/installer-script files by name wherever they
   sit, including inside `tools\`. Re-checked against the real Paint.NET
   and Notepad++ inventories (both also Chocolatey-based captures) to
   confirm no regression - Paint.NET's candidate count is unchanged (315,
   still 99.0%), and re-checking Notepad++ surfaced one more real pattern
   (a Chocolatey extension package's nested
   `<pkg>.extension\extensions\` folder, and a third Chocolatey hook
   script name, `chocolateyBeforeModify.ps1`, generalized to
   `chocolatey*.ps1`).

   Since Stage 1 ran against Tor Browser under the old (buggy) rules,
   the 186 real payload files were skipped entirely rather than resolved
   or unresolved - their true identity/coverage number needs an actual
   Stage 1 re-run against the real VHDX with the fixed rules, not
   something derivable from the existing inventory JSON alone. That
   re-run is the next step, including checking whether `nss3.dll`/
   `softokn3.dll` (bundled NSS crypto libraries) turn up any real CVEs
   once resolved.

   **The re-run (2026-08-13): 0.0% -> 35.3% coverage (18/51), confirming
   the fix.** Real, current findings: `tor.exe`'s embedded OpenSSL
   version banner (`"OpenSSL 3.5.6 7 Apr 2026"`) resolved via
   `string-signature` and matched a long list of real 2026 OpenSSL CVEs
   (including a CRITICAL, CVE-2026-34182) via `mapped-cpe` - a strong,
   current-day validation of that path.

   `nss3.dll`/`softokn3.dll`/`freebl3.dll` did NOT turn up their own real
   version, though - a genuine, structural limitation, not a bug:
   Mozilla stamps every DLL in the Firefox/Tor Browser tree with the
   *browser's* product version (`pe-version-resource` returns "Tor
   Browser 140.10.0" for all of them), and
   `Resolve-VersionIdentity.ps1`'s dispatcher only falls through to
   `Get-StringSignatureIdentity` when `Get-PEVersionResourceIdentity`
   returns `$null` - it never does here, so string-signature scanning
   (the same technique that found OpenSSL in `tor.exe`) never gets a
   chance to run on these specific files, even though NSS is known to
   embed its own plain-text version string internally. `application.ini`/
   `platform.ini` were checked as a cheaper alternative and ruled out -
   Firefox's well-documented format for those only carries the umbrella
   app version, not per-bundled-library versions.
   
   A real fix would mean not treating a `pe-version-resource` hit as
   terminal for known multi-DLL vendors (Mozilla, likely Chromium/
   Electron too) - also run string-signature scanning and prefer it when
   it matches a known library name. That's a dispatch-priority design
   change, not a quick pattern addition, and the exact NSS string format
   needs verifying against a real binary before writing a signature for
   it - not done here, flagged as a candidate follow-up alongside the
   `deps.json` parser above.

   **Live test against a real Firefox package (2026-08-13)**: same
   Mozilla umbrella-versioning pattern as Tor Browser (most DLLs stamped
   with the browser's own product version, 61.3% coverage, 46/75), but
   surfaced a more serious, distinct bug this time: `browser\omni.ja`
   contains a Chrome-User-Agent-spoofing string
   (`"Chrome/67.0.3396.87"`, part of Firefox's own site-compatibility
   overrides - Firefox ships this even though it doesn't use Chromium at
   all) that matched the `Electron Chromium` string-signature pattern,
   wrongly resolving it as an embedded Chromium 67.0.3396.87 and pulling
   in a decade of real Chrome CVEs (back to 2012) that have nothing to do
   with this package. This was the first live trigger of the
   `electron-embedded` method ever (previous packages' Electron
   detection needs were already satisfied by a real PE version resource,
   per point 2's Electron note) - and it fired wrong the very first time.
   Fixed by requiring the `Electron/<version>` token real Electron apps
   always carry adjacent to `Chrome/` in their own default User-Agent
   string, which Firefox's spoofed string doesn't have. Verified against
   both the real false-positive text and a genuine synthetic Electron UA
   string that the fix discriminates correctly.

   Also noted, not fixed: `onnxruntime.dll`'s embedded `ProductVersion`/
   `FileVersion` are literally the unsubstituted build macro
   `"ORT_VERSION"` - a real upstream Microsoft build defect, honestly
   reflected by this pipeline rather than guessed around.

   **The re-run confirmed the fix and immediately found a second,
   related false positive**: the `Electron Chromium` entry was gone, but
   `omni.ja`'s arbitrary bundled text now matched `Electron Node.js`
   instead (`"Node.js v8.11.1"`), again wrongly attributing an embedded
   Node.js runtime - and its own decade of CVEs - to a Gecko-based app
   with none. Two different signatures false-positiving on the exact same
   file is evidence the file itself, not any one pattern, is the real
   problem: `.ja` (Mozilla's own resource-bundle format) is genuine
   Firefox content but never a vendored native library, and its arbitrary
   JS/JSON/locale text makes it structurally unsafe for last-resort
   string-signature scanning no matter which signature is checked. Fixed
   at the dispatcher level instead of patching yet another pattern -
   `.ja` files now short-circuit to a genuinely unresolved identity
   before string-signature scanning ever runs (verified this returns
   `$null` without even opening the file). Left unresolved rather than
   excluded, since it's real content, just not safely identifiable this
   way.

   **Cross-package survey (2026-08-13)**: ran `resolve`+`report` against
   OBS Studio and 7-Zip (previously Stage-1-only), giving 9 real packages
   total with full vulnerability-matching results. OBS Studio was the
   single biggest CVE find of the whole project: bundled `libcurl
   8.12.1-DEV` matched ~30 real CVEs (6 CRITICAL) via the curated
   `cpe-mappings.yaml` entry, plus 2 more from bundled zlib. Across all 9
   packages, only 4 (Python, Remix-H1-DROOG, Tor Browser, OBS Studio)
   produced any confirmed finding at all - every one traced to either
   `jar-pom-properties`+OSV or a curated `mapped-cpe` entry for a
   well-known bundled native library. Every package where the *main*
   application binary was the only thing resolved (`heuristic`-only)
   produced zero findings - the honest boundary this project set out to
   measure, not a gap in effort.

   Also noticed, with now-overwhelming evidence: `appinstall.cap`/
   `printers.bak`/`DisableShortPaths`/`Suppress.ACL` showed up in a
   top-level `\Data\` folder (sibling to `\Volumes\C\...`, not inside it)
   on all 9 packages, always these exact 4 filenames, always unresolved.
   Added `flexapp-capture-scaffolding` to `ExclusionRules.psd1` - FlexApp/
   ProfileUnity's own package-capture scaffolding, not app content.
   Scoped to the exact folder+filename combination (not a bare filename
   or folder-name match) so a real app's own internal folder that happens
   to be named "data" is never touched. Verified against 5 real
   inventories (Paint.NET, 7-Zip, OBS Studio, Remix, Chromium) that
   nothing newly-excluded ever had a resolved identity - small coverage
   bumps across all of them (e.g. 7-Zip 50%->60%, Remix 73.1%->75.4%).

   **`cpe-mappings.yaml` curation against the real NVD dictionary
   (2026-08-13)**: `nvd.nist.gov` itself is blocked from direct fetch in
   this dev environment (same as `services.nvd.nist.gov`), but web
   search still surfaces its indexed detail pages - used that to check
   the app-specific products this file's own comment had previously
   left out for being unverified. Confirmed real, dedicated NVD entries
   for FFmpeg (`ffmpeg:ffmpeg`, 428 records), Qt (`qt:qt` - not `qt6`,
   397 records), and Chromium (`chromium:chromium`, distinct from Google
   Chrome); confirmed NO dedicated entry exists for OBS Studio, x264,
   CEF, or ANGLE (OBS Studio's own product doesn't appear in the
   dictionary at all under any name found; ANGLE/CEF bugs surface as
   Chromium CVEs instead of their own) - correctly left those out rather
   than guessing.

   A vendor/product fix alone wasn't enough for FFmpeg/Qt: their
   detected version strings (FFmpeg's own git-tag convention `"n7.1.1"`;
   Qt's 4-part Win32 FILEVERSION `"6.8.3.0"`) don't match NVD's plain
   3-part dictionary format. Added a version-transform mechanism
   (`versionPattern`/`versionGroup` on a mapping entry) rather than
   leaving those two silently non-functional - falls back to the raw
   version unchanged if the pattern doesn't match, same "don't guess"
   spirit as everywhere else in this pipeline. Also switched the
   existing `Electron Chromium` mapping from its documented
   `google:chrome` approximation to the now-confirmed real
   `chromium:chromium` CPE - more accurate, since Electron bundles
   Chromium, not Chrome itself. Verified against real OBS Studio and
   Chromium data that all three produce clean, correctly-shaped CPE
   strings ready for a live `resolve` run to confirm real matches.
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

   **Implemented (2026-08-13).** Verified via web search (api.osv.dev is
   also blocked from direct fetch in this dev sandbox, same as
   services.nvd.nist.gov) that NuGet is a real OSV-supported ecosystem with
   standard `pkg:nuget/<name>@<version>` purl querying - the scoped design
   above was safe to build. Added `Get-DepsJsonIdentities`
   (`stage1-extract/Modules/DepsJson.psm1`): parses `.libraries`, skips
   `type: "project"` entries (the app itself), returns one synthetic
   component per remaining entry (`package`, `runtimepack`, etc.) - same
   shape as jar/asar nested components, since one `*.deps.json` names many
   logical dependencies from a single physical file. Wired into
   `Resolve-VersionIdentity.ps1`'s dispatcher (`*.deps.json` filename match,
   alongside the existing `package.json`/`METADATA` special cases) and
   `Get-FileInventory.ps1`'s componentType guess. Added the
   `dotnet-deps-json` -> `pkg:nuget/<name>@<version>` purl mapping to
   `stage2-resolve/normalize.py`'s `build_purl`. Verified locally against a
   synthetic multi-package `.deps.json` fixture (pwsh is available in this
   dev sandbox even though live NVD/OSV network calls aren't) - confirmed
   the `project` entry is correctly skipped, `package`/`runtimepack` entries
   both resolve, and relative-path rooting for the synthetic entries matches
   the jar/asar convention exactly. 98 Python tests passing (2 new).

6. **Candidate follow-up, now implemented: Mozilla-DLL dispatch-priority
   fix.** Per the Tor Browser/Firefox re-run findings above,
   `nss3.dll`/`softokn3.dll`/`freebl3.dll` never got a chance at
   string-signature scanning because `Resolve-ComponentIdentity` treated any
   successful `Get-PEVersionResourceIdentity` hit as terminal, even when
   that hit was just the browser's own umbrella version stamped onto every
   DLL in the tree. Fixed the dispatch-priority half of this (not the
   NSS-signature half - see below): added a `UmbrellaVersionedProducts`
   list to `string-signatures.psd1` (`Firefox`, `Tor Browser`,
   `Thunderbird` - Mozilla's Gecko-platform family, the only vendor with
   live evidence of this behavior so far), and a `Test-UmbrellaVersionedIdentity`
   helper in `Resolve-VersionIdentity.ps1`: when a PE-resource identity's
   `product` matches one of these, the dispatcher now also runs
   string-signature scanning and prefers that result over the umbrella
   identity *only if it actually matches a known vendored-library banner*
   (OpenSSL/zlib/libcurl/SQLite/etc in `Signatures`) - never overrides with
   a non-match. `Import-StringSignatures` now returns both `Signatures` and
   `UmbrellaVersionedProducts` from the one config file; the one caller
   (`Invoke-FlexAppInventory.ps1`) updated accordingly.

   Verified locally (no Pester tests existed for stage1 before this; used
   the same "synthetic fixture + direct pwsh invocation" style as the rest
   of this project's local validation) against three scenarios: (1) a
   PE identity stamped "Tor Browser 140.10.0" on a file that also contains
   a real OpenSSL banner string - correctly switches to the OpenSSL
   identity; (2) the same umbrella PE identity on a file with no known
   banner text - correctly keeps the umbrella identity unchanged, no
   regression; (3) a non-Mozilla PE identity ("7-Zip") on a file that
   happens to contain OpenSSL-shaped text anyway - correctly left alone,
   since it never enters the umbrella-check branch at all.

   **Deliberately still NOT done**: this does not make NSS's real version
   resolvable yet. No NSS-specific string-signature pattern was added,
   because the exact embedded-string format still needs verifying against
   a real binary (this dev sandbox has no way to inspect a live
   softokn3.dll/freebl3.dll) - writing a guessed pattern here would risk
   exactly the kind of false-positive/false-confidence this project has
   spent the whole session avoiding. The dispatch mechanism is now correct
   and general (it will already improve any Mozilla-family DLL that
   happens to bundle a library `Signatures` already knows how to
   recognize); a genuine NSS signature is a separate, still-open follow-up
   that needs real binary access to do honestly.

**Live test against a real Nextcloud Client package (2026-08-13, after the
inventory schema fix above)**: 19.2% coverage (148/772) at first, but the
FFmpeg/zlib/Qt curated mappings from the earlier round validated hard on
current, real CVEs - bundled FFmpeg 8.0.1 matched 20+ real 2023/2025/2026
CVEs (several HIGH) via `mapped-cpe`, plus 2 zlib 1.3.1 CVEs. Zero
low-confidence noise in the findings.

Two clear noise patterns accounted for 568 of the 624 unresolved files
(91%): 507 were Qt Quick QML files (`.qml` UI-component declarations and
`.qmltypes` compiled type-metadata sidecars, part of Qt's own bundled QML
module tree - QtQuick, several Controls style variants, Qt5Compat, ...) and
their `qmldir` module-manifest files (plain text, no extension); 61 were
under `\ProgramData\Package Cache\{GUID}...\` - Windows Installer's shared
bootstrapper cache, where WiX/Burn-based installers (VC++ Redistributable,
.NET/.NET Desktop/ASP.NET Core Runtime prerequisites, versions 5.0.17
through 8.0.14 seen live) stash their `.msi`/`.exe`/`.cab` payloads,
keyed by installer GUID rather than app name - never part of the scanned
app. Added `qml-ui-file` (`.qml`/`.qmltypes` extensions) +
`qml-module-manifest` (`qmldir` filename) + `windows-installer-package-cache`
(`\Package Cache\` path) to `ExclusionRules.psd1`. Verified locally (pwsh
against the real relative paths from this scan) that all three correctly
exclude the noise and leave real app content (`nextcloudcmd.exe`, a
synthetic `ffmpeg.dll` path) untouched.

The remaining ~56 unresolved files are a genuine identity-resolution gap,
not noise: real, non-trivial native libraries (`brotlicommon.dll`/
`brotlidec.dll`, `bz2.dll`, `harfbuzz.dll`, KDE's `KF6Archive.dll`,
`libpng16.dll`, Qt's own `libsqlite.dll`/`qt6keychain.dll`, Nextcloud's own
sync-engine DLLs) - all sizeable (100KB-5MB), no read errors, valid
`pe-native` componentType, but no PE version resource and no
string-signature match. Notably `libsqlite.dll` is literally SQLite but
doesn't match the existing SQLite signature pattern, meaning this build
either has no embedded version banner or a different format than what's
already known. Left unresolved rather than guessed at - same principle as
the NSS follow-up above, this needs a real binary to inspect before writing
a new pattern, not something to fix blind from this dev sandbox.

7. **New feature, implemented: a polished PDF report (`report --pdf`).**
   Requested directly - after enough rounds of live testing produced a pile
   of separate `.md`/`.json` outputs per package, with no single document to
   hand someone. Added `pdf_report.py` (`render_pdf_report`), using
   `reportlab` - chosen over `weasyprint` specifically because it ships
   pure-Python/prebuilt wheels with no native system libraries (cairo/
   pango) to install, since this project actually runs on Windows machines
   without a build toolchain. The PDF combines the same data as
   `coverage-report.md` (resolution %, excluded/resolved breakdowns,
   unresolved-file list) and `findings.md` (confirmed vs. low-confidence
   findings, severity-color-coded) into one document - a presentation layer
   over the same source of truth, not a second copy of it.

   Refactored `reporting.py`'s finding dedup-by-identity/severity-sort logic
   (previously inline in `render_findings`) into a shared
   `build_finding_rows()` so the Markdown and PDF renderers can't quietly
   drift apart on what counts as "the same finding." Wired via a `--pdf`
   flag on the existing `report` subcommand (no new subcommand - it's the
   same data, just another output format) rather than a separate `--out`
   path, so it always lands next to the other three files.

   Verified with 6 new tests (structural PDF validity - `%PDF-`/`%%EOF`
   markers, non-trivial size - across empty/no-data/zero-candidate edge
   cases, plus a CLI-level `report --pdf` integration test confirming the
   file actually lands in `--out` alongside the others) and, since PDF
   rendering can't be meaningfully unit-tested for visual correctness,
   rendered the real Nextcloud Client scan data end-to-end and inspected
   the output directly.

8. **New feature, implemented: a local web UI (`webui/`).** Requested
   directly, as a "full pipeline runner" (pick a package, click a button,
   watch it run) rather than just a report viewer - explicitly out of this
   PoC's original non-goals list ("No GUI, no web service"), superseded by
   this direct request. Chose a local Flask app over a desktop GUI toolkit
   or a static-HTML-only approach: cross-platform, no install beyond
   `pip install flask`, and naturally suited to streaming a running job's
   log/status via polling.

   "Run a new scan" shells out to `pwsh` for Stage 1
   (`Invoke-FlexAppInventory.ps1`, unavoidable - it's PowerShell), captures
   its stdout/stderr into a per-job log, parses the `Wrote
   <path>.inventory.json` line it prints to find the result without
   assuming a naming convention, then calls Stage 2's `flexapp_vuln`
   functions **in-process** (`resolve_vuln_matches`, `compute_coverage`,
   `build_sbom`, `render_coverage_report`/`render_findings`,
   `render_pdf_report` - the exact functions `cli.py`'s `resolve`/`report`
   commands call) rather than shelling out to a second Python process.
   Renamed `cli.py`'s `_package_display_name` to `package_display_name` so
   both the CLI and the web UI share one implementation instead of the web
   UI reaching into a "private" name or duplicating it.

   "Open an existing scan output folder" reuses the same Stage 2 write
   path (`jobs.write_reports`, shared with the fresh-scan flow so the two
   code paths can't quietly diverge) against any directory containing a
   `*.inventory.json`, loading a sibling `vuln-matches.json` if one already
   exists rather than re-querying OSV/NVD - the same "no network needed"
   property `report` already has.

   Jobs run in a background thread with an in-memory registry (no
   database - this is a local, single-user tool, see `webui/README.md`).
   Download links deliberately never carry a raw filesystem path from the
   browser: every scan (fresh or opened) gets a random id, and
   `/download/job/<id>/<kind>` / `/download/open/<id>/<kind>` only serve
   the exact paths this process already computed for that id - chose this
   over a generic `?path=` parameter specifically to avoid building an
   arbitrary-file-read primitive, even though the realistic risk is low for
   a `127.0.0.1`-only local tool. The dev server binds to `127.0.0.1` only,
   documented as a hard requirement (not a default to casually change) in
   `webui/README.md`'s "Security" section.

   Verified with 11 Flask-test-client tests (Stage 1 missing-`pwsh`/
   missing-script error path; the full Stage 2 + PDF pipeline via
   `load_existing_result` against real fixture data, no network; HTTP
   routes - missing-field validation, unknown-job 404s, download-kind
   validation). Then live-validated beyond unit tests: ran the actual dev
   server, drove it with a headless browser against the real Nextcloud
   Client scan output directory (not the small test fixture) via "Open an
   existing scan output folder," and confirmed the same 71.6% coverage,
   34 confirmed findings, and 56 unresolved components the CLI/PDF path
   already produced for that package - screenshotted both the dashboard
   and the results page to visually confirm table rendering and severity
   color-coding render correctly, not just that the numbers matched.

   **Follow-up, same day: a filesystem browser for the path fields.**
   Requested directly after trying the UI - typing/pasting full paths by
   hand into "Run a new scan"/"Open an existing scan output folder"
   defeated some of the point of having a UI at all. Added a small
   server-side directory browser (`browse.py` + a `/browse` route +
   `browse.html`): drives → folders → files, defaulting to this repo's own
   directory. Two modes share the same view - file-picker mode (package
   path) filters to `.vhdx`/`.exe`/`.flexapp` only and has no "select this
   folder" affordance; directory-picker mode (output dir / open-existing
   dir) shows folders only with a "Select this folder" button. Selecting
   an entry redirects back to `/` with the chosen path as a query
   parameter, which the dashboard now reads to prefill the right input -
   including on validation-error re-renders, so a partially-filled form
   never loses what you'd already typed or browsed to.

   Not a new trust boundary: this process already runs arbitrary local
   PowerShell against whatever path you type in, so listing directory
   contents on request doesn't grant it anything it didn't already have -
   documented explicitly in `webui/README.md`'s "Security" section rather
   than left implicit. Verified with 9 new tests (`browse.py`'s
   dir/file-extension filtering and nonexistent-path handling directly;
   the `/browse` route's mode-specific rendering, unknown-target
   rejection, and error handling through Flask's test client) plus a
   live, driven-by-a-headless-browser round trip: opened the browse view,
   navigated into a real subdirectory, clicked "Select this folder," and
   confirmed the dashboard came back with that exact path pre-filled.

   **Bug found immediately on real use, same day**: selecting a package
   file, then browsing for the output directory, silently dropped the
   already-selected package path. Cause: every link on the browse page
   only carried the field currently being edited (`target`), not the
   other two path fields' current values - so navigating away to pick the
   output directory lost whatever had already been picked for the package
   path. Fixed by threading all three field values ("carry") through
   every link on the browse page (drives, up, subfolder navigation) and
   the final select-file/select-folder links, computed server-side in the
   `/browse` route rather than attempted in the template (Jinja's `|` is
   reserved for filters, so dict-merging inline in `browse.html` wasn't a
   clean option - precomputed full URLs in `app.py` and handed the
   template plain `(name, url)` pairs instead). Verified with a real
   headless-browser round trip (select a package file, then browse for
   and select an output folder, confirm both fields still hold their
   values) and 2 new regression tests that parse the actual rendered
   `href`s and check `package_path` survives on every link when browsing
   for `output_dir`, not just asserting on the final state.

9. **New, same day: rebranded the web UI to Liquidware's real design
   system.** The project owner provided an actual style-guide export
   (`Liquidware_style_guide_baseline.zip`, the "Stratusphere UX" design
   system - colors, Inter font, PrimeIcons/Material Symbols icon fonts,
   logo SVGs, a full README documenting the product's content voice and
   visual language) rather than asking for a guess. Extracted and applied
   it directly: primary blue scale, zinc-based neutral scale, the brand's
   radii/shadow/spacing tokens, Inter variable font, the flame/droplet
   logo in the header and as favicon, and a Title Case copy pass across
   headings and form labels per the design system's own documented
   content rules (Title Case for UI chrome, imperative-verb buttons, no
   emoji anywhere).

   Two deliberate, documented substitutions rather than literal
   compliance:
   - **CVE severity colors do not reuse GFP (Good/Fair/Poor).** GFP is
     Stratusphere's real data-language and green there means "healthy" -
     no CVE severity, not even LOW, is an honest "good" in that sense, so
     reusing green for a real vulnerability finding would misrepresent it.
     Severity badges (web UI and `pdf_report.py`, kept in sync) map only
     to the brand's "poor" (red-600) and "fair" (amber-600) tones, plus a
     darker red for CRITICAL and neutral gray for LOW/UNKNOWN.
   - **PrimeIcons (the design system's default icon font) was not used**,
     even though the guide names it first: the export's own README flags
     that the `.woff2` binary isn't included, only CSS class definitions
     meant to load the font from a CDN in previews. Pulling an icon font
     from a CDN would give this otherwise self-contained, offline-friendly
     local tool's own page rendering a new network dependency it doesn't
     need for anything else. Used **Material Symbols Rounded** instead -
     genuinely bundled locally in the export, Apache-2.0 licensed, and
     explicitly documented as the design system's own fallback icon font
     where PrimeIcons lacks a glyph. Replaced every emoji glyph in
     `browse.html`/`index.html` (folder/file/up-arrow/select) with it.

   Verified with a real headless-browser pass across the dashboard, the
   browse view (drives/folders/files, the "Select This Folder" button),
   and a results page rendered from real Nextcloud Client scan data -
   logo, Inter font, brand blue, dark zinc table headers, and the
   red/amber severity coloring all confirmed rendering correctly, not
   just asserted from the CSS source. All 22 `webui` tests and 104
   `stage2-resolve` tests still passing (2 needed updating for the Title
   Case copy change, e.g. "Run a New Scan").

   Also added `Spark_License.pdf` (Liquidware's standard Sparks Tool
   license/disclaimer) to this project's root, at the project owner's
   request - confirmed by hash it's byte-identical to the copy already
   shipped in the sibling `FlexAppOneDownloadMonitor` project, so both now
   carry the same file rather than two independently-sourced copies.

10. **Same day: self-audit against the Sparks Tool Project Review
    Checklist v1, then applied the approved fixes.** Followed the
    checklist's own explicit protocol (which matches this repo's
    CLAUDE.md working agreement): read-only audit of all 7 items first,
    findings reported with a Blocking/Should-fix split, explicit
    go-ahead requested before any edit - "OK" received, then implemented.

    Findings, before any fix: encoding was already solid everywhere
    except two unguarded `Get-Content` calls on a PowerShell failure
    path; no US-only date parsing anywhere, all internal timestamps
    already ISO 8601 UTC; no CDN/telemetry/hardcoded-credential/
    disabled-TLS issues found; **no SBOM existed for this tool's own
    dependencies** (distinct from `sbom.py`'s scanned-package output -
    a real gap, not a naming confusion); **no user-facing version
    display** (a version string existed internally but never reached a
    CLI flag or the webui); `Spark_License.pdf` was at the root already
    but with no SBOM to sit beside it and no disclaimer banner in
    `README.md` yet; no third-party notices file existed.

    Fixes applied: generated `bom.cdx.json` from a **clean venv**
    (`cyclonedx-py environment` against a fresh install of exactly
    `requirements.txt` + `webui/requirements.txt`, not this dev
    sandbox's already-polluted environment) so the resolved-version tree
    reflects what a real install produces, not a guess. Excluded `pip`/
    `setuptools` from the final SBOM - they're venv bootstrap machinery
    present in any environment, not something this tool's code actually
    imports, and including them would misrepresent what the tool
    depends on. Validated the result against the real CycloneDX 1.6 JSON
    schema **offline**, using `cyclonedx-python-lib`'s own validator
    (which resolves its `$ref`s from locally-bundled schema files) rather
    than raw `jsonschema` with a resolver that tries to fetch schema
    fragments over the network and fails in this sandbox. All 22
    components came back permissively licensed (MIT/BSD-3-Clause/
    Apache-2.0/MPL-2.0/PSF-2.0/MIT-CMU) - confirmed by inspecting the
    actual license metadata, not assumed from memory - so no escalation
    was needed for copyleft/source-available-but-not-open licenses.

    Built `THIRD-PARTY-NOTICES.txt` by pulling each component's real
    bundled license file out of its installed dist-info (found for all
    but a couple, after fixing a package-name-normalization bug in the
    first attempt - dist-info folder names replace `-` with `_` in the
    name segment, which broke a naive substring match), falling back to
    the standard SPDX template text only for the couple of packages whose
    own distribution didn't bundle a license file locally, noted
    inline rather than silently substituted.

    Added `python -m flexapp_vuln --version` (reads the existing
    `flexapp_vuln.__version__` - one source of truth, not a second
    hardcoded string) and a footer on every webui page showing the same
    version plus links to `/license` and `/sbom` (both fixed server
    paths serving files at the repo root - no user-supplied path
    involved, so this doesn't reopen the arbitrary-path-read question
    the download routes already closed off for scan results).

    Fixed `Expand-FlexAppOne.ps1`'s two unguarded `Get-Content` calls
    (failure-path stderr/stdout capture reads) to specify `-Encoding
    UTF8` explicitly - low blast radius, since it only affects an
    already-thrown diagnostic message's text, not a success-path
    artifact anything downstream depends on.

    Added the checklist-required disclaimer banner to the top of
    `README.md` (matching `FlexAppOneDownloadMonitor`'s wording) plus a
    per-item compliance status table, and verified with 3 new webui
    tests (footer shows the live version; `/license` serves a real PDF;
    `/sbom` serves valid CycloneDX JSON when present) plus a live check
    of both routes against the running dev server. All 104
    `stage2-resolve` + 25 `webui` tests passing.

    **The Grype CVE scan (§5), completed same day**: `grype` isn't
    installed in this sandbox and `grype.anchore.io` is blocked here
    regardless (same precedent as OSV/NVD), so the project owner ran it
    directly against the final `bom.cdx.json` on their own machine and
    supplied the report. **Zero matches of any severity** across all 22
    components (Grype v0.117.0, vulnerability DB schema v6.1.9, built
    2026-08-13T06:39:30Z, scan run 2026-08-13). Saved as
    `grype-report.json` at the repo root and referenced from
    `README.md`'s compliance table. All 7 checklist items now show Pass.

    This remains a self-audit, not a formal SE-signed Sparks Tool
    submission - the checklist's attestation section is intentionally
    left blank, since that requires a named human reviewer's sign-off,
    not something an AI assistant can complete on someone's behalf.

11. **New, same day: every finding's ID links to its real public
    record.** Requested directly. Added `reporting.py`'s
    `vulnerability_url()`: CVE ids resolve against NVD (the
    authoritative record regardless of which source - OSV or NVD -
    actually matched it), GHSA ids against GitHub's advisory database,
    and everything else OSV-sourced (PYSEC-, RUSTSEC-, GO-, ...) against
    osv.dev, which mirrors and links out to the real upstream advisory
    for whatever ecosystem it came from. Wired into `build_finding_rows`
    (shared by all three renderers, so the Markdown table, the web UI,
    and the PDF can never disagree on what a given id links to) rather
    than computed separately in each.

    `render_findings`'s Markdown table now emits `[CVE-...](url)`;
    `result.html` wraps the id in a real `<a href>`; `pdf_report.py` uses
    reportlab's `<link href="...">` mini-markup (colored brand blue,
    underlined) - reportlab's own hyperlink support, not a fake-looking
    styled span. Two pre-existing tests needed a small update (they
    counted raw CVE-id substring occurrences, which doubled now that the
    id also appears inside its own URL - fixed to count the link-wrapped
    form specifically, `[CVE-...]`, rather than loosening the assertion).

    Verified with 5 new `reporting.py` tests (URL scheme per id pattern,
    `None` for a missing id, the id showing up in `build_finding_rows`'
    output, the Markdown link form), 1 new webui test (a real `<a href>`
    to NVD appears in the rendered results page), and visually: a
    headless-browser screenshot of the real Nextcloud Client findings
    table with every CVE ID showing as a clickable link, plus rendering
    the PDF and confirming a link is actually clickable in it (not just
    styled to look like one).

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
