# ProfileUnity SplashScreen Manager — project constraints

Carried over from the claude.ai session this project was handed off from, so
the constraints survive across sessions. `Set-ProfileUnitySplashScreenLogo.ps1`
is the source of truth for behaviour; this file records the decisions behind it.

## Non-negotiable spec — ask before changing any of this

- **Target file:** `C:\Program Files\ProfileUnity\Client.NET\client-custom-logo-300x86.<ext>`.
  Both the filename and the folder are dictated by ProfileUnity itself (Liquidware
  KB 12914471137293), not by us. Allowed input extensions:
  `.bmp .jpg .jpeg .gif .png .tif .tiff`; normalized to `.jpg` / `.tif` on write,
  because ProfileUnity does not recognize `.jpeg` / `.tiff`.
- **`-TargetDir`** exists so the tool can be pointed at a staging/test copy.
- **History + manifest** live in
  `C:\ProgramData\Liquidware\ProfileUnitySplashScreenLogoManager\`, deliberately
  outside `Client.NET` so they survive a ProfileUnity client reinstall/upgrade.
- **Self-elevation** via UAC relaunch — writing to `Program Files` needs admin.
- **`LwL.ProfileUnity.Client.CtxInit.exe`** (in the target folder) is what the
  "Preview Splash Screen" button launches, to see the real splash screen without
  a logon cycle.
- **Single file.** The whole app stays one `.ps1` unless explicitly asked to split
  it. Brand assets are base64-embedded for the same reason — no loose asset files.

## Branding

From the Liquidware / Stratusphere UX style guide baseline:

- Brand blue `#0061A0` (primary-600) header bar, `#005084` / `#003F67` hover/press.
- Zinc neutral surfaces: `#FAFAFA` `#F4F4F5` `#E4E4E7` `#D4D4D8` `#71717A` `#27272A`.
- Good `#16A34A` / Poor `#DC2626` reused for success/error status text;
  Fair `#CA8A04` for the "pending, not yet applied" state.
- Fixed 48px header bar, 8px-radius flat cards with a whisper-soft shadow,
  4px radius on buttons/fields, 1px `surface-300` borders.
- Liquidware wordmark + flame icon rasterized from the guide's SVGs. Hex-honeycomb
  texture bleeds subtly from the top-left of the content area.
- Type: 14px base / 16px header. **Font is Segoe UI, not Inter** — WPF can't load
  the variable `.woff2` the guide ships, and Segoe UI is the guide's own documented
  fallback. Deliberate and documented, not an oversight.
- Copy: Title Case labels/buttons, terse impersonal system messages, no emoji.

If brand assets are ever regenerated, re-derive from the style guide's `assets/`
SVGs rather than re-compressing the already-lossy embedded PNGs.

## Tried and abandoned — don't re-attempt without a good reason

- **In-app embedded browser popout** (WinForms `WebBrowser`) for image search.
  The IE11/Trident engine behind it has no WebP decoder (broken-image icons on
  most modern sites) and incomplete CSS/JS support (garbled layouts). User-agent
  spoofing didn't help — Google's block is almost certainly engine
  fingerprinting, not header-based. Bing didn't render any better either.
  Opening the user's real default browser is the reliable approach. See README
  v1.4–v1.7 for the full trail.
- **Embedding the Inter font.** Technically possible (the guide's variable woff2
  converts to ttf cleanly) but it means shipping a second file and fighting WPF's
  shaky variable-font support. Not worth it for a single-file tool.

## Packaging

`Build-Exe.ps1` compiles via `ps2exe` with `-iconFile app-icon.ico -noConsole
-requireAdmin -STA`. **Must run on Windows** — it can't be built or tested from a
Linux sandbox, so changes made here are unverified until run on a real machine.
Output is unsigned, so expect SmartScreen/AV friction on first run until signed.

## Sparks Tool review status

This project is prepared for release under the Liquidware Sparks Tool License and
has been through the Sparks Tool Project Review Checklist v1 — see
`SPARKS-AUDIT.md` for the audit, what was changed, and what is still open.

Things to keep true when changing this project:

- **Version lives in exactly one place:** `$AppVersion` in
  `Set-ProfileUnitySplashScreenLogo.ps1`. `Build-Exe.ps1` reads it; the SBOM,
  the exe metadata and the zip filename all derive from it. Never hardcode it
  a second time. Bump it for every release, including fix-only releases, and add
  a `CHANGELOG.md` entry.
- **`Spark_License.pdf` and `bom.cdx.json` ship side by side** at the top level
  of the repo and of every distributable, and both the README and the About
  dialog point at them. Don't move or bury either.
- **Any dependency change invalidates the SBOM and the CVE scan.** Regenerate
  `bom.cdx.json` and re-run Grype against it, in that order, before shipping.
- **`-LiteralPath`, not `-Path`,** on every filesystem call. Browser-named files
  like `logo[1].png` are a normal input for this tool, and `-Path` treats the
  brackets as a wildcard. `New-Item` is the exception — it has no
  `-LiteralPath` parameter.
- **Timestamps use `Get-TimestampString` / `ConvertTo-SortableDate`,** never a
  bare `.ToString('...')` or `[datetime]` cast. `:` in a .NET custom format
  string is the culture's time separator, not a literal.
- **Never commit a PrimeNG/PrimeUI license key** (or any other credential). This
  is a WPF app with no web framework, so it has no legitimate use for one.
- Run `tests/Invoke-LogicTests.ps1` after touching the manifest, timestamp or
  file-path logic. It lifts the functions out of the app with the PowerShell
  parser, so it tests the real code rather than a copy.
