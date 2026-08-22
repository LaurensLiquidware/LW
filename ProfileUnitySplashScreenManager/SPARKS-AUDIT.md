# Sparks Tool Project Review — Audit Report

**Project:** ProfileUnitySplashScreenManager
**Reviewed against:** Sparks Tool Project Review Checklist v1
**Audit date:** 2026-08-22
**Files reviewed:** `Set-ProfileUnitySplashScreenLogo.ps1` (637 lines), `Build-Exe.ps1`, `app-icon.ico`, `README.md`, `CLAUDE.md`
**Reference material supplied:** Sparks Tool License PDF, `Spark_Liquidware_style_guide_baseline.zip`, PrimeUI license key
**Phase:** 1 — audit complete, **no files have been edited**. Phase 2 summary is at the bottom. Awaiting approval before any change.

---

## Summary table

| # | Item | Status | Notes |
|---|------|--------|-------|
| 1 | Double-byte / Unicode handling | **Fail** | BOM-less source; two unencoded JSON reads; **no `-LiteralPath` anywhere** — a `logo[1].png` from a browser breaks the copy |
| 2 | Regional date, time, number formats | **Fail** | Culture-dependent timestamp *write* + culture-invariant *read* — breaks the history grid on some locales; local time, no offset |
| 3 | External URL / CDN references | **Fail (disclosure only)** | One retained endpoint (Google image search) is undisclosed in the README. No CDNs, no secrets, no telemetry, git history clean |
| 4 | Open source identified + CycloneDX 1.6 JSON SBOM | **Fail — blocking** | No SBOM. Shipped components: zero. One decision needed re: ps2exe |
| 5 | Zero Critical / High CVEs (Grype scan of SBOM) | **Not run — blocking** | Grype unavailable in this sandbox, and §4's SBOM must exist first |
| 6 | Version number visible to end user | **Fail — blocking** | No version in the tool at all; the only version string (`1.0.0.0`) contradicts the README's v1.7. No `CHANGELOG.md` |
| 7 | License PDF + SBOM packaged and visible | **Fail — blocking** | Neither file present; README carries none of the required disclaimers; no About screen |
| 8 | UI consistency (style guide / PrimeNG) | **Mostly Pass** | Every colour token exact, both brand marks verified faithful to the source SVGs. Two off-scale type sizes to fix |

Also recorded below: four correctness findings (C1–C4) found while reading, outside the eight checklist items.

---

## 1. Character encoding — double-byte and non-Latin input

**Status: Fail**

**What I checked:** source-file encoding, every file read/write, all string-truncation and regex logic, path handling, filename construction, clipboard round-trip, and the WPF display path.

### F1.1 — Source files are BOM-less ASCII *(should fix, mechanical)*

`Set-ProfileUnitySplashScreenLogo.ps1` and `Build-Exe.ps1` are both plain ASCII with no byte-order mark (confirmed with `file`). Fine today because every literal in both files is ASCII, but Windows PowerShell 5.1 — which this script targets — reads a BOM-less `.ps1` using the host's **active code page**, not UTF-8. The first time anyone adds a non-ASCII literal (a translated status message, an em-dash in a comment), it silently mojibakes on a differently-configured machine.

This is the identical finding that was fixed in `FlexAppOneDownloadMonitor` (round 2 of that review), so there's precedent for the fix.

**Fix:** re-save both files as UTF-8 with BOM. Encoding-only, no logic change — but re-confirm PowerShell 5.1 still parses the file afterwards.

### F1.2 — Two JSON reads with no explicit encoding *(should fix, mechanical)*

- `Get-Content $ManifestPath -Raw` — `:81`
- `Get-Content $CurrentMetaPath -Raw` — `:96`

Both writes *are* explicit (`Set-Content -Encoding UTF8`, `:91`, `:156`, `:176`), and PS 5.1's `-Encoding UTF8` emits a BOM, which `Get-Content` then detects — so the round trip works today. It breaks if either JSON file is ever rewritten by another tool as BOM-less UTF-8 (an MDM script, a text editor, a config-management push) and the host code page isn't UTF-8.

This is not hypothetical for this tool specifically: `manifest.json` stores `OriginalName`, which is a **user-chosen image filename** and can legitimately be `会社ロゴ.png` or `Ångström-logo.png`.

**Fix:** add `-Encoding UTF8` to both reads.

### F1.3 — No `-LiteralPath` anywhere; wildcard metacharacters in filenames break the tool *(should fix — this one is a live bug, not a latent one)*

Every filesystem call in the script uses `-Path` (or positional path) semantics, which interpret `[`, `]`, `*` and `?` as wildcard patterns. There is not a single `-LiteralPath` in the file.

Why this matters here more than in most tools: the documented workflow is *search the web for a logo → save or copy it → Browse to it*. **Browsers name repeat downloads `logo[1].png`, `logo(1).png`, `image[2].jpg`.** A `[1]` in the filename is a character-class wildcard that matches nothing, so:

1. `Load-ImagePreview` → `Test-Path $path` returns `$false` → preview silently shows nothing (`:460`).
2. `Set-PendingPreview` enables **Set as Splash Logo** anyway (`:524`) — the guard only checks the extension.
3. `Set-NewLogo` → `Copy-Item -Path $SourcePath` throws `ItemNotFoundException` (`:149`) → caught → the user sees a generic "An error occurred: Cannot find path…" for a file they can plainly see on disk.

Worse: `Archive-CurrentLogo` has already run by then (`:146`), so the previous live logo has been moved to history and deleted from `Client.NET`. **The machine is left with no live splash logo and a confusing error.** It's recoverable via Restore, but the user has to work that out.

Affected lines: `79`, `95`, `102`–`103`, `116`, `128`–`129`, `133`, `149`, `162`–`164`, `169`, `184`–`185`, `460`, `484`, `596`.

**Fix:** switch every one to `-LiteralPath`. On `:103`, use `-LiteralPath $TargetDir` and keep `-Filter` (the filter is a deliberate wildcard and should stay). Mechanical, but it touches the write path, so it needs re-testing.

### F1.4 — Archived-filename sanitising is already Unicode-correct *(pass, no fix)*

`:115` — `([IO.Path]::GetFileNameWithoutExtension($originalName) -replace '[^\w\-]', '_')`. PowerShell's `-replace` is .NET regex, where `\w` is **Unicode-aware** by default, so `会社ロゴ.png` archives as `20260822-143900__会社ロゴ.png` rather than being flattened to underscores. Correct as written.

Consequence worth knowing: the History folder can therefore legitimately contain non-ASCII filenames, which is exactly why F1.2 and F1.3 need fixing — the manifest that names those files is read with a default encoding, and the files themselves are opened with wildcard-expanding paths.

### F1.5 — Clipboard temp files: collision and no cleanup *(note)*

`:205` — `"pu-logo-search-{0}.png" -f (Get-Date -Format 'yyyyMMdd-HHmmss')`. Second-level precision, and `[IO.FileMode]::Create` truncates, so two clipboard imports inside the same second silently overwrite each other. Harmless in practice (the first was already previewed and, if committed, already copied), but a GUID or `.NewGuid()` suffix is free. These temp PNGs are also never cleaned up — they accumulate in `%TEMP%` for the life of the profile.

### Evidence gap

**No double-byte round-trip test has been run.** This is a WPF app and cannot be executed in this Linux sandbox. Per the checklist §1 evidence requirement, this needs a real Windows run with Japanese / Simplified Chinese / Korean / Cyrillic / accented-Latin strings — at minimum one as a **source image filename** (the `OriginalName` path), plus one filename containing `[1]` to prove F1.3. `FlexAppOneDownloadMonitor` closed the same gap on your Windows VM in round 8; the same approach applies here.

---

## 2. Regional formats — dates, times, numbers

**Status: Fail**

**What I checked:** every `Get-Date`, `.ToString(...)`, cast and sort in the script; the manifest/meta JSON schema; the dimension comparison.

### F2.1 — Culture-dependent write, culture-invariant read: the history grid breaks on some locales *(should fix — real, locale-dependent failure)*

Three writes use a **custom** .NET format string:

```
(Get-Date).ToString('yyyy-MM-dd HH:mm:ss')     # :124 DateArchived, :154 and :174 DateSet
```

In a .NET *custom* format string, `:` is not a literal — it is the **time-separator placeholder**, substituted with the current culture's `DateTimeFormat.TimeSeparator`. Under a culture whose separator isn't `:`, this writes `2026-08-22 14.39.00` into `manifest.json`.

The read then does the opposite:

```
$manifest = @(Get-Manifest) | Sort-Object { [datetime]$_.DateArchived } -Descending   # :506
```

PowerShell's `[datetime]` cast parses with **InvariantCulture**, which will not accept `14.39.00`. The sort scriptblock throws, and **the history grid fails to populate** — on exactly the machines the checklist tells us to test on, and nowhere else.

So the bug is the mismatch: culture-sensitive on the way out, culture-invariant on the way back in.

**Fix:** make both ends invariant —
- write: `.ToString('yyyy-MM-dd HH:mm:ss', [System.Globalization.CultureInfo]::InvariantCulture)`
- read: `[datetime]::ParseExact($_.DateArchived, 'yyyy-MM-dd HH:mm:ss', [System.Globalization.CultureInfo]::InvariantCulture)`, with a tolerant fallback so entries already written by the current build (or by a differently-cultured machine) still sort instead of taking the grid down.

### F2.2 — Stored timestamps are local time with no offset *(should fix, needs a migration decision)*

`DateArchived` and `DateSet` are local wall-clock with no zone or offset. Checklist §2 is explicit: *"Store UTC with offset; convert for display."* These values are **sorted and compared** (`:506`), not merely displayed, so this isn't the "display may use locale" exemption. A machine that changes time zone, or a DST fall-back hour, produces a history list that sorts wrongly.

**Fix:** store round-trip (`'o'`) or `yyyy-MM-ddTHH:mm:sszzz` and format for display at render time. Higher blast radius than the rest: it changes the on-disk manifest format, so it needs a read-both-formats migration for manifests already in the field.

### F2.3 — Filename timestamps are safe *(pass, note)*

`Get-Date -Format 'yyyyMMdd-HHmmss'` (`:114`, `:205`) contains no culture-sensitive placeholder (`-` is a literal; there's no `:` or `/`), so it's stable across cultures and sorts lexicographically. Fine as-is.

### F2.4 — Dimension check compares formatted strings *(should fix, minor)*

`Get-ImageDimensionsText` returns `"$w x $h"` (`:193`), and callers compare that **string** against the literal `'300 x 86'` (`:497`, `:519`). PowerShell's string interpolation of an integer is invariant, so this happens to work — but it's the pattern the checklist calls out (compare values, not formatted strings), and it has a real edge: on a decode failure the function returns the literal `'unknown'` (`:194`), which is `-ne '300 x 86'`, so the user gets *"Recommended size is 300x86 — this file is unknown."*

**Fix:** return width and height as integers, compare `$w -ne 300 -or $h -ne 86`, and handle the decode-failure case with its own message.

### Clean

- **No date is ever parsed from user or external input** anywhere in the codebase — the `MM/DD/YYYY` ambiguity class simply doesn't arise.
- No numeric parsing at all, so no decimal-comma exposure.
- No 12-hour clocks, no hardcoded English month/day names, no week-numbering, no currency.
- Image dimensions come from `System.Drawing` as integers, never via a locale-formatted string.

### Evidence gap

Needs a run under a non-US locale and a non-zero-offset time zone (`de-DE` or `ja-JP`, and something like UTC+9) alongside the US baseline. A locale with a non-`:` time separator is the one that actually exercises F2.1.

---

## 3. External references — URLs, CDNs, and remote code

**Status: Fail on disclosure only — the code itself is clean**

### External references found, and disposition

| Reference | Where | Disposition |
|---|---|---|
| `https://www.google.com/search?tbm=isch&q=<term>` | `:535`, `:552` | **Retained** — needs disclosure (see below) |
| `http://schemas.microsoft.com/winfx/2006/xaml*` | `:225`–`:226` | **Not an external reference.** XML namespace identifiers; XAML never dereferences them |
| `https://support.liquidware.com/hc/…/12914471137293` | `README.md:5` | **Retained** — public Liquidware KB, documentation only, never fetched at runtime |
| `https://www.powershellgallery.com` (implicit) | `Build-Exe.ps1:30` | **Retained, build-time only** — `Install-Module ps2exe`. Not in the shipped artifact; must still be disclosed (see §4) |

**Retained runtime endpoint, per the checklist's required detail:**

- **Host:** `www.google.com`
- **Purpose:** opens an image search in the user's **default browser** so they can find a logo. Nothing is embedded, fetched, parsed or scraped by the tool — it hands a URL to the shell (`Start-Process`) and the browser does the rest.
- **Data transmitted:** the search term the user types into the box, URL-encoded, plus whatever the user's own browser normally sends (cookies, UA). The tool itself transmits nothing.
- **TLS:** yes. No certificate validation is disabled anywhere in the project, and there is no plain-HTTP call.
- **Air-gapped behaviour:** the tool's core function — set, archive, restore, preview the splash logo — is entirely local and works fine with no network. On an egress-restricted machine, **Search** still reports *"Opened image search…"* because `Start-Process` succeeded; the browser then shows its own connection error. Nothing breaks, but the tool's own message is misleading.

**Findings:**

- **F3.1 (should fix):** the README documents the search feature but never discloses it as an outbound third-party endpoint, and there is no endpoint list anywhere in the project. §3 requires that list to exist, explicitly stated as "none" if empty — and here it isn't empty.
- **F3.2 (worth a decision, not a fix):** for air-gapped or search-restricted customers there's no way to turn Search off or point it elsewhere. A `-SearchUrlTemplate` parameter (defaulting to the current value) would cover both without changing default behaviour. Flagging as a decision because it's a feature change, not a defect.

### Clean — verified, not assumed

- **No CDN-loaded runtime dependencies.** All three brand assets are base64-embedded in the script. This is the single most common failure mode for AI-generated UI code and this project got it right; worth crediting explicitly.
- **No remote code execution / fetch-then-run** in the shipped script. The only `Install-Module` is in the separate build script.
- **No telemetry, analytics, error reporting or phone-home** of any kind.
- **No external fonts, images, favicons or tracking pixels.** Segoe UI is an OS font; nothing is loaded at runtime.
- **No AI/LLM or other third-party API endpoints.**
- **No internal Liquidware hosts, staging environments, internal IPs or non-public endpoints.**
- **No placeholder, personal or scratch URLs** — no pastebin, no personal repos, no tunnel hosts, no `localhost` assumptions, no invented documentation links. The one KB link is real and resolves to the article the tool implements.
- **No credentials, API keys, tokens, connection strings or webhook URLs** in source, config, comments, README, or **git history** — swept the full history (`git log -p --all`), zero matches.
- Comments, doc-comments, README, the XAML block and the embedded resources were all searched, not just executable code.

### PrimeUI / PrimeNG commercial license key — handling

The key supplied alongside this review is a signed JWT: `product: primeui`, `tier: commercial`, `type: dev`, **valid 2026-07-15 → 2027-07-15** — i.e. currently active, not expired.

- This project is a **WPF desktop application**. It has no Angular, no PrimeNG, no web framework, and no declared dependencies of any kind — so the key has no legitimate place in it.
- **The key does not appear in this project, in this report, in the SBOM to be generated, or anywhere in git history**, and it was not written to any file. Confirmed by sweep.
- Flagging rather than silently ignoring: it was delivered through a file upload, so it should be treated as **exposed in transit** and rotated on that basis alone, independent of anything in this repo. Nothing to remediate *here*.
- If a future Sparks submission does have an Angular/PrimeNG front end, this check must be re-run against that code — a clean result here says nothing about that.

---

## 4. Open source components and SBOM

**Status: Fail — blocking (no SBOM exists)**

### Component inventory

**Third-party components in the shipped artifact: zero.**

Everything the script binds to is Windows-inbox .NET Framework, present on the target OS and not redistributed by us:

| Assembly | Loaded at | Nature |
|---|---|---|
| `PresentationFramework`, `PresentationCore`, `WindowsBase` | `:34`–`:36` | WPF, .NET Framework |
| `System.Windows.Forms` | `:37` | `OpenFileDialog` only |
| `System.Drawing` | `:38` | image dimension read only |

There are no package-manager manifests, no vendored source, no bundled binaries, no containers. Nothing floating, nothing invented — I verified there is no dependency declaration to be wrong about.

**Brand assets are first-party, and I verified their provenance** rather than taking the handoff note on trust. Decoded all three embedded base64 blobs and compared against the supplied style guide:

| Embedded asset | Size | Verified against |
|---|---|---|
| `$Base64_LogoHeader` | 240×81 PNG, RGBA | `assets/logo-primary-light.svg` (viewBox 601×203 → aspect 2.960 vs 2.963 ✓). Sampled pixels: white wordmark + `#03a9f4` flame — matches `--lwl-flame` exactly |
| `$Base64_AppIcon` | 96×96 PNG, RGBA | `assets/logo.svg` (viewBox 1920×1920 ✓). Sampled pixels: `#0b72ba` circle — matches `--lwl-mark-circle` exactly — plus white flame |
| `$Base64_HexBg` | 420×300 PNG | `assets/lwl-hex.png` (2880×2766). A **crop**, not a proportional scale (aspect 1.40 vs 1.04) — see F8.5 |

All Liquidware-owned. No attribution obligations, no SBOM entries.

### F4.1 — No SBOM *(blocking)*

No `bom.cdx.json` exists. Per §4, a project with genuinely zero third-party components still ships a valid CycloneDX 1.6 JSON SBOM with an empty `components` list — "no open source" is a claim requiring evidence. `FlexAppOneDownloadMonitor/bom.cdx.json` is the working precedent to model.

### F4.2 — ps2exe scope: a decision, not a fix *(escalation)*

`Build-Exe.ps1` compiles the script with **ps2exe** (Markus Scholtes, MIT). This needs your call, because it changes the answer to §4:

- **If only the `.ps1` ships** → third-party components = **zero**, empty SBOM, no notices file needed.
- **If the `.exe` ships** → ps2exe isn't just a build tool: it *generates and embeds a host stub into the artifact you distribute*. That code is in the customer's hands, so it belongs in the SBOM (`pkg:powershell/ps2exe@<version>` or equivalent, `type: application`) with its MIT text in `THIRD-PARTY-NOTICES.txt` per License §3.

MIT is compatible with distribution under the Sparks Tool License either way — no copyleft, nothing source-available-but-not-open, nothing unlicensed. There is no license *problem* here; there's a scope question about what the SBOM must describe.

### F4.3 — ps2exe version is unpinned *(should fix)*

`Install-Module -Name ps2exe -Scope CurrentUser -Force -AllowClobber` (`Build-Exe.ps1:30`) resolves to whatever is latest at build time, and `Get-Module -ListAvailable` accepts any already-installed version. §4 requires pinned, non-floating versions. Two builds on two machines can embed different ps2exe versions — which matters more if F4.2 lands on "the exe ships."

**Fix:** `-RequiredVersion <x.y.z>`, chosen after confirming the version on your build machine, and record it in the README.

---

## 5. Vulnerabilities — no Critical or High

**Status: Not run — blocking**

Two things prevent closing this now, and neither is a code defect:

1. **§4's SBOM doesn't exist yet.** §5 requires Grype to scan *the SBOM*, not the source tree, and specifically the **final post-remediation** SBOM. There is nothing to feed it.
2. **Grype is not available in this environment** — no `grype`, `syft` or `cyclonedx` binary, and this is a Linux sandbox with no route to install and update a vulnerability database that would count as evidence.

**Expected result once run:** zero matches, because the component set is empty. That is a prediction, not evidence, and it is not a pass. `FlexAppOneDownloadMonitor` was in exactly this position and closed it in round 7 by running Grype on a machine with real internet access (v0.117.0, DB schema v6.1.9) — the same route applies here.

**What has to be recorded when it runs:** Grype version, vulnerability-database version and build date, the scan output saved to a file (not a screenshot), and the scan date — which must be within a few days of submission.

No component is abandoned, deprecated or pulled from a registry, since there are no components.

---

## 6. Version number visible to the end user

**Status: Fail — blocking**

### F6.1 — There is no version anywhere in the running tool *(blocking)*

The end user cannot determine what they're running without reading the source, which is precisely what §6 forbids. Confirmed by grep: no version constant, no About screen, no version in the window title, no startup banner, no log header. The tool writes no log at all.

### F6.2 — The one version string in the project contradicts the documentation *(blocking)*

`Build-Exe.ps1:42` hardcodes `-version '1.0.0.0'` in the exe's file metadata. `README.md`'s changelog ends at **v1.7**. So the compiled artifact's own properties claim 1.0.0.0 for what the docs call 1.7 — the exact "hardcoding the same string in five places guarantees they will disagree" failure §6 describes, except here there is no single source of truth to disagree *with*.

### F6.3 — No `CHANGELOG.md` *(should fix)*

Version history exists and is genuinely good — v1.0 through v1.7, including honest write-ups of the abandoned embedded-browser work — but it lives inside `README.md`. §6 requires `CHANGELOG.md`, and `FlexAppOneDownloadMonitor` already follows that split.

### Decision needed: which number ships *(escalation)*

Not something I should pick. The options:

- **Continue the existing line — `1.7.0`.** Honest about the seven iterations already delivered; matches the README the user has been reading.
- **Reset for Sparks — `0.1.0` or `0.2.0`.** Matches the `FlexAppOneDownloadMonitor` precedent, which was deliberately bumped from `1.0` to `0.2` for its submission, and signals field-tool maturity rather than product maturity.

Whichever you choose has to match in **four** places (§6's "one source of truth"): the value shown in the UI, `metadata.component.version` in the SBOM, `-version` in `Build-Exe.ps1`, and the distributable's filename. The fix should define it **once** in the script and have the build read it, rather than repeating the literal.

---

## 7. License PDF and SBOM packaged and visible to the end user

**Status: Fail — blocking**

### F7.1 — Neither file is present *(blocking)*

No `Spark_License.pdf`, no `bom.cdx.json` in the repository or in any distributable.

**The supplied PDF is verified current:** md5 `699f3a80f50d70f17af6684f8347ce1e` — **byte-identical** to the copy already shipping in `FlexAppOneDownloadMonitor`, i.e. the current v1.0 (8-4-26). It opens correctly (42,335 bytes).

Ship it as **`Spark_License.pdf`**. The uploaded name `Spark_License8426.pdf` embeds a date fragment; §7 asks for a name free of parentheses and spaces, and matching the sibling project's plain name keeps both submissions consistent.

### F7.2 — The README carries none of the required legal content *(blocking)*

Measured against §7 point by point, the current README is missing all of:

- The license's own headline warning — ***"IMPORTANT: READ BEFORE DOWNLOADING OR USING."*** — near the top.
- The core disclaimers (License §§1, 5, 6): community/field-contributed utility, **not a Liquidware commercial product**, **"AS IS" with no warranty, support or maintenance**, outside Liquidware's standard product development lifecycle, used at the customer's own risk.
- A `Files` table pointing at `Spark_License.pdf` and `bom.cdx.json`.
- An explicit statement of what the SBOM *is* — a CycloneDX 1.6 inventory of third-party components, provided so the customer's security team can review it against their own policy.

`FlexAppOneDownloadMonitor/README.md` already has all of this in the right shape and is the obvious template.

Nothing in the current README **implies** the tool is a supported Liquidware product, and no copyright notice or proprietary legend has been removed or obscured (License §2(e), §7) — the failure is omission, not misrepresentation.

### F7.3 — Nothing in the tool itself points at either document *(blocking)*

§7 requires both to be surfaced in the About screen / help output / startup banner alongside the version. **The tool has no About, Help or Diagnostics affordance at all** — there is no surface to put this on, so one has to be added. This is coupled to F6.1: the same dialog solves both.

`FlexAppOneDownloadMonitor` solved it with a Diagnostics dialog carrying version, license and SBOM references.

### F7.4 — No `THIRD-PARTY-NOTICES.txt` *(conditional)*

Required only if F4.2 resolves to "the exe ships" (then it holds ps2exe's MIT text). Not needed if only the `.ps1` is distributed.

### F7.5 — No distributable artifact is defined *(should fix)*

There is no zip, installer or package, so §7's "run the checks against the artifact you will actually distribute" can't be satisfied and the required file-listing evidence can't be produced. `FlexAppOneDownloadMonitor` settled on a zip; the same choice here would package: the `.ps1`, `Build-Exe.ps1`, `app-icon.ico`, `Spark_License.pdf`, `bom.cdx.json`, `README.md`, `CHANGELOG.md`.

Note the interaction with §6: if the artifact filename carries the version (`ProfileUnitySplashScreenManager-<version>.zip`), that's the fourth place the number has to agree.

---

## 8. UI consistency (style guide / PrimeNG)

**Status: Mostly Pass** — two off-scale type sizes to fix; everything else verified correct

Carried over from the `FlexAppOneDownloadMonitor` review. That audit had to reason about the style guide indirectly; this time the guide itself was supplied, so I checked the actual token values in `_ds/liquidware-ui-*/colors_and_type.css` (249 lines) rather than the handoff's description of them.

### PrimeNG / Angular: N/A

This is a WPF desktop application. No Angular, no PrimeNG, no web framework, no `package.json` — confirmed by grep. The commercial license key is therefore irrelevant to this project and stays out of it; see §3 for the full handling note.

### Verified correct — exact token matches

Every colour in the XAML resolves to a real token, at the right value:

| Script resource | Value | Style guide token |
|---|---|---|
| `Primary500` / `600` / `700` / `800` / `50` | `#0072BC` / `#0061A0` / `#005084` / `#003F67` / `#F2F8FC` | `--p-primary-500/600/700/800/50` ✓ all five exact |
| `Surface0/50/100/200/300/500/800` | `#FFFFFF` `#FAFAFA` `#F4F4F5` `#E4E4E7` `#D4D4D8` `#71717A` `#27272A` | `--p-surface-0/50/100/200/300/500/800` ✓ all seven exact |
| `GoodColor` / `PoorColor` | `#16A34A` / `#DC2626` | `--good-color` / `--poor-color` ✓ |
| `$FairBrush` (`:445`) | `#CA8A04` | `--fair-color` ✓ — correctly used for the "pending, not applied" state |
| `$MutedBrush` (`:446`) | `#71717A` | `--p-text-muted-color` → `--p-surface-500` ✓ |

Structure and behaviour also check out:

- Header bar **48px** = `--header-height: 48px` ✓, filled with `Primary600` = `--header-bg` ✓, header text **16px** = `--header-font-size: 16px` ✓.
- Window base **14px** = `--text-base` ✓ (the guide's root is 14px, not 16px — the script got this right).
- Card and field borders `Surface300` = `--frame-border-color` ✓.
- Button labels are weight **Normal**, and the guide is explicit: *"button labels are 400, not bold"* ✓.
- Hover/press ladder `600 → 700 → 800` matches `--p-primary-color` / `-hover-` / `-active-` ✓.
- `DataGridRow` selected → `Primary50`, hover → `Surface100`: consistent with the guide's light-scheme surface washes ✓.
- **Segoe UI is a legitimate spec choice, not a deviation.** `--font-sans` is `'Inter var', Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', …` — Segoe UI is the guide's own designated fallback for a machine without Inter, which is every bare Windows box this tool runs on. Same conclusion the FlexApp review reached, now confirmed against the actual token.
- **Both brand marks are faithful to the source SVGs** — decoded and pixel-sampled, see the §4 provenance table. The header correctly uses the *light* (white + `#03a9f4` flame) wordmark variant for a dark blue bar.

### F8.1 — 13px is not a token *(should fix)*

`FontSize="13"` appears in eight places — `TxtCurrentInfo` (`:365`), the "Search Images:" label and its `TextBox` (`:368`, `:370`), `GridHistory` (`:392`), `TxtStatus` (`:409`), among others. The guide's scale has no 13px step:

| Token | Value | Documented use |
|---|---|---|
| `--text-xs` | 10.5px | badges, version tag, micro-labels |
| `--text-sm` | 12px | table cells, dense UI |
| `--text-base` | 14px | body / default |
| `--text-md` | 16px | header, card-title emphasis |

**Fix:** `GridHistory` → **12px** (`.lwl-cell` is explicitly `--text-sm` for dense data). `TxtCurrentInfo`, `TxtStatus` and the search label/field → **14px** (`.lwl-body`), keeping the muted colour for secondary text (`.lwl-muted` is a colour class, not a size class — muted text stays at base size).

### F8.2 — Card titles are one step too small *(should fix)*

`.lwl-card-title` is `--text-md` (16px) at `--weight-medium` (500). "Current Splash Logo" (`:364`) and "Logo History" (`:385`) are `FontSize="14" FontWeight="Medium"` — right weight, wrong size.

**Fix:** 16px on both. The weight is already correct.

### F8.3 — Radius comment is inaccurate; the render is fine *(note, no fix proposed)*

The guide's radii are rem-based against the 14px root: `--radius-sm: 0.25rem` = **3.5px** (buttons/fields), `--radius-lg: 0.5rem` = **7px** (cards). The script uses 4px and 8px, and its comment (`:220`) calls them "4px (sm)" and "8px (lg)".

A half-pixel of corner radius is invisible, and WPF would happily take `3.5`/`7`. The thing that's actually wrong is the **comment**, which states token values that don't match the guide. Worth correcting the comment if we're in the file anyway; not worth changing the geometry.

### F8.4 — Card shadow is softer than the token *(note)*

`--card-shadow` is two layers: `0 0 1px black/60%` (a hairline) plus `0 1px 2px black/20%` (a tight drop). WPF's single `DropShadowEffect` can't express two layers, and the script uses `Opacity 0.10, BlurRadius 6, ShadowDepth 1` — softer and roughly 3× blurrier than the token.

The hairline layer is already effectively served by the card's 1px `Surface300` border, so the closest achievable single-effect match would be about `Opacity 0.2, BlurRadius 2, ShadowDepth 1`. Purely cosmetic; listing it so the decision is recorded either way.

### F8.5 — Hex texture is a crop, not a scale *(note)*

`$Base64_HexBg` is 420×300 (aspect 1.40) taken from `assets/lwl-hex.png` at 2880×2766 (aspect 1.04) — so it's a crop of the honeycomb, not a proportional downscale. This looks deliberate (a corner bleed wants a crop) and reads correctly at 220×150 with `Stretch="Uniform"`. Recorded so nobody later "fixes" it back to the full aspect ratio and gets a different-looking texture.

---

## Correctness findings outside the checklist

Found while reading the script end to end. Not checklist items — listed separately so they don't get conflated with the eight, and so they can be approved or declined independently.

### C1 — Setting the live logo as its own source destroys it *(should fix, data-loss)*

`Set-NewLogo` calls `Archive-CurrentLogo` (`:146`) **before** `Copy-Item -Path $SourcePath` (`:149`). `Archive-CurrentLogo` copies the live logo to history and then **deletes it** (`:128`). If the previewed source *is* the live logo, the copy's source no longer exists → `Copy-Item` throws → the machine is left with **no live splash logo**.

Repro: Browse to `C:\Program Files\ProfileUnity\Client.NET\client-custom-logo-300x86.png`, click **Set as Splash Logo**.

Recoverable (the file is in history, Restore brings it back), but the user sees only a generic error. **Fix:** compare full paths at the top of `Set-NewLogo` and short-circuit with *"That file is already the live splash logo."*

Same shape as the F1.3 failure — both leave the tool mid-transaction, which is the underlying weakness: archive-then-copy is not atomic and has no rollback.

### C2 — Two live logo files can coexist and the wrong one may win *(should fix)*

`Get-ExistingLiveLogo` takes `Select-Object -First 1` (`:103`). Because archive-then-copy isn't atomic, an interrupted run — or a pre-existing `.png` plus a hand-placed `.jpg` — leaves two `client-custom-logo-300x86.*` files. The tool then manages whichever the filesystem enumerates first, while **ProfileUnity may read the other one**, so the preview in the UI can disagree with the logo users actually see.

**Fix:** detect >1 match and either warn or archive all but the newest. Worth doing because the failure is silent and the symptom ("the tool shows the right logo but the splash shows the old one") is hard to diagnose in the field.

### C3 — Restore duplicates history entries *(note, decision)*

`Restore-FromHistoryEntry` archives the current logo but doesn't remove the entry being restored (`:161`–`:179`), so flipping A → B → A grows history by one entry each way. Probably intended (history as an append-only log). Flagging so it's a recorded decision rather than an oversight.

### C4 — Extension is validated, content isn't *(note)*

`Set-PendingPreview` checks only the extension (`:511`). A file named `.png` that isn't a PNG passes, `Load-ImagePreview` swallows the decode error and sets `Source = $null` (`:474`), and the Set button stays enabled — so a non-image can be copied into `Client.NET`, where ProfileUnity will render no logo at all.

**Fix:** treat a null preview as a validation failure and refuse to enable **Set as Splash Logo**. Cheap, and it stops a bad file before it reaches `Program Files`.

### C5 — Self-elevation relaunch is `.ps1`-only *(note)*

`:29`–`:30` relaunches via `powershell.exe -File "$PSCommandPath"`. Under a ps2exe build `$PSCommandPath` isn't a script path, so this fallback wouldn't work — but `-requireAdmin` means Windows elevates before the script runs, so the branch is never reached. Correct as-is; noting it because the README describes the internal logic as a "fallback," which it isn't for the exe.

---

## Phase 2 — Summary of changes needed

### Blocking (cannot submit without these)

| # | Item | Files | Change |
|---|---|---|---|
| B1 | §7 | **add** `Spark_License.pdf` | Ship the verified-current v1.0 PDF at the project root under the plain name |
| B2 | §4 | **add** `bom.cdx.json` | CycloneDX 1.6 JSON, valid against the schema, empty `components` (pending B7), `metadata` version matching B5 |
| B3 | §5 | *(evidence)* | Run Grype against the final SBOM on a networked machine; record version + DB version + date, save output to a file |
| B4 | §7 | `README.md` | Add the "IMPORTANT: READ BEFORE DOWNLOADING OR USING" header, the not-a-commercial-product / AS-IS / no-support disclaimers, a Files table pointing at the PDF and SBOM, and what the SBOM is |
| B5 | §6 | `Set-ProfileUnitySplashScreenLogo.ps1`, `Build-Exe.ps1` | Single `$AppVersion` constant; `Build-Exe.ps1` reads it instead of hardcoding `1.0.0.0`. **Needs your decision on the number** |
| B6 | §6 + §7 | `Set-ProfileUnitySplashScreenLogo.ps1` | Add an About dialog surfacing version + license + SBOM (the tool currently has no such surface) |
| B7 | §4 | *(decision)* | Whether the `.exe` ships → whether ps2exe (MIT) enters the SBOM and needs `THIRD-PARTY-NOTICES.txt` |

### Should fix

| # | Item | Files | Change | Blast radius |
|---|---|---|---|---|
| S1 | §1 F1.3 | `.ps1` (15 call sites) | `-Path` → `-LiteralPath` throughout | **Behavioural** — touches the write path, needs re-testing |
| S2 | §2 F2.1 | `.ps1` `:124`,`:154`,`:174`,`:506` | Invariant-culture write + `ParseExact` read, with tolerant fallback | **Behavioural** — fixes a locale-dependent break |
| S3 | §2 F2.2 | `.ps1` + manifest format | Store UTC with offset; format for display | **Behavioural** — on-disk format change, needs a migration path |
| S4 | §1 F1.1 | both `.ps1` files | Re-save as UTF-8 with BOM | Encoding-only, but re-verify PS 5.1 parses |
| S5 | §1 F1.2 | `.ps1` `:81`,`:96` | Add `-Encoding UTF8` to both reads | Mechanical |
| S6 | §2 F2.4 | `.ps1` `:188`–`:195`,`:497`,`:519` | Return dimensions as ints; compare values; distinct decode-failure message | Mechanical |
| S7 | §3 F3.1 | `README.md` | Disclose the Google endpoint: host, purpose, data transmitted, TLS, air-gap behaviour | Docs only |
| S8 | §6 F6.3 | **add** `CHANGELOG.md` | Move the README changelog out, matching the sibling project | Docs only |
| S9 | §4 F4.3 | `Build-Exe.ps1:30` | Pin ps2exe with `-RequiredVersion` | Build-time |
| S10 | §8 F8.1/F8.2 | `.ps1` XAML | 13px → 12px (grid) / 14px (body); card titles 14px → 16px | Visual only |
| S11 | §7 F7.5 | *(new)* | Define and build the distributable zip | Packaging |
| S12 | C1 | `.ps1` `:132` | Refuse to set the live logo as its own source | **Behavioural**, prevents data loss |
| S13 | C2 | `.ps1` `:101`–`:104` | Detect multiple live logo files; warn or archive extras | **Behavioural** |
| S14 | C4 | `.ps1` `:510`–`:526` | Treat an undecodable image as a validation failure | **Behavioural** |
| S15 | §1 F1.5 | `.ps1` `:205` | GUID-suffix clipboard temp files | Mechanical |
| S16 | §8 F8.3 | `.ps1` `:220` | Correct the radius comment (3.5px/7px) | Comment only |

### Optional / declined unless you say otherwise

- F3.2 — a `-SearchUrlTemplate` parameter for air-gapped or search-restricted customers. Feature change, not a defect.
- F8.4 — retune the card shadow to `Opacity 0.2 / Blur 2 / Depth 1`. Cosmetic; current value is defensible.
- C3 — restore-duplicates-history. Probably intended behaviour; leaving as-is unless you want it changed.
- C5 — README wording about the elevation "fallback" under ps2exe.

### Decisions needed from you (not fixes)

1. **Version number** (B5) — continue as `1.7.0`, or reset to `0.1.0`/`0.2.0` for parity with `FlexAppOneDownloadMonitor`?
2. **Does the `.exe` ship, or only the `.ps1`?** (B7) — determines whether ps2exe enters the SBOM and whether `THIRD-PARTY-NOTICES.txt` is needed.
3. **Manifest format migration** (S3) — accept a format change for UTC-with-offset, or keep local-time strings and just make them culture-invariant (S2 only)?
4. **PrimeUI key** — recommend rotating on exposure-in-transit grounds. Nothing to remediate in this repo.

### Files that would change

- **Modified:** `Set-ProfileUnitySplashScreenLogo.ps1`, `Build-Exe.ps1`, `README.md`
- **Added:** `Spark_License.pdf`, `bom.cdx.json`, `CHANGELOG.md`, possibly `THIRD-PARTY-NOTICES.txt`, possibly a distributable zip
- **Unchanged:** `app-icon.ico`, `CLAUDE.md`

### Blast radius summary

- **Mechanical, low risk:** S4, S5, S6, S7, S8, S9, S15, S16, B1, B4
- **Behavioural, needs re-testing on Windows:** S1, S2, S3, S12, S13, S14, B5, B6, S10
- **Cannot be verified in this environment at all:** anything requiring the app to run — the §1 double-byte round trip, the §2 non-US-locale run, the §5 Grype scan, and every §6/§7 screenshot. This is a WPF + ps2exe project and the sandbox is Linux. `FlexAppOneDownloadMonitor` closed the equivalent gaps on your Windows VM.

### What cannot be closed here regardless of approval

§5 (Grype), and the run-time evidence for §§1, 2, 6 and 7. Those need a Windows machine with network access. Everything else can be completed here.

---

## Submission Summary *(draft — not submittable yet)*

| # | Item | Status | Notes / evidence location |
|---|------|--------|---------------------------|
| 1 | Double-byte / Unicode handling | **Fail** | F1.1–F1.3 open; no round-trip evidence yet |
| 2 | Regional date, time, number formats | **Fail** | F2.1 breaks the history grid on some locales; no non-US-locale run yet |
| 3 | External URL / CDN references | **Fail (disclosure)** | Code clean; endpoint list missing from README |
| 4 | Open source identified + CycloneDX 1.6 JSON SBOM | **Fail** | No SBOM; ps2exe scope undecided |
| 5 | Zero Critical / High CVEs | **Not run** | Grype unavailable here; needs the final SBOM first |
| 6 | Version number visible to end user | **Fail** | No version in the tool; `1.0.0.0` vs README v1.7 |
| 7 | License PDF + SBOM packaged and visible | **Fail** | Neither file present; no About surface |
| 8 | UI consistency (style guide / PrimeNG) | **Mostly Pass** | All colours exact, marks verified; F8.1/F8.2 open |

**Project:** ProfileUnitySplashScreenManager
**Version submitted:** *undecided — see B5*
**Repository / artifact location:** `LaurensLiquidware/LW`, branch `claude/profileunity-splashscreen-manager-35z0ef`
**Third-party components:** none in the shipped `.ps1`; ps2exe (MIT) if the `.exe` ships — see B7
**Critical / High CVEs outstanding:** unknown — scan not yet run
**Grype scan date / DB version:** *not yet run*
**External endpoints retained:** `www.google.com` (image search, opened in the user's default browser, search term transmitted, TLS, no runtime fetch by the tool). Build-time only: `www.powershellgallery.com` (ps2exe)
**Open escalations / requested exceptions:** version number (B5); ps2exe/SBOM scope (B7); manifest migration (S3); PrimeUI key rotation
**Changes approved by (name / date):** *pending — nothing has been changed*
**Approved changes deferred, not made:** *n/a*
**Packaged path of license PDF + SBOM:** *not yet packaged — proposed at the project root, side by side*

### Attestation

*Not signed. This is a Phase 1 audit report, not a submission. The attestation is the SE's to sign once the approved changes are applied and the outstanding evidence has been captured on a Windows machine.*
