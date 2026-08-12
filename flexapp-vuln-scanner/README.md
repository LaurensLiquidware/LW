# FlexApp Vulnerability Scanner (PoC)

A feasibility measurement, not a product: for a real enterprise FlexApp
package, what percentage of the third-party components inside it can be
resolved to a version identity precise enough to match against a
vulnerability database? See [`PLAN.md`](PLAN.md) for the full design
rationale, the exact coverage-percentage definition, and the assumptions
this project makes about FlexApp internals.

## Status

| Stage | Status |
|---|---|
| Stage 1 — extraction & inventory (PowerShell 7) | **Built.** Mount/extract, XML metadata, file walk/hash, exclusion filtering, identity resolution. See below for what's validated vs. still needing a real Windows host. |
| Stage 2 — resolution & vulnerability matching (Python) | Not started. OSV.dev matching, NVD matching, and reporting (`sbom.cdx.json`, `coverage-report.md`, `findings.md`) are next per `PLAN.md`'s build order. |

## What it does

Classic FlexApp (`.vhdx`) and FlexApp One (self-extracting `.exe`) packages
seal their application install into a container that OS-level inventory and
conventional vulnerability scanners can't see into. Stage 1 mounts/extracts
a package read-only, walks every file, and resolves as many components as
possible to a `vendor:product:version` identity using the highest-confidence
method available for each file type — without the package or any of its
contents ever leaving the machine it's scanned on. The result is a single
JSON inventory file (see [`schemas/inventory.schema.json`](schemas/inventory.schema.json))
that Stage 2 will consume to answer the coverage question.

## Prerequisites

- **Stage 1**: PowerShell 7 on Windows, with permission to mount VHDX images
  (`Mount-DiskImage`) and network access to the package store. No external
  PowerShell modules — Stage 1 is deliberately dependency-free.
- **Stage 2**: Not yet implemented. Will require Python 3.11+ and a pinned
  `requirements.txt` (per `PLAN.md`) once built.

## Running Stage 1

See [`stage1-extract/README.md`](stage1-extract/README.md) for full usage,
flags, and a description of every step it performs. Quick version:

```powershell
# A single classic FlexApp package
.\stage1-extract\Invoke-FlexAppInventory.ps1 -Path 'D:\FlexAppShare\winscp_20260730160821.vhdx' -OutputDir 'C:\scan-out'

# A single FlexApp One package (self-extracting exe)
.\stage1-extract\Invoke-FlexAppInventory.ps1 -Path 'D:\FlexAppShare\OBS-Studio.exe' -OutputDir 'C:\scan-out'

# A folder of packages (non-recursive; .vhdx/.exe/.flexapp only)
.\stage1-extract\Invoke-FlexAppInventory.ps1 -Path 'D:\FlexAppShare' -OutputDir 'C:\scan-out'
```

Each package produces one `<package-basename>.inventory.json` in
`-OutputDir`.

## Running Stage 2

Not yet built.

## Known limitations

- **Stage 2 doesn't exist yet** — there is currently no way to go from a
  Stage 1 inventory JSON to a coverage number, SBOM, or vulnerability
  findings.
- **Directory input to Stage 1 is non-recursive** (top-level packages only).
- **Exclusion is a path/name heuristic**, not a hash comparison against a
  known-good clean-Windows-install set (that's a stated stretch goal in
  `PLAN.md`, not implemented).
- **`Mount-ClassicFlexApp.ps1` and `Expand-FlexAppOne.ps1` are unverified
  against a real Windows host.** This project has been developed and
  functionally tested inside a Linux environment with PowerShell 7
  installed for that purpose — every module that doesn't require
  Windows-only APIs (XML metadata parsing, the file walk/hash, exclusion
  rules, and PE/.NET/Java/Node/Python/string-signature identity resolution)
  has been syntax-checked and smoke-tested against real artifacts (real
  `.package.xml` samples, real jars built with the JDK's own `jar` tool,
  a real `app.asar` built with the `asar` npm package), with hashes
  cross-checked against `sha256sum` and full inventory output validated
  against `schemas/inventory.schema.json`. Only the two functions that call
  `Mount-DiskImage`/`Dismount-DiskImage` and invoke a real FlexApp One
  package executable have not been run for real — that needs a Windows host
  and an actual package, which this development environment doesn't have.
- **No CVE data yet.** Stage 1 resolves identity only; nothing has been
  checked against OSV.dev or NVD until Stage 2 exists.
- **flexappone.exe assumptions** — the FlexApp One CLI reference used here
  came from documentation pasted into this project's development
  conversation, not fetched independently (the doc site is blocked by this
  environment's network policy), plus one confirmed real test
  (`OBS-Studio.exe --extract C:\FA1`). Exit code / stderr behavior on a
  failed extraction is still undocumented — see `PLAN.md`'s resolved
  assumption 3 for how Stage 1 handles that.

## Non-goals (this PoC)

No GUI, no web service, no database, no ProfileUnity console integration,
no Stratusphere correlation, no remediation/repackaging automation, and no
attempt to handle every application type. See `PLAN.md` for the full
rationale.
