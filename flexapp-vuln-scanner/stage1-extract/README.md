# Stage 1 — extraction & inventory

PowerShell 7 only. Produces one `<package-basename>.inventory.json` per
package in `-OutputDir`. See `../schemas/inventory.schema.json` for the
exact output contract and `../PLAN.md` for the design rationale.

This pass does **not** resolve version identity or apply the exclusion
ruleset yet — every file gets `identity: null`, `excluded: false`, and a
best-effort `componentType` guessed from its extension/name. That's the next
increment (see `PLAN.md`'s build order).

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

1. Classic `.vhdx`: mounted read-only via `Mount-DiskImage`, always dismounted
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
5. The whole thing is written out as one inventory JSON. Nothing else leaves
   the machine.

## Known limitations at this stage

- Directory input is non-recursive (top-level packages only).
- No exclusion filtering yet — every walked file is reported.
- No identity resolution yet — that's PE/.NET/Java/Node/Python/string-scan
  parsing, coming next.
- `Mount-ClassicFlexApp.ps1` and `Expand-FlexAppOne.ps1` require Windows and
  a real package to validate — untested against the real FlexApp One
  executable and VHDX mount path in this environment.
