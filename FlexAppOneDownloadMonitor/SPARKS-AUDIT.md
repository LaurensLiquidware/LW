# Sparks Tool Project Review — Audit Report

**Project:** FlexAppOneDownloadMonitor
**Reviewed against:** Sparks Tool Project Review Checklist v1
**Audit date:** 2026-08-11
**Files reviewed:** `FlexAppOneDownloadMonitor.ps1`, `Start-FlexAppOneDownloadMonitor.vbs`, `README.md`, `archive/FlexAppOneDownloadMonitor_v1.ps1` (all renamed in Phase 3, round 6 — see below; these are the current names)
**Phase:** 3 — Approved items applied, across eight rounds. Round 1: §6 and §7. Round 2: §1 (code fix). Round 3: §8 color-matching sub-item. Round 4: version bumped from `1.0` to `0.2`. Round 5: distributable format decided (zip) and built. Round 6: project renamed from `FlexAppDownloadMonitor` to `FlexAppOneDownloadMonitor` throughout (files, install folder, script contents, and this report). Round 7: §5 (CVE scan) completed — 0 vulnerabilities found. Round 8: §1's real-Windows evidence captured on your Windows VM — item fully closed. **Every checklist item is now Pass/Fixed/N/A with evidence — nothing remains open.**

---

## Summary table

| # | Item | Status | Notes |
|---|------|--------|-------|
| 1 | Double-byte / Unicode handling | **Fixed** | BOM added to `.ps1`; explicit `-Encoding UTF8` on config read; tooltip truncation now text-element-safe. Round-trip test completed on a real Windows VM — see below. |
| 2 | Regional date, time, number formats | **Pass** | No hardcoded US formats found |
| 3 | External URL / CDN references | **Pass** | Zero external references found — fully local/offline |
| 4 | Open source identified + CycloneDX 1.6 JSON SBOM | **Pass** | Zero third-party components; SBOM generated (empty) |
| 5 | Zero Critical / High CVEs (Grype scan of SBOM) | **Pass** | Scanned on a machine with real internet access (Grype v0.117.0, DB schema v6.1.9, built 2026-08-11T06:26:49Z) — 0 vulnerability matches |
| 6 | Version number visible to end user | **Fixed** | `AppVersion` constant added and surfaced in the flyout title, tray tooltip, log, and Diagnostics dialog; `CHANGELOG.md` added |
| 7 | License PDF + SBOM packaged and visible | **Fixed** | `Spark_License.pdf` and `bom.cdx.json` now ship at the project root; README and Diagnostics dialog point to both |
| 8 | UI consistency (style guide / PrimeNG) | **N/A (PrimeNG) / Fixed (colors)** | No Angular/PrimeNG in this project. Colors now re-pointed to the style guide's dark-scheme tokens, per your approval. See below. |

---

## 1. Character encoding — double-byte and non-Latin input

**Status: Fixed** (approved and applied)

**Phase 3 — what was actually changed:**
- `FlexAppOneDownloadMonitor.ps1` is now saved with a UTF-8 BOM (was BOM-less ASCII).
- `Load-Config`'s `Get-Content` call now specifies `-Encoding UTF8` explicitly (`FlexAppOneDownloadMonitor.ps1:137`).
- Added a `Truncate-DisplaySafe` helper (`:304-318`) that walks .NET text elements via `System.Globalization.StringInfo` instead of a raw `Substring`, and switched the tray tooltip truncation to use it (`:891`) — a display name containing a surrogate pair or combining mark can no longer be cut in half.
- Did not change `Start-FlexAppOneDownloadMonitor.vbs`'s encoding — it has no non-ASCII content and no read/write encoding decisions of its own to make.

**Phase 3, round 8 — real-Windows evidence captured, item closed.** You ran the round-trip test on a Windows VM using a purpose-built set of test files (Japanese, Simplified Chinese, Korean, Cyrillic, accented Latin, a long name containing an emoji/surrogate pair, and a `.token`-path Japanese name) dropped into the watched cache folder. Results, from your screenshots:

- **History list**: every name — Simplified Chinese (`简体中文软件安装程序`), Japanese (`日本語データパッケージ`, and separately via the `.token` instant-completion path as `テストアプリ`), Korean (`한국어 소프트웨어 패키지` — hyphens correctly turned to spaces), Cyrillic (`Обновление программного обеспечения Данные`), and accented Latin (`Ångström café naïve Update`) — rendered completely intact, with correct elapsed durations, and the `.token`-path entry correctly showed no duration (by design — that path has no observed start time).
- **Active flyout entry**: the long emoji-bearing name displayed and progressed normally while downloading.
- **Tray tooltip** (the specific code path this fix touches): hovering the tray icon while `LongNameWithEmoji-🚀🎉-Package-Update-Installer-Name-Very-Long-Edition` was active showed `FlexApp Download Monitor - LongNameWithEmoji 🚀 Package Up...` — cut cleanly at the OS's tooltip width limit with `...` appended, no broken half-character, no mojibake, no exception. This is the exact scenario `Truncate-DisplaySafe` was written for, and it held up under a real surrogate pair in the real code path.

**No corruption, no crashes, no `?`/mojibake anywhere across any of the test strings.** §1 is now fully closed — code fix and evidence both complete.

**What I checked (Phase 1):** Source file encoding, all file read/write calls, string length/truncation logic, filename handling, regex, and console/log output.

**Findings:**

- `FlexAppOneDownloadMonitor.ps1` and `Start-FlexAppOneDownloadMonitor.vbs` are both plain ASCII with no byte-order mark (confirmed via `file`). That's fine *today* because every literal string in the file is ASCII, but it's a latent trap: Windows PowerShell 5.1 (the version this script targets — see the STA relaunch guard) reads a BOM-less `.ps1` using the **system's active code page**, not UTF-8. The moment anyone edits this file in an editor that saves BOM-less UTF-8 and adds a non-ASCII literal (e.g., a translated string, an em-dash, a non-English display name in a comment), the file will silently misinterpret those characters on a differently-configured machine. — `FlexAppOneDownloadMonitor.ps1:1`
  - **Fix:** Save the `.ps1` with a UTF-8 BOM (PowerShell 5.1's own default for `Out-File`/ISE "Save as UTF-8"), or add an explicit `#Requires` header note and a project convention to always save as UTF-8-with-BOM. Low blast radius — encoding-only, no logic change, but re-test that PowerShell 5.1 still parses it after re-saving.

- `Load-Config` reads the JSON config file with no explicit encoding: `Get-Content -LiteralPath $script:ConfigPath -Raw | ConvertFrom-Json` — `FlexAppOneDownloadMonitor.ps1:134`. If a user or automation tool ever writes the config file with UTF-8 (no BOM) from a non-Windows-PowerShell tool, and this app is later run under a non-Latin-1 code page, this default-encoding read could mis-decode the `CacheDir` path if it ever contains non-ASCII characters (e.g., a username with Cyrillic/CJK characters in the profile path, which does happen in enterprise environments).
  - **Fix:** `Get-Content -LiteralPath $script:ConfigPath -Raw -Encoding UTF8 | ConvertFrom-Json`. Mechanical, low blast radius.

- Tray tooltip truncation: `$tip.Substring(0, 60) + '...'` — `FlexAppOneDownloadMonitor.ps1:867`. `.Substring` on a .NET string counts UTF-16 code units, not characters. A display name containing a character outside the Basic Multilingual Plane (e.g., some emoji, some rare CJK extension characters) is represented as a surrogate pair; truncating mid-pair corrupts the string (renders as `�` or throws depending on context) and could also split a combining-mark sequence (e.g., some Cyrillic/diacritic combinations) so the base character displays without its accent.
  - **Fix:** Truncate on a text element boundary, e.g. iterate with `System.Globalization.StringInfo` or check `char.IsSurrogatePair`/`IsLowSurrogate` at the cut point before substringing. This is the one item here I'd flag as needing a short round of re-testing (double-byte test strings through the tray tooltip specifically) since it touches display logic, not just an encoding flag.

- `Add-Content -Encoding UTF8` (log, `FlexAppOneDownloadMonitor.ps1:121`) and `Set-Content -Encoding UTF8` (config, `FlexAppOneDownloadMonitor.ps1:148`) are correctly explicit. Worth noting for the record: PowerShell 5.1's `-Encoding UTF8` writes **UTF-8 with a BOM**, which is fine for a private log/config file but would matter if either file is ever consumed downstream by a tool that chokes on a BOM (e.g., some log shippers). Not a fix, just a thing to be aware of if the log format's contract ever changes.

- `Get-Item`/`Get-ChildItem` calls throughout consistently use `-LiteralPath`, which is the correct choice and avoids the common failure mode of wildcard characters in filenames (`[`, `]`, `*`) breaking path handling — this also happens to help with some non-Latin filenames that get misinterpreted as glob patterns in naive implementations. No fix needed here; noting it as a thing that's already done right.

- Format-DisplayName's regex operations (`-replace '\.(exe|msi|msix|appx|zip)$'`, `-replace '-', ' '`) anchor only on ASCII literals and don't truncate by byte count, so non-Latin app names pass through intact. No fix needed.

**Not tested at the time of the original audit (no Windows environment available in this session)** — completed in Phase 3, round 8, on your Windows VM. See above.

---

## 2. Regional formats — dates, times, numbers

**Status: Pass**

**What I checked:** Every `Get-Date`, `.ToString(...)`, and format-string call in the script; the config JSON; the log format.

**Findings:**

- No date is ever *parsed* from user or external input anywhere in this codebase — there is no date-parsing code path at all, so the classic `MM/DD/YYYY` ambiguity bug class doesn't apply here.
- All display timestamps use explicit, locale-invariant format strings: `'yyyy-MM-dd HH:mm:ss.fff'` (log, `:120`), `'HH:mm:ss.fff'` (in-memory diagnostics, `:350`, `:389`), `'HH:mm:ss'` (flyout history rows, `:685`/`:687`). These are all 24-hour, ISO-flavored, and don't depend on culture. Good.
- `Format-Bytes`/`Format-MB` use the `N`/`N2`/`N0` numeric format specifiers (`:303-311`), which *are* culture-sensitive (comma vs. period for decimal/grouping) — but this is exactly the checklist's "display to the user may use OS/user locale" case, since these values are only ever shown in the UI, never stored, parsed, or compared. This is correct behavior, not a bug.
- No currency, no AM/PM 12-hour clock anywhere, no day-of-week/month-name strings hardcoded to English.
- Config file (`FlexAppOneDownloadMonitor.config.json`) stores a single string path — no date/number fields to worry about.

**One minor note, not blocking:** the log timestamp format (`yyyy-MM-dd HH:mm:ss.fff`) has no timezone/offset. That's fine for a single-machine desktop tool reasoning about its own elapsed time, but if these logs are ever centrally aggregated across machines in different time zones, correlating them will be ambiguous. Low priority; flagging per the checklist's "log correlation across time zones" callout.

---

## 3. External references — URLs, CDNs, and remote code

**Status: Pass — list is "none"**

**What I checked:** Every file in the project (`.ps1`, `.vbs`, `.md`) for `http`, `https`, `www.`, CDN patterns, API endpoints, and any fetch/download/invoke-webrequest style calls.

**Findings:** Zero external references of any kind.

- No `Invoke-WebRequest`, `Invoke-RestMethod`, `.NET WebClient`, or any networking call anywhere in the script.
- No CDN-loaded fonts, scripts, styles, or icons — the tray icon is drawn programmatically with `System.Drawing` (GDI+), not loaded from anywhere.
- No telemetry, analytics, or "phone home" behavior.
- No hardcoded internal Liquidware hosts, staging URLs, or placeholder/scratch URLs (no pastebin, no ngrok, no localhost assumptions).
- No credentials, API keys, tokens, or connection strings in source, config, or comments.
- This tool only touches the local filesystem (`C:\ProgramData\Liquidware\ProfileUnity\Cache\FlexAppOne\`) and local Performance Counters — it will work unmodified in a fully air-gapped environment.

This item needs no remediation. **External endpoints retained: none.**

---

## 4. Open source components and SBOM

**Status: Pass**

**What I checked:** Whether the project declares or vendors any third-party dependency (package manifests, vendored source, copy-pasted snippets, bundled binaries), then ran Syft against the project directory to cross-check by static analysis rather than relying on my own read-through alone.

**Findings:**

- There is no package manifest of any kind in this project (no `package.json`, `.csproj`/`.sln`, `requirements.txt`, `go.mod`, etc.) — it's two plain script files.
- Everything the script calls (`System.Windows.Forms`, `System.Drawing`, `System.Diagnostics.PerformanceCounter`, `WScript.Shell` in the VBS launcher) is part of the .NET Framework / Windows OS itself, not a third-party redistributable component that needs its own license/SBOM entry.
- Ran `syft scan dir:./FlexAppOneDownloadMonitor -o cyclonedx-json` (Syft v1.21.0) as an independent check — it also found **zero** components, confirming the manual read.
- No vendored/copied third-party source was found in either the current or archived version of the script.

**SBOM:** Since the project genuinely has zero third-party components, I generated a valid, schema-checked CycloneDX 1.6 JSON SBOM with an empty `components` array, per the checklist's explicit instruction to do this rather than omit the SBOM. It's attached as `FlexAppOneDownloadMonitor/bom.cdx.json` (see "Attached files" below) — **not yet referenced from the README or packaged per §7, since that's a §7 finding pending your approval.**

- Validated against the official CycloneDX 1.6 JSON schema (fetched from the CycloneDX specification repo) using `jsonschema` — **valid**.
- `metadata.component` = `FlexAppOneDownloadMonitor`, version `1.0` (matches the version string currently in the script's comment header — see §6 for why that's itself a finding).
- `metadata.timestamp` = generation time of this audit.

**License summary:** No third-party SPDX IDs to report — "no open source" is the accurate finding, backed by the empty, schema-valid SBOM above.

---

## 5. Vulnerabilities — no Critical or High

**Status: Pass** (completed outside this session, per the plan below)

**Phase 3 — completed:** This session's network policy blocks `grype.anchore.io` (see the original finding below), so per the checklist's own instruction I stopped rather than substituting a different scanner, and asked you to run it in an environment with real internet access. You did, on your Mac:

- Installed Grype v0.117.0.
- `grype db update` → succeeded, DB updated to schema `v6.1.9`, built `2026-08-11T06:26:49Z`, pulled from `https://grype.anchore.io/databases/v6/...` (full record via `grype db status`, `Status: valid`).
- `grype sbom:bom.cdx.json` → **0 vulnerability matches** (0 critical, 0 high, 0 medium, 0 low, 0 negligible).

This matches the expectation from §4 (zero third-party components → nothing for a CVE to attach to), and now it's an actual scan result rather than an inference. **No Critical/High findings, no exceptions needed, no components to list as upgraded.** This closes the one remaining blocker from the original audit.

**Original finding (Phase 1), for reference:**

I installed Grype v0.90.0 (the required scanner) directly from its GitHub release, since that's the only scanner this checklist allows. However, **Grype's vulnerability database could not be downloaded in this session**: `grype db update` failed because the outbound network policy for this session explicitly blocks `grype.anchore.io` (confirmed via the proxy status endpoint — `connect_rejected`, "gateway answered 403 to CONNECT (policy denial)"). This was an organizational egress policy denial, not a transient failure, so per both this environment's proxy guidance and your instruction ("if Grype is unavailable, say so and stop rather than guessing or substituting a different scanner"), I stopped there rather than working around it with a different tool or a stale/absent database, and asked you to run it elsewhere instead.

---

## 6. Version number visible to the end user

**Status: Fixed** (approved and applied)

**Phase 3 — what was actually changed:** Added a single `$script:AppVersion` constant (`FlexAppOneDownloadMonitor.ps1:111`) and surfaced it in four places: the log startup line (`:166`), the flyout panel title bar (`:524`), the tray tooltip's idle text (`:869`), and the Diagnostics dialog (`:792`). Added `CHANGELOG.md` at the project root. Did not touch the active-download tray tooltip variants, since those are already close to the 63-character `NotifyIcon` limit and the idle state already gives the version a stable, always-reachable home.

**Phase 3, round 2 — version bumped to 0.2:** You chose `0.2` over the initial `1.0` draft. Updated in lockstep everywhere the version appears: `AppVersion` constant, the doc-comment header (`:20`), `README.md`'s version line, `CHANGELOG.md`'s heading, and `bom.cdx.json`'s `metadata.component.version` (bumped the SBOM's own `version` field from `1` to `2` and refreshed its `timestamp`, since the document changed). All five now consistently say `0.2`.

**Original finding (Phase 1), for reference:**

**What I checked:** Tray icon tooltip, flyout panel title/UI, right-click menu, log output, and the `.vbs` launcher for any version display; the script's own metadata.

**Findings:**

- A version does exist — `Version: 1.0` in the comment-block header at the top of the script (`FlexAppOneDownloadMonitor.ps1:20`) — but it is a **source comment only**. It is never surfaced anywhere the end user or their support desk would actually see it: not in the tray tooltip (`FlexAppOneDownloadMonitor.ps1:860-870`), not in the flyout panel title ("FlexApp One Downloads", `:521`), not in the right-click menu, not in the Diagnostics dialog (`:785-814`, which reports PID/counters/history count but no version), not in the log's startup banner (`:163`, which logs "started" and the watched path but no version), and there's no `-Version`/`-v` command-line switch at all.
- There is no `CHANGELOG.md` in the project.
- No file/assembly version metadata exists either (it's a script, not a compiled binary, so there's no natural place for that — all the more reason it needs to be in the visible places above).

**Fix (not applied — pending your approval):**
- Add a single `$script:AppVersion = '1.0'` constant near the top, and surface it in: the log startup line (`:163`), the Diagnostics dialog (`:788` area), and the flyout title bar or tray tooltip idle text (`:521` / `:862`).
- Add a `CHANGELOG.md` at the project root.
- Once a version is chosen for this Sparks submission, make sure it matches the SBOM's `metadata.component.version` (§4) — currently both say `1.0`, but that will need to move in lockstep going forward.
- This is a small, mechanical, low-blast-radius change (adding read-only display of an existing constant) — safe to bundle if approved.

---

## 7. License PDF and SBOM packaged and visible to the end user

**Status: Fixed** (approved and applied)

**Phase 3 — what was actually changed:** Saved the supplied PDF as `FlexAppOneDownloadMonitor/Spark_License.pdf` (renamed from `Spark_License8426.pdf`), sitting at the project root next to `bom.cdx.json`, matching the checklist's example layout. Added a section to the top of `README.md` with the license's "IMPORTANT: READ BEFORE DOWNLOADING OR USING" headline and the §1/§5/§6 core disclaimers (community/field tool, not a Liquidware commercial product, AS IS, no support), plus a `Files` table entry explicitly naming and describing the SBOM. Added `$script:LicensePath`/`$script:SbomPath` constants (`FlexAppOneDownloadMonitor.ps1:111-112`) and surfaced both file paths in the Diagnostics dialog (`:793-794`) alongside the version from §6. Did not add an installer/first-run flow, since this tool doesn't have one (copy-paste-install per the README) — the README and Diagnostics dialog are the two "in-app" surfaces that exist.

**Phase 3, round 5 — distributable format decided: zip.** You chose a zip as the distributable for now. Built `FlexAppOneDownloadMonitor-0.2.zip` containing exactly six files at a flat top level — `FlexAppOneDownloadMonitor.ps1`, `Start-FlexAppOneDownloadMonitor.vbs`, `Spark_License.pdf`, `bom.cdx.json`, `README.md`, `CHANGELOG.md` — so the license and SBOM sit immediately next to the tool inside the zip, satisfying the "packaged together" requirement concretely rather than abstractly. **Deliberately excluded** from the zip: `SPARKS-AUDIT.md` (this report — an internal review record, not something the customer needs) and `archive/FlexAppOneDownloadMonitor_v1.ps1` (superseded source history, not part of what runs). The zip is a build output, not checked into the repository — it's a scratch artifact for this delivery; if you want zip-building automated as part of the repo (a build script, a release workflow), that's a new, separate ask.

**Original finding (Phase 1), for reference:**

**What I checked:** The project directory tree, the README, whether either the Sparks license PDF or an SBOM ships anywhere in this project today, and — now that you've supplied it — the actual text of `Spark_License8426.pdf` (Liquidware Sparks Tool License and Disclaimer, v1.0).

**Findings:**

- Neither the Sparks Tool License PDF nor any SBOM currently exists anywhere in `FlexAppOneDownloadMonitor/` (prior to this audit — the `bom.cdx.json` I generated for §4 is new, from this audit pass, and is **not yet referenced or packaged** per this section's requirements).
- The README (`README.md`) makes no mention of a license, a disclaimer, or an SBOM anywhere. It also doesn't currently identify itself as a Sparks/community tool rather than a supported Liquidware product. Checked against the actual license text now: §1 requires disclosing that the Tool is "a community and field-contributed utility," "not a Liquidware commercial product," provided "outside Liquidware's standard product development lifecycle," with no security-review or support guarantees; §5 requires disclosing it's provided without support/maintenance/updates/patches; §6 requires the "AS IS"/no-warranty language. None of that appears in the README today.
- There is no `legal/` or `licenses/` subfolder, no `THIRD-PARTY-NOTICES.txt` — though per §4 there's currently nothing to put in one, since there are no third-party components.
- Distribution note: this project ships via the `.vbs` launcher + `.ps1` pair copied to `C:\FlexAppOneDownloadMonitor\` per the README's own install instructions — there's no zip/installer artifact yet, so "the distributable" and "the repo" are currently the same thing. Worth deciding what the actual customer-facing distributable will be (a zip? a folder copy?) since that changes what "packaged together" means concretely.
- One license term worth flagging even though it's not a checklist item: License §2(d) says the licensee ("you") may not "obtain possession of any source code or other technical material relating to the Tool." This is a script — the source *is* the artifact; there's no separate compiled form to distribute instead. That clause is presumably meant for Liquidware's compiled commercial products and reads oddly applied to a PowerShell tool distributed as source, but it's the license text as given — flagging as a question for you below rather than a finding I can "fix."

**Fix (not applied — still pending your explicit go-ahead on this specific item, per the checklist's "item by item" approval rule):**
- Save the supplied PDF into the project as `FlexAppOneDownloadMonitor/Spark_License.pdf` (filename with no spaces/parentheses per the checklist's own guidance — the supplied filename `Spark_License8426.pdf` should be renamed on the way in) alongside `bom.cdx.json` at the same top level.
- Add a short section near the top of `README.md`: the license's "IMPORTANT: READ BEFORE DOWNLOADING OR USING" headline, a one-line explanation of what `bom.cdx.json` is and why it's there, and the §1/§5/§6 disclaimers above.
- Surface both in-app: the Diagnostics dialog or an "About" menu item would be a natural place, alongside the version from §6.
- **This is a blocking item per the checklist.** The content blocker (not having the PDF) is now resolved — but I have not placed the file or edited the README yet, since that's still an edit pending your approval, not just a missing input.

---

## 8. UI consistency (style guide / PrimeNG)

**Status: N/A for PrimeNG/Angular; color-matching sub-item now Fixed (approved and applied)**

This item as scoped in your prompt is about Angular/web UI consistency against the attached Liquidware Style Guide (`colors_and_type.css`, PrimeNG-based component kit, Inter/Material-icon fonts) and, specifically, whether a PrimeNG commercial license key is safely kept out of shipped/committed artifacts.

- `FlexAppOneDownloadMonitor` is a native Windows Forms desktop application (System.Windows.Forms/System.Drawing), not an Angular application. It has no dependency on PrimeNG, Angular, or any web framework at all — confirmed by grep across the project for `PrimeNG`/`Angular` (no matches) and by the full file read in §4 (zero declared dependencies of any kind).
- Because there's no PrimeNG usage in this project, **the PrimeNG license key you attached for reference does not appear anywhere in this project, this report, the generated SBOM, or any committed file** — I did not write it into anything, and I'm not going to. Flagging this explicitly per your instruction to report a key exposure as top-severity rather than remediate silently: there is currently nothing to remediate here, but if a future version of this tool (or another Sparks submission) adds an Angular/PrimeNG front end, the same check needs to be re-run against that code, not assumed clear from this report.

**Phase 3 — color-matching, what was actually changed (you approved restyling to match):**

Re-pointed every hardcoded WinForms color in the flyout panel and tray icon to the nearest token in `colors_and_type.css`'s dark-scheme (`.dark`) palette, since the flyout is already a dark UI:

| Element | Was | Now | Style guide token |
|---|---|---|---|
| Flyout canvas | `RGB(32,34,38)` | `RGB(9,9,11)` | `--p-surface-950` |
| Top bar / header | *(unset — fell through to a light system gray, a pre-existing rendering gap)* | `RGB(0,63,103)` | `--p-primary-800` |
| Title text | White | `RGB(244,244,245)` | `--p-surface-100` |
| "Clear history" button text | LightGray | `RGB(161,161,170)` | `--p-surface-400` |
| "Clear history" button background | matched flyout canvas | matched top bar | *(now consistent with its actual container)* |
| Download-row card background | `RGB(44,46,51)` | `RGB(24,24,27)` | `--p-surface-900` |
| Row name text | White | `RGB(244,244,245)` | `--p-surface-100` |
| Row detail text (elapsed/size) | Gainsboro | `RGB(161,161,170)` | `--p-surface-400` |
| Row highlighted line (speed/ETA) | `RGB(120,190,255)` (arbitrary) | `RGB(74,163,224)` | `--link-color-dark` |
| Row size badge (top-right) | DimGray | `RGB(113,113,122)` | `--p-surface-500` |
| Section headers (ACTIVE/HISTORY) | Gray | `RGB(161,161,170)` | `--p-surface-400` |
| Empty-state text | Gainsboro | `RGB(161,161,170)` | `--p-surface-400` |
| Tray icon accent circle | `RGB(0,120,215)` (Windows system blue) | `RGB(11,114,186)` | `--lwl-mark-circle` (literally documented as "the circular app-icon background") |

Two things I deliberately did **not** change:
- **Typeface** — kept Segoe UI rather than switching to Inter. The style guide's own `--font-sans` stack lists `'Inter var', Inter, ..., 'Segoe UI', ...` — Segoe UI is explicitly the designated fallback for a system where Inter isn't installed, which describes a bare Windows machine running this tool. Bundling the actual Inter `.woff2` files would add a new third-party font dependency needing its own SBOM/license entries (§4/§7) for a cosmetic gain on a small tray flyout — disproportionate blast radius for what was asked.
- **Font weight nuance** (medium/semibold/bold distinctions from the type tokens) — GDI+/WinForms `Font.Style` only supports a binary Bold flag, not arbitrary numeric weights, so the existing Bold/Regular choices (titles bold, body regular) are the closest achievable match; no code change possible here without embedding a variable font.

While in there, fixed one adjacent rendering gap that the recolor task surfaced: the top bar `Panel` never had an explicit `BackColor` set, so it was rendering as a default light-gray system panel behind the (correctly dark-styled) title label and button — explicitly setting it to the header token above fixes this as a natural part of applying the palette, not a separate scope expansion.

No fix needed or proposed for this item as scoped.

---

## Blockers — status after Phase 3

1. **§7 — License PDF and SBOM not packaged. → Fixed.** `Spark_License.pdf` and `bom.cdx.json` now ship at the project root; README and the Diagnostics dialog point to both.
2. **§5 — CVE scan not completed. → Fixed.** Run on a machine with real internet access: Grype v0.117.0, DB schema v6.1.9 (built `2026-08-11T06:26:49Z`), **0 vulnerability matches**.
3. **§6 — No visible version number. → Fixed.** Version is now shown in the flyout title, tray tooltip, log, and Diagnostics dialog.

No copyleft/incompatible licenses, no hardcoded secrets, and no undisclosed external endpoints were found — those specific blocking categories are clear.

**No remaining blockers.** Every checklist item is Pass, Fixed, or N/A (with justification, per §8).

## Should-fix (remaining, non-blocking)

- §1 — ~~Encoding/truncation fixes~~ **Fully closed.** Code fix applied and the real-Windows double-byte round-trip test completed (round 8) — tray tooltip, flyout, History, log, and the `.token` path all confirmed intact under Japanese/Chinese/Korean/Cyrillic/accented-Latin/emoji test strings.
- §2 — Log timestamps have no timezone/offset; low priority given this is a single-machine tool, but worth a one-line format change if these logs are ever aggregated centrally.
- §7 — ~~Decide the distributable format~~ **Decided: zip.** `FlexAppOneDownloadMonitor-0.2.zip` built for this delivery (contents listed in §7 above). Not yet automated as part of a build/release process — that's a separate ask if wanted.

## Round 6 — project rename (FlexAppDownloadMonitor → FlexAppOneDownloadMonitor)

Per your instruction, renamed everything that still carried the old `FlexAppDownloadMonitor` name to `FlexAppOneDownloadMonitor` — the repo folder was already correctly named; this covered the files, the on-disk install path, and every internal reference:

- **Files renamed** (`git mv`, so history is preserved):
  - `FlexAppOneDownloadMonitor.ps1` (was `FlexAppDownloadMonitor.ps1`)
  - `Start-FlexAppOneDownloadMonitor.vbs` (was `Start-FlexAppDownloadMonitor.vbs`)
  - `archive/FlexAppOneDownloadMonitor_v1.ps1` (was `archive/FlexAppDownloadMonitor_v1.ps1`)
- **Script contents updated** in both the current script and the archived v1 snapshot: the `.SYNOPSIS` doc-comment name, the `.NOTES` run-it-with-PowerShell example path, and the config/log filenames the script generates on disk (`FlexAppOneDownloadMonitor.config.json`, `FlexAppOneDownloadMonitor.log` — these are built from `$PSScriptRoot` plus a literal filename, so the literal had to change to match).
- **`Start-FlexAppOneDownloadMonitor.vbs`**: its hardcoded `scriptPath` now points at `C:\FlexAppOneDownloadMonitor\FlexAppOneDownloadMonitor.ps1`.
- **`README.md`**: H1 title changed to "FlexApp One Download Monitor" (matching the app's own in-app title, "FlexApp One Downloads," which already said "One" — the README heading was the one place still missing it), the `Files` table, the install folder (`C:\FlexAppOneDownloadMonitor`), and every command/path example (`Unblock-File`, the quick-test `powershell.exe` command, the config path, the log path).
- **`SPARKS-AUDIT.md`** (this report): every file-path reference throughout Phases 1–3 updated to the current names, so the report doesn't point at files that no longer exist. Substance of every finding is unchanged — only names/paths were mechanically updated.
- **Not renamed:** `bom.cdx.json`'s `metadata.component.name` was already `FlexAppOneDownloadMonitor` (correct since it was generated after the repo folder's own name, well before the script/file rename) — no change needed there. `Spark_License.pdf` and `CHANGELOG.md` don't reference the old name and needed no changes.
- **Rebuilt:** `FlexAppOneDownloadMonitor-0.2.zip` regenerated with the current (post-rename) filenames — `FlexAppOneDownloadMonitor.ps1`, `Start-FlexAppOneDownloadMonitor.vbs`, `Spark_License.pdf`, `bom.cdx.json`, `README.md`, `CHANGELOG.md`. Superseded the earlier zip, which still had the old names.

## Files actually changed in Phase 3 (§6, §7, §1, §8 color-matching, version bump, then the rename — each approved separately)

- **Modified:** `FlexAppOneDownloadMonitor.ps1` —
  - §6/§7 round: added `AppVersion`/`LicensePath`/`SbomPath` constants; surfaced version in the log line, flyout title, tray idle tooltip, and Diagnostics dialog; surfaced license/SBOM paths in the Diagnostics dialog.
  - §1 round: re-saved with a UTF-8 BOM; added `-Encoding UTF8` to the config `Get-Content` call; added a `Truncate-DisplaySafe` helper and switched the tray tooltip truncation to use it instead of a raw `Substring`.
  - §8 round: re-pointed every hardcoded flyout/tray color to the nearest Liquidware style guide dark-scheme token (full mapping table in §8 above); fixed the top bar's missing `BackColor` as a natural side effect of that pass. Typeface (Segoe UI) and font-weight granularity intentionally left unchanged — see §8 for why.
  - Version-bump round: `AppVersion` changed from `1.0` to `0.2`; doc-comment header (`:20`) updated to match.
  - Rename round: file renamed from `FlexAppDownloadMonitor.ps1`; doc-comment name, run-it example path, and config/log filenames updated to match — see "Round 6" above.
  - `README.md`: added the license/disclaimer callout up top and a `Files` table entry for the license PDF, SBOM, and changelog; version line updated to `0.2`; rename round updated the title, file table, install path, and every command example.
- **Renamed (`git mv`):** `Start-FlexAppOneDownloadMonitor.vbs` (was `Start-FlexAppDownloadMonitor.vbs`, contents updated to match — see "Round 6"), `archive/FlexAppOneDownloadMonitor_v1.ps1` (was `archive/FlexAppDownloadMonitor_v1.ps1`, contents updated to match).
- **Added:** `Spark_License.pdf` (renamed from the supplied `Spark_License8426.pdf`), `CHANGELOG.md` (heading updated to `0.2`, with the encoding/color-matching work folded into the same entry since nothing shipped as `1.0`).
- **Carried over from Phase 1, now referenced:** `bom.cdx.json` (generated during the audit for §4) is now wired into the README and Diagnostics dialog per §7; its `metadata.component.version` updated to `0.2` and its own `version`/`timestamp` bumped to reflect the edit. Its `name` field never needed a rename.
- **§5 round:** no file changes — you ran `grype db update` and `grype sbom:bom.cdx.json` yourself on a Mac with normal internet access; result (0 vulnerability matches) folded into this report. Nothing in the project needed to change for this round.
- **Blast radius:** the §6/§7 changes are purely additive (new constants read-only, new UI text, new files, new README section). The §1 changes touch one real runtime code path — the tooltip truncation logic — the rest is encoding-only with no behavioral difference against the current ASCII-only content. The rename round is mechanical (names/paths only, no logic change) but touches every file in the project — re-test recommendation: confirm the app still finds/creates its config and log files under the new name, and that the `.vbs` launcher's updated hardcoded path is correct on the actual install target before relying on it. Re-testing recommendation from the §1 round still applies: confirm the tray tooltip still renders correctly at both the un-truncated and truncated lengths — the double-byte-specific verification still needs the real Windows environment noted in §1 above.

## Questions for me

1. ~~Do you have the current Sparks Tool License PDF?~~ **Answered** — `Spark_License8426.pdf` (v1.0) supplied and reviewed; content folded into §7 above.
2. ~~Can §5 be unblocked?~~ **Resolved** — ran outside this session, on a machine with normal internet access. No policy change needed after all; see §5.
3. ~~Should colors match the style guide?~~ **Answered** — colors now re-pointed to the dark-scheme tokens; typeface intentionally left as Segoe UI. See §8.
4. ~~What's the distributable format?~~ **Answered** — zip, for now. `FlexAppOneDownloadMonitor-0.2.zip` built. See §7.
5. ~~Is `1.0` the version you want to ship?~~ **Answered** — bumped to `0.2`, applied everywhere (constant, doc-comment, README, CHANGELOG, SBOM).
6. ~~License §2(d) source-code clause~~ **Actioned** — not resolved (it's a legal question, not something an edit can fix), but formally flagged below under "Escalations / exceptions requested" so it's visible to whoever signs off on submission, per your instruction.

---

## Escalations / exceptions requested

Per the checklist's own guidance ("anything that needs a decision rather than a fix... goes to your reviewer before submission, not in the submission" / the Submission Summary's "Open escalations / requested exceptions" field) — carrying this forward so it isn't lost between now and sign-off:

- **License §2(d) vs. source-distributed Sparks tools.** The Liquidware Sparks Tool License §2(d) prohibits the licensee from "obtain[ing] possession of any source code or other technical material relating to the Tool." `FlexAppOneDownloadMonitor` ships *as* its source (`FlexAppOneDownloadMonitor.ps1` + `Start-FlexAppOneDownloadMonitor.vbs` — there is no separate compiled artifact). Read literally, §2(d) is unsatisfiable for this tool and, presumably, for any PowerShell/script-based Sparks submission. This reads like boilerplate written for Liquidware's compiled commercial products and not adapted for source-distributed community tools — but that's a legal/license-authoring judgment call, not something I can resolve by editing this project. **Requesting**: confirmation from whoever owns the Sparks Tool License template (or legal) on whether §2(d) is intended to apply to script/source-distributed tools, and if not, whether the template should be clarified for future submissions. Not treated as a blocker for this specific review since I have no basis to override the license text myself, but it should be visible before this submission is signed off.

## Round 9 — re-verification (2026-08-12)

You supplied a fresh copy of the style guide baseline (`Spark_Liquidware_style_guide_baseline.zip`), a fresh copy of the license PDF (`Spark_License8426.pdf`), the PrimeNG commercial license token (`PrimeNG.txt`), and the checklist itself, and asked me to re-run this against them. This was audit-only — no files were edited, since nothing below required a fix.

- **§7 — License PDF.** Diffed the newly supplied `Spark_License8426.pdf` against the `Spark_License.pdf` already committed at the project root. Identical text, still Version 1.0, all ten sections unchanged. No update needed.
- **§8 — Style guide colors.** Extracted the newly supplied style guide zip and diffed its `colors_and_type.css` tokens against the twelve hardcoded WinForms colors in `FlexAppOneDownloadMonitor.ps1` (the round-3 mapping table above). All still match exactly — `--p-surface-950` (`#09090b`, `:529`), `--p-primary-800` (`#003f67`, `:538`), `--p-surface-100` (`#f4f4f5`, `:543`/`:590`), `--p-surface-400` (`#a1a1aa`, `:553`/`:611`/`:646`/`:720`), `--p-surface-900` (`#18181b`, `:587`), `--link-color-dark` (`#4aa3e0`, `:620`), `--p-surface-500` (`#71717a`, `:630`), `--lwl-mark-circle` (`#0b72ba`, `:776`). No drift, no re-pointing needed.
- **§8 — PrimeNG token.** Re-confirmed zero Angular/PrimeNG dependency in this project (grep for `primeng`/`angular` across the whole tree: no matches outside this report's own prose). The supplied token does not appear anywhere in this project, this report, the SBOM, or any committed file, and I have not written it anywhere. Noted for the record, not a finding against this project: the style guide baseline's own `fonts/primeicons-cdn.css` loads PrimeIcons from `unpkg.com` — a live CDN reference — but that lives in the design-system reference package itself, not in `FlexAppOneDownloadMonitor`, so it's outside this tool's §3 scope. Would matter if a future Angular/PrimeNG-based Sparks tool vendors that file directly; re-check then, don't assume clear from this note.
- **§1–6.** Re-read `README.md`, `CHANGELOG.md`, and `bom.cdx.json` — all still consistent with the Phase 3 closeout (SBOM valid, empty components, version `0.2` everywhere it should be).

**Result: no new findings, nothing changed.** The project remains fully closed — every checklist item still Pass/Fixed/N/A per the summary above.

## Attached files

- `FlexAppOneDownloadMonitor/bom.cdx.json` — CycloneDX 1.6 JSON SBOM, schema-validated, empty component list (§4/§7), now wired into the README and Diagnostics dialog.
- `FlexAppOneDownloadMonitor-0.2.zip` — the customer-facing distributable (§7 round 5): `FlexAppOneDownloadMonitor.ps1`, `Start-FlexAppOneDownloadMonitor.vbs`, `Spark_License.pdf`, `bom.cdx.json`, `README.md`, `CHANGELOG.md` at a flat top level. Delivered to you directly rather than committed to the repo, since it's a build output, not source.

This report now reflects Phase 1 (original audit) through Phase 3, round 8 (all approved fixes applied, §5's CVE scan completed, §1's real-Windows evidence captured). **Nothing remains open.** Every checklist section is Pass, Fixed, or N/A with a stated justification (§8) and, where applicable, real evidence rather than an inference.

## Submission Summary

| # | Item | Status |
|---|------|--------|
| 1 | Double-byte / Unicode handling | Fail — fixed, evidence captured on a real Windows VM |
| 2 | Regional date, time, number formats | Pass |
| 3 | External URL / CDN references | Pass — none |
| 4 | Open source identified + CycloneDX 1.6 JSON SBOM | Pass — zero third-party components |
| 5 | Zero Critical / High CVEs (Grype scan of SBOM) | Pass — 0 vulnerability matches (Grype v0.117.0, DB schema v6.1.9, built 2026-08-11T06:26:49Z) |
| 6 | Version number visible to end user | Fail — fixed |
| 7 | License PDF + SBOM packaged and visible | Fail — fixed |
| 8 | UI consistency (style guide / PrimeNG) | N/A (no PrimeNG/Angular) — colors fixed to match style guide |

**Project:** FlexAppOneDownloadMonitor
**Version submitted:** 0.2
**Repository:** `LaurensLiquidware/LW`, branch `claude/flexapp-download-monitor-setup-0blcm9`
**Third-party components:** none
**Critical / High CVEs outstanding:** 0
**Grype scan date / DB version:** 2026-08-11, DB schema v6.1.9 (built 2026-08-11T06:26:49Z)
**External endpoints retained:** none
**Open escalations / requested exceptions:** License §2(d) source-code clause — see "Escalations / exceptions requested" above
**Changes approved by:** you, item-by-item across Phase 3 rounds 1–8 (this report)
**Approved changes deferred, not made:** none — everything approved was applied
**Packaged path of license PDF + SBOM:** `FlexAppOneDownloadMonitor/Spark_License.pdf` and `FlexAppOneDownloadMonitor/bom.cdx.json`, top level, and inside `FlexAppOneDownloadMonitor-0.2.zip`
