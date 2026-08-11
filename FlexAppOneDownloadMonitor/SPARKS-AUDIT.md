# Sparks Tool Project Review — Audit Report

**Project:** FlexAppOneDownloadMonitor
**Reviewed against:** Sparks Tool Project Review Checklist v1
**Audit date:** 2026-08-11
**Files reviewed:** `FlexAppDownloadMonitor.ps1`, `Start-FlexAppDownloadMonitor.vbs`, `README.md`, `archive/FlexAppDownloadMonitor_v1.ps1`
**Phase:** 3 — Approved items applied, across two rounds. Round 1: §6 and §7. Round 2: §1. §5 remains blocked, not approved (blocked on network policy, not a code fix). Everything in Phases 1–2 below is unchanged from the original audit; a "Phase 3 — applied" note has been added to each fixed section describing exactly what was changed.

---

## Summary table

| # | Item | Status | Notes |
|---|------|--------|-------|
| 1 | Double-byte / Unicode handling | **Fixed** | BOM added to `.ps1`; explicit `-Encoding UTF8` on config read; tooltip truncation now text-element-safe. Round-trip test with real Windows still outstanding — see below. |
| 2 | Regional date, time, number formats | **Pass** | No hardcoded US formats found |
| 3 | External URL / CDN references | **Pass** | Zero external references found — fully local/offline |
| 4 | Open source identified + CycloneDX 1.6 JSON SBOM | **Pass** | Zero third-party components; SBOM generated (empty) |
| 5 | Zero Critical / High CVEs (Grype scan of SBOM) | **NEEDS-INFO — blocked (not approved)** | Could not reach Grype's vulnerability DB (see below); stopping per instructions rather than substituting a scanner |
| 6 | Version number visible to end user | **Fixed** | `AppVersion` constant added and surfaced in the flyout title, tray tooltip, log, and Diagnostics dialog; `CHANGELOG.md` added |
| 7 | License PDF + SBOM packaged and visible | **Fixed** | `Spark_License.pdf` and `bom.cdx.json` now ship at the project root; README and Diagnostics dialog point to both |
| 8 | UI consistency (style guide / PrimeNG) | **N/A** | This is a native WinForms desktop app, not an Angular/web UI — the Liquidware web style guide and PrimeNG do not apply. See below. |

---

## 1. Character encoding — double-byte and non-Latin input

**Status: Fixed** (approved and applied)

**Phase 3 — what was actually changed:**
- `FlexAppDownloadMonitor.ps1` is now saved with a UTF-8 BOM (was BOM-less ASCII).
- `Load-Config`'s `Get-Content` call now specifies `-Encoding UTF8` explicitly (`FlexAppDownloadMonitor.ps1:137`).
- Added a `Truncate-DisplaySafe` helper (`:304-318`) that walks .NET text elements via `System.Globalization.StringInfo` instead of a raw `Substring`, and switched the tray tooltip truncation to use it (`:891`) — a display name containing a surrogate pair or combining mark can no longer be cut in half.
- Did not change `Start-FlexAppDownloadMonitor.vbs`'s encoding — it has no non-ASCII content and no read/write encoding decisions of its own to make.
- **Not done, still open:** the actual round-trip test with Japanese/Cyrillic/CJK test strings through the running app, since no Windows environment is available in this session. That evidence still needs to be captured on a real Windows box before sign-off.

**What I checked (Phase 1):** Source file encoding, all file read/write calls, string length/truncation logic, filename handling, regex, and console/log output.

**Findings:**

- `FlexAppDownloadMonitor.ps1` and `Start-FlexAppDownloadMonitor.vbs` are both plain ASCII with no byte-order mark (confirmed via `file`). That's fine *today* because every literal string in the file is ASCII, but it's a latent trap: Windows PowerShell 5.1 (the version this script targets — see the STA relaunch guard) reads a BOM-less `.ps1` using the **system's active code page**, not UTF-8. The moment anyone edits this file in an editor that saves BOM-less UTF-8 and adds a non-ASCII literal (e.g., a translated string, an em-dash, a non-English display name in a comment), the file will silently misinterpret those characters on a differently-configured machine. — `FlexAppDownloadMonitor.ps1:1`
  - **Fix:** Save the `.ps1` with a UTF-8 BOM (PowerShell 5.1's own default for `Out-File`/ISE "Save as UTF-8"), or add an explicit `#Requires` header note and a project convention to always save as UTF-8-with-BOM. Low blast radius — encoding-only, no logic change, but re-test that PowerShell 5.1 still parses it after re-saving.

- `Load-Config` reads the JSON config file with no explicit encoding: `Get-Content -LiteralPath $script:ConfigPath -Raw | ConvertFrom-Json` — `FlexAppDownloadMonitor.ps1:134`. If a user or automation tool ever writes the config file with UTF-8 (no BOM) from a non-Windows-PowerShell tool, and this app is later run under a non-Latin-1 code page, this default-encoding read could mis-decode the `CacheDir` path if it ever contains non-ASCII characters (e.g., a username with Cyrillic/CJK characters in the profile path, which does happen in enterprise environments).
  - **Fix:** `Get-Content -LiteralPath $script:ConfigPath -Raw -Encoding UTF8 | ConvertFrom-Json`. Mechanical, low blast radius.

- Tray tooltip truncation: `$tip.Substring(0, 60) + '...'` — `FlexAppDownloadMonitor.ps1:867`. `.Substring` on a .NET string counts UTF-16 code units, not characters. A display name containing a character outside the Basic Multilingual Plane (e.g., some emoji, some rare CJK extension characters) is represented as a surrogate pair; truncating mid-pair corrupts the string (renders as `�` or throws depending on context) and could also split a combining-mark sequence (e.g., some Cyrillic/diacritic combinations) so the base character displays without its accent.
  - **Fix:** Truncate on a text element boundary, e.g. iterate with `System.Globalization.StringInfo` or check `char.IsSurrogatePair`/`IsLowSurrogate` at the cut point before substringing. This is the one item here I'd flag as needing a short round of re-testing (double-byte test strings through the tray tooltip specifically) since it touches display logic, not just an encoding flag.

- `Add-Content -Encoding UTF8` (log, `FlexAppDownloadMonitor.ps1:121`) and `Set-Content -Encoding UTF8` (config, `FlexAppDownloadMonitor.ps1:148`) are correctly explicit. Worth noting for the record: PowerShell 5.1's `-Encoding UTF8` writes **UTF-8 with a BOM**, which is fine for a private log/config file but would matter if either file is ever consumed downstream by a tool that chokes on a BOM (e.g., some log shippers). Not a fix, just a thing to be aware of if the log format's contract ever changes.

- `Get-Item`/`Get-ChildItem` calls throughout consistently use `-LiteralPath`, which is the correct choice and avoids the common failure mode of wildcard characters in filenames (`[`, `]`, `*`) breaking path handling — this also happens to help with some non-Latin filenames that get misinterpreted as glob patterns in naive implementations. No fix needed here; noting it as a thing that's already done right.

- Format-DisplayName's regex operations (`-replace '\.(exe|msi|msix|appx|zip)$'`, `-replace '-', ' '`) anchor only on ASCII literals and don't truncate by byte count, so non-Latin app names pass through intact. No fix needed.

**Not tested (no Windows environment available in this session):** an actual round-trip of Japanese/Cyrillic/CJK strings through the running app (tray tooltip, flyout labels, log file, config file) as the checklist's evidence section asks for. This needs to happen on a real Windows box before sign-off; I can't produce that screenshot from here.

---

## 2. Regional formats — dates, times, numbers

**Status: Pass**

**What I checked:** Every `Get-Date`, `.ToString(...)`, and format-string call in the script; the config JSON; the log format.

**Findings:**

- No date is ever *parsed* from user or external input anywhere in this codebase — there is no date-parsing code path at all, so the classic `MM/DD/YYYY` ambiguity bug class doesn't apply here.
- All display timestamps use explicit, locale-invariant format strings: `'yyyy-MM-dd HH:mm:ss.fff'` (log, `:120`), `'HH:mm:ss.fff'` (in-memory diagnostics, `:350`, `:389`), `'HH:mm:ss'` (flyout history rows, `:685`/`:687`). These are all 24-hour, ISO-flavored, and don't depend on culture. Good.
- `Format-Bytes`/`Format-MB` use the `N`/`N2`/`N0` numeric format specifiers (`:303-311`), which *are* culture-sensitive (comma vs. period for decimal/grouping) — but this is exactly the checklist's "display to the user may use OS/user locale" case, since these values are only ever shown in the UI, never stored, parsed, or compared. This is correct behavior, not a bug.
- No currency, no AM/PM 12-hour clock anywhere, no day-of-week/month-name strings hardcoded to English.
- Config file (`FlexAppDownloadMonitor.config.json`) stores a single string path — no date/number fields to worry about.

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

**Status: NEEDS-INFO — blocked, stopping rather than substituting**

I installed Grype v0.90.0 (the required scanner) directly from its GitHub release, since that's the only scanner this checklist allows. However, **Grype's vulnerability database cannot be downloaded in this environment**: `grype db update` fails because the outbound network policy for this session explicitly blocks `grype.anchore.io` (confirmed via the proxy status endpoint — `connect_rejected`, "gateway answered 403 to CONNECT (policy denial)"). This is an organizational egress policy denial, not a transient failure, so per both this environment's proxy guidance and your instruction ("if Grype is unavailable, say so and stop rather than guessing or substituting a different scanner"), I am stopping here rather than working around it with a different tool or a stale/absent database.

**What this means in practice:** given §4's finding that this project has zero third-party components, the *ceiling* on this item's severity is low — there's nothing in the SBOM for a CVE to attach to. But I want to be precise: that's a reasonable inference, not a substitute for actually running the required scan, so I'm reporting this as blocked rather than marking it Pass.

**What I need from you:** either (a) run `grype sbom:FlexAppOneDownloadMonitor/bom.cdx.json` yourself in an environment with access to `grype.anchore.io`, or point me at how this session's egress policy can be adjusted for that host, and I'll complete this item and fold the result back into this report.

---

## 6. Version number visible to the end user

**Status: Fixed** (approved and applied)

**Phase 3 — what was actually changed:** Added a single `$script:AppVersion = '1.0'` constant (`FlexAppDownloadMonitor.ps1:109`) and surfaced it in four places: the log startup line (`:166`), the flyout panel title bar (`:524`, now "FlexApp One Downloads  v1.0"), the tray tooltip's idle text (`:869`, now "FlexApp Download Monitor v1.0 - idle"), and the Diagnostics dialog (`:792`). Added `CHANGELOG.md` at the project root recording this as the 1.0 release. Did not touch the active-download tray tooltip variants, since those are already close to the 63-character `NotifyIcon` limit and the idle state already gives the version a stable, always-reachable home. The version matches `bom.cdx.json`'s `metadata.component.version` (both `1.0`).

**Original finding (Phase 1), for reference:**

**What I checked:** Tray icon tooltip, flyout panel title/UI, right-click menu, log output, and the `.vbs` launcher for any version display; the script's own metadata.

**Findings:**

- A version does exist — `Version: 1.0` in the comment-block header at the top of the script (`FlexAppDownloadMonitor.ps1:20`) — but it is a **source comment only**. It is never surfaced anywhere the end user or their support desk would actually see it: not in the tray tooltip (`FlexAppDownloadMonitor.ps1:860-870`), not in the flyout panel title ("FlexApp One Downloads", `:521`), not in the right-click menu, not in the Diagnostics dialog (`:785-814`, which reports PID/counters/history count but no version), not in the log's startup banner (`:163`, which logs "started" and the watched path but no version), and there's no `-Version`/`-v` command-line switch at all.
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

**Phase 3 — what was actually changed:** Saved the supplied PDF as `FlexAppOneDownloadMonitor/Spark_License.pdf` (renamed from `Spark_License8426.pdf`), sitting at the project root next to `bom.cdx.json`, matching the checklist's example layout. Added a section to the top of `README.md` with the license's "IMPORTANT: READ BEFORE DOWNLOADING OR USING" headline and the §1/§5/§6 core disclaimers (community/field tool, not a Liquidware commercial product, AS IS, no support), plus a `Files` table entry explicitly naming and describing the SBOM. Added `$script:LicensePath`/`$script:SbomPath` constants (`FlexAppDownloadMonitor.ps1:111-112`) and surfaced both file paths in the Diagnostics dialog (`:793-794`) alongside the version from §6. Did not add an installer/first-run flow, since this tool doesn't have one (copy-paste-install per the README) — the README and Diagnostics dialog are the two "in-app" surfaces that exist.

**Original finding (Phase 1), for reference:**

**What I checked:** The project directory tree, the README, whether either the Sparks license PDF or an SBOM ships anywhere in this project today, and — now that you've supplied it — the actual text of `Spark_License8426.pdf` (Liquidware Sparks Tool License and Disclaimer, v1.0).

**Findings:**

- Neither the Sparks Tool License PDF nor any SBOM currently exists anywhere in `FlexAppOneDownloadMonitor/` (prior to this audit — the `bom.cdx.json` I generated for §4 is new, from this audit pass, and is **not yet referenced or packaged** per this section's requirements).
- The README (`README.md`) makes no mention of a license, a disclaimer, or an SBOM anywhere. It also doesn't currently identify itself as a Sparks/community tool rather than a supported Liquidware product. Checked against the actual license text now: §1 requires disclosing that the Tool is "a community and field-contributed utility," "not a Liquidware commercial product," provided "outside Liquidware's standard product development lifecycle," with no security-review or support guarantees; §5 requires disclosing it's provided without support/maintenance/updates/patches; §6 requires the "AS IS"/no-warranty language. None of that appears in the README today.
- There is no `legal/` or `licenses/` subfolder, no `THIRD-PARTY-NOTICES.txt` — though per §4 there's currently nothing to put in one, since there are no third-party components.
- Distribution note: this project ships via the `.vbs` launcher + `.ps1` pair copied to `C:\FlexAppDownloadMonitor\` per the README's own install instructions — there's no zip/installer artifact yet, so "the distributable" and "the repo" are currently the same thing. Worth deciding what the actual customer-facing distributable will be (a zip? a folder copy?) since that changes what "packaged together" means concretely.
- One license term worth flagging even though it's not a checklist item: License §2(d) says the licensee ("you") may not "obtain possession of any source code or other technical material relating to the Tool." This is a script — the source *is* the artifact; there's no separate compiled form to distribute instead. That clause is presumably meant for Liquidware's compiled commercial products and reads oddly applied to a PowerShell tool distributed as source, but it's the license text as given — flagging as a question for you below rather than a finding I can "fix."

**Fix (not applied — still pending your explicit go-ahead on this specific item, per the checklist's "item by item" approval rule):**
- Save the supplied PDF into the project as `FlexAppOneDownloadMonitor/Spark_License.pdf` (filename with no spaces/parentheses per the checklist's own guidance — the supplied filename `Spark_License8426.pdf` should be renamed on the way in) alongside `bom.cdx.json` at the same top level.
- Add a short section near the top of `README.md`: the license's "IMPORTANT: READ BEFORE DOWNLOADING OR USING" headline, a one-line explanation of what `bom.cdx.json` is and why it's there, and the §1/§5/§6 disclaimers above.
- Surface both in-app: the Diagnostics dialog or an "About" menu item would be a natural place, alongside the version from §6.
- **This is a blocking item per the checklist.** The content blocker (not having the PDF) is now resolved — but I have not placed the file or edited the README yet, since that's still an edit pending your approval, not just a missing input.

---

## 8. UI consistency (style guide / PrimeNG)

**Status: N/A**

This item as scoped in your prompt is about Angular/web UI consistency against the attached Liquidware Style Guide (`colors_and_type.css`, PrimeNG-based component kit, Inter/Material-icon fonts) and, specifically, whether a PrimeNG commercial license key is safely kept out of shipped/committed artifacts.

- `FlexAppOneDownloadMonitor` is a native Windows Forms desktop application (System.Windows.Forms/System.Drawing), not an Angular application. It has no dependency on PrimeNG, Angular, or any web framework at all — confirmed by grep across the project for `PrimeNG`/`Angular` (no matches) and by the full file read in §4 (zero declared dependencies of any kind).
- Because there's no PrimeNG usage in this project, **the PrimeNG license key you attached for reference does not appear anywhere in this project, this report, the generated SBOM, or any committed file** — I did not write it into anything, and I'm not going to. Flagging this explicitly per your instruction to report a key exposure as top-severity rather than remediate silently: there is currently nothing to remediate here, but if a future version of this tool (or another Sparks submission) adds an Angular/PrimeNG front end, the same check needs to be re-run against that code, not assumed clear from this report.
- On the softer "does it look like a Liquidware tool" question: the tray icon uses a Windows-system blue (`RGB(0,120,215)`, Windows' own accent blue) rather than any color pulled from the style guide's palette (`colors_and_type.css`), and the flyout panel uses a dark neutral gray (`RGB(32,34,38)`) with the system default "Segoe UI" font rather than the style guide's Inter typeface. Since this is a native desktop tray utility rather than a branded product UI, I don't think matching the web design system word-for-word is a real requirement — but flagging it as a "Questions for me" item below since I can't tell if there's an expectation here.

No fix needed or proposed for this item as scoped.

---

## Blockers — status after Phase 3

1. **§7 — License PDF and SBOM not packaged. → Fixed.** `Spark_License.pdf` and `bom.cdx.json` now ship at the project root; README and the Diagnostics dialog point to both.
2. **§5 — CVE scan not completed.** Still blocked by network policy in this session, not by anything in the code — not approved for this round, unresolved. Needs to be run wherever `grype.anchore.io` is reachable before sign-off, even though §4 suggests the practical risk ceiling is low (zero third-party components).
3. **§6 — No visible version number. → Fixed.** Version is now shown in the flyout title, tray tooltip, log, and Diagnostics dialog.

No copyleft/incompatible licenses, no hardcoded secrets, and no undisclosed external endpoints were found — those specific blocking categories are clear.

**Remaining blocker: §5 (CVE scan).** Everything else is resolved.

## Should-fix (remaining, non-blocking)

- §1 — ~~Encoding/truncation fixes~~ **Fixed**, see below. Only the real-Windows double-byte round-trip *test* (the checklist's evidence requirement) is still outstanding — that needs an actual Windows box, not more code changes.
- §2 — Log timestamps have no timezone/offset; low priority given this is a single-machine tool, but worth a one-line format change if these logs are ever aggregated centrally.
- §7 — Decide what the actual customer-facing distributable artifact is (zip vs. folder copy) so "packaged together" has a concrete target.

## Files actually changed in Phase 3 (§6, §7, then §1 — each approved separately)

- **Modified:** `FlexAppDownloadMonitor.ps1` —
  - §6/§7 round: added `AppVersion`/`LicensePath`/`SbomPath` constants; surfaced version in the log line, flyout title, tray idle tooltip, and Diagnostics dialog; surfaced license/SBOM paths in the Diagnostics dialog.
  - §1 round: re-saved with a UTF-8 BOM; added `-Encoding UTF8` to the config `Get-Content` call; added a `Truncate-DisplaySafe` helper and switched the tray tooltip truncation to use it instead of a raw `Substring`.
  - `README.md`: added the license/disclaimer callout up top and a `Files` table entry for the license PDF, SBOM, and changelog.
- **Added:** `Spark_License.pdf` (renamed from the supplied `Spark_License8426.pdf`), `CHANGELOG.md`.
- **Carried over from Phase 1, now referenced:** `bom.cdx.json` (generated during the audit for §4) is now wired into the README and Diagnostics dialog per §7.
- **Not touched:** §5 (CVE scan) — left exactly as reported, no code changes made, blocked on network policy rather than code.
- **Blast radius:** the §6/§7 changes are purely additive (new constants read-only, new UI text, new files, new README section). The §1 changes touch one real runtime code path — the tooltip truncation logic — the rest is encoding-only with no behavioral difference against the current ASCII-only content. Re-testing recommendation: confirm the tray tooltip still renders correctly at both the un-truncated and truncated lengths (a quick manual check, no double-byte strings needed to catch a regression in the ASCII case) — the double-byte-specific verification still needs the real Windows environment noted in §1 above.

## Questions for me

1. ~~Do you have the current Sparks Tool License PDF?~~ **Answered** — `Spark_License8426.pdf` (v1.0) supplied and reviewed; content folded into §7 above.
2. Is there a way to allow this session's egress policy to reach `grype.anchore.io` (or an internal mirror of the Grype DB), so §5 can actually complete instead of staying blocked?
3. Should the tray icon/flyout colors and font be brought in line with the Liquidware style guide palette (`colors_and_type.css`) even though this is a native desktop app, not a web UI? I don't have a strong signal either way from the checklist.
4. What's the intended customer-facing distributable format (zip, installer, folder) — this determines exactly what "packaged together" in §7 needs to look like?
5. Is `1.0` the version you want to ship, or should this be bumped as part of the Sparks submission?
6. License §2(d) forbids the licensee from "obtain[ing] possession of any source code" — but this Tool ships *as* source (a `.ps1`/`.vbs` pair). Is that clause meant to apply here, or is it boilerplate carried over from Liquidware's compiled commercial products that doesn't quite fit a source-distributed Sparks tool? Not something I can resolve by editing code — flagging for you/legal.

---

## Attached files

- `FlexAppOneDownloadMonitor/bom.cdx.json` — CycloneDX 1.6 JSON SBOM, schema-validated, empty component list (§4). Not yet referenced from the README or packaged per §7 pending your approval.

No existing project file was modified during this audit.
