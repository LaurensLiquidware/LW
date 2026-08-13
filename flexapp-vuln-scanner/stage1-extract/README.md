# Stage 1 — extraction & inventory

PowerShell 7 only. Produces one `<package-basename>.inventory.json` per
package in `-OutputDir`. See `../schemas/inventory.schema.json` for the
exact output contract and `../PLAN.md` for the design rationale.

Every non-excluded file gets identity resolution (PE/.NET, Java jar/war
including nested fat-jar components, Node package.json including inside
app.asar, Python dist-info/egg-info, and string-signature scanning as a
last resort) and every file gets exclusion filtering
(`ExclusionRules.psd1`). Jar/asar containers can contribute extra synthetic
`files[]` entries for nested components (see "Nested components" below).

## Usage

```powershell
# A single classic FlexApp package
.\Invoke-FlexAppInventory.ps1 -Path 'D:\FlexAppShare\winscp_20260730160821.vhdx' -OutputDir 'C:\scan-out'

# A single FlexApp One package (self-extracting exe)
.\Invoke-FlexAppInventory.ps1 -Path 'D:\FlexAppShare\OBS-Studio.exe' -OutputDir 'C:\scan-out'

# A folder of packages (non-recursive; .vhdx/.exe/.flexapp only)
.\Invoke-FlexAppInventory.ps1 -Path 'D:\FlexAppShare' -OutputDir 'C:\scan-out'
```

Add `-Verbose` to see each mount/walk step as it happens.

## What it does

1. Classic `.vhdx`: mounted read-only via `Mount-DiskImage`, then mounted to
   a scratch folder via `Add-PartitionAccessPath` (confirmed on a real
   Windows 11 host: Windows does not reliably auto-assign a drive letter to
   a VHDX volume even though the disk/partition come online healthy - so
   this is not optional, waiting longer never helps), always dismounted
   in a `finally` block (plus an engine-exit safety net for Ctrl-C).
2. FlexApp One `.exe`/`.flexapp`: unwrapped via `<Package>.exe --extract <tmp> --skipico`
   (hardcoded — see `Expand-FlexAppOne.ps1`'s safety note on why no other
   flag is ever passed to that executable), then the resulting `.vhdx` is
   mounted exactly like a classic package.
3. The sidecar `<basename>.package.xml` (next to the `.vhdx`, or produced by
   the FlexApp One extraction) is parsed for package-level metadata.
   `<Icon>`, `<license>`, and `<CallToHome>` are deliberately never read into
   the output — `<license>` in particular can carry a real person's contact
   details, unrelated to the packaged app.
4. Every file on the mounted volume is walked, hashed (streaming SHA-256,
   safe for very large files), and given a best-effort `componentType`.
5. Every walked file is checked against `ExclusionRules.psd1` (path/name
   heuristics — OS system paths, resource-only assemblies, debug symbols,
   fonts/icons/media, satellite culture resources). Excluded files are
   still reported (`excluded: true`, `exclusionReason: "..."`) - never
   silently dropped.
6. Every non-excluded file goes through `Resolve-VersionIdentity.ps1`'s
   dispatcher, in PLAN.md's priority order: a managed (.NET) assembly check
   first (more precise than the Win32 resource compilers also embed),
   falling back to the Win32 version resource for native PE; JAR/WAR via
   `META-INF/maven/*/*/pom.properties` (highest confidence anywhere in this
   pipeline) or `MANIFEST.MF`, recursing into nested jars; `package.json`
   directly or inside `app.asar`; Python `dist-info`/`egg-info`; and
   string-signature scanning (`config/string-signatures.psd1`) as a last
   resort for anything still unresolved.
7. The whole thing is written out as one inventory JSON. Nothing else leaves
   the machine.

## Nested components

A single physical file can contain more than one component: a Spring Boot
fat jar bundles dependency jars in `BOOT-INF/lib/`, and an Electron
`app.asar` can contain several `package.json`s. These show up as extra
synthetic entries in `files[]`, using a `<real path>!/<inner path>`
convention, e.g.:

```
Program Files\App\outer-app.jar
Program Files\App\outer-app.jar!/BOOT-INF/lib/inner-lib-2.15.3.jar
```

Each carries its own `sizeBytes`/`sha256` (of the nested entry's bytes,
where computable) and `identity`, exactly like a physical file would.

## Live validation (2026-08-13)

Run against a real OBS Studio package on a real Windows host, both as
classic VHDX and FlexApp One — byte-identical results (2170 files, zero
crashes, zero read errors). This is what caught the drive-letter issue
above, and drove the addition of `.ini`/`.pak`/`.effect` to
`ExclusionRules.psd1` (config/locale/resource-pack data files, not
components — see `../PLAN.md`'s resolved assumption 1 for the full
before/after coverage numbers on that package).

## Known limitations at this stage

- Directory input is non-recursive (top-level packages only).
- Exclusion is a path/name heuristic only - the PLAN.md stretch goal of
  hashing against a known-good clean-Windows-install set is not implemented.
- `string-signatures.psd1`'s pattern set is narrower than a real package
  needs - the live test found real third-party libraries (`lua51.dll`,
  `librist.dll`, `srt.dll`, `datachannel.dll`) with neither a Win32 version
  resource nor a matching pattern. Left unresolved rather than guessing at
  byte content not inspected directly.
- Electron's bundled Chromium/Node versions are resolved via string-signature
  scanning of the main executable (`method: electron-embedded`), not by
  inspecting `app.asar` - the archive reader here only extracts
  `package.json` metadata. (In the live test, `libcef.dll`'s own Win32
  version resource already embedded its Chromium version, so this path
  wasn't needed for that package - but it remains the fallback for apps
  that don't embed it that way.)
