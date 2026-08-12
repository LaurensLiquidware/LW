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
flexapp-vuln-poc/
  stage1-extract/                      # PowerShell 7, runs on/near the package store
    Invoke-FlexAppInventory.ps1        # entry point: dispatches by extension
    Mount-ClassicFlexApp.ps1           # Mount-DiskImage -ReadOnly + guaranteed dismount
    Expand-FlexAppOne.ps1              # flexappone.exe --extract wrapper
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
      ]
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
   - **`VersionMajor`/`Minor`/`Build`/`Revision` were all `0` in this
     sample** — present in the schema but apparently not populated on this
     capture. Treat this as unreliable/optional, not a dependable version
     source, until a package with non-zero values turns up.
   - **The only real version signal is buried in free text inside `History`**:
     `"   Version 6.9.5.9678, 6.9.5.9678 Wed 07/01/2026+2c00aca1f87b2c3f690ec2e94a38fcd923ab779e"`.
     This needs a small regex extractor (`Version ([\d.]+)`), and it's a
     second-tier signal — corroborating evidence for the file-level identity
     resolution in §"Version identity extraction," not something to trust
     standalone (it's admin-entered free text, not derived from the binary).
   - **`Links[].Target` is the most useful field here**: it gives the
     absolute installed path of the shortcut'd executable(s)
     (`C:\Program Files (x86)\WinSCP\WinSCP.exe`). Stage 1 will capture these
     as `shortcutTargets` and Stage 2 can use them to identify the package's
     "primary" component with higher confidence than treating every `.exe`
     in the tree as equally significant.
   - **`Icon` is a large embedded base64 PNG** — irrelevant to component
     resolution. `Read-PackageMetadataXml.ps1` will explicitly skip/drop this
     field rather than pass it through to the JSON inventory (no reason to
     bloat the stage1→stage2 payload with an icon on every package).
   - Still open: a FlexApp One sample's metadata (this one is `PackageType:
     Vhd`, i.e. classic) — if FlexApp One packages carry the same
     `.package.xml` sidecar format, great; if not, this parser needs a
     second branch.
3. **FlexApp One (.flexapp/.exe) format**: I'm assuming `flexappone.exe`
   ships with (or alongside) the packages and supports a documented
   `--extract` flag that unpacks to a normal directory tree. Please confirm
   the exact CLI syntax and where that binary normally lives in your
   environment — I don't want to guess flags against a binary I can't run
   from here.
4. **Scale**: one real data point now — the winscp sample declares
   `SizeInGb: 10` (the max/provisioned size, `VirtualDiskType: Expandable` —
   i.e. a sparse VHDX) but `ActualSizeInBytes: 306184192` (~292 MB), which is
   the number that actually matters for how much stage 1 has to walk/hash.
   Still need a rough sense of your larger, messier enterprise packages
   (the brief mentions "large installer-bundled enterprise applications with
   messy internals" as the first real test target) — is 292 MB representative
   of the low end and multi-GB the norm for those? That determines whether
   the SHA-256 pass needs throttling/parallelism from day one or can stay
   simple for v1.
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
