# ProfileUnity SplashScreen Logo Manager

**Version 0.2.0**

> **IMPORTANT: READ BEFORE DOWNLOADING OR USING.** This is a Liquidware
> **Sparks Tool** — a community/field-contributed utility, **not a Liquidware
> commercial product**. It is provided outside Liquidware's standard product
> development lifecycle, **"AS IS" with no warranty, support, or maintenance**,
> and is used at your own risk. See `Spark_License.pdf` for the full license and
> disclaimer.

A single-file PowerShell + WPF GUI tool for setting the ProfileUnity client
splash screen logo, per Liquidware KB
[12914471137293](https://support.liquidware.com/hc/en-us/articles/12914471137293-ProfileUnity-adding-custom-logo-to-splash-screen).

## Files

| File | Purpose |
|---|---|
| `Set-ProfileUnitySplashScreenLogo.ps1` | The application itself |
| `ProfileUnitySplashScreenManager.exe` | Optional compiled build of the same script (see [Building the .exe](#building-the-exe)) |
| `Build-Exe.ps1` | Compiles the exe, finalises the SBOM, and packages the distributable |
| `app-icon.ico` | Application icon, rendered at 16/32/48/64/128/256px |
| `Spark_License.pdf` | **Liquidware Sparks Tool License and Disclaimer — read this before use** |
| `bom.cdx.json` | Software Bill of Materials (CycloneDX 1.6, JSON) — an inventory of the third-party components in this tool, provided so your security team can review it against your own policy |
| `THIRD-PARTY-NOTICES.txt` | Third-party license texts and attribution notices |
| `CHANGELOG.md` | Version history |

The license PDF and the SBOM also open directly from the tool's **About**
dialog, in the header bar.

## What it does

- Lets you browse for a logo image (`.bmp`, `.jpg`/`.jpeg`, `.gif`, `.png`,
  `.tif`/`.tiff`), or pull one straight off the Windows clipboard.
- Drops it into `C:\Program Files\ProfileUnity\Client.NET` as
  `client-custom-logo-300x86.<ext>` (`.jpeg` → `.jpg`, `.tiff` → `.tif`, to
  match the exact names ProfileUnity looks for).
- Before overwriting, moves whatever logo is currently live into a **history**
  folder so nothing is ever lost.
- Shows a history list (date archived, original filename) with **Restore
  Selected** and **Delete Selected From History**, so you can flip back to a
  previous logo in one click.
- Warns (non-blocking) if the selected image isn't exactly 300×86, since that's
  the recommended splash size, and refuses files that don't decode as images.
- Launches the real splash screen so you can check the logo in context without
  waiting for a logon.

## Where things are stored

| What | Location |
|---|---|
| Live splash logo (what ProfileUnity actually reads) | `C:\Program Files\ProfileUnity\Client.NET\client-custom-logo-300x86.<ext>` |
| History files + manifest | `C:\ProgramData\Liquidware\ProfileUnitySplashScreenLogoManager\` |

Keeping history under `ProgramData` (rather than inside `Client.NET`) means it
survives a ProfileUnity client reinstall/upgrade.

## Usage

1. Copy `Set-ProfileUnitySplashScreenLogo.ps1` to the machine (or golden image)
   where the ProfileUnity Client.NET folder lives.
2. Unblock it (it came from outside the machine):
   ```powershell
   Unblock-File .\Set-ProfileUnitySplashScreenLogo.ps1
   ```
3. Run it: `powershell -ExecutionPolicy Bypass -File .\Set-ProfileUnitySplashScreenLogo.ps1`
   - It will self-elevate (UAC prompt), since writing to `Program Files`
     requires admin rights.
4. Type a term into **Search Images** and click **Search** (or press Enter) —
   this opens an image search in your **default browser**. Right-click an image
   and choose **Copy image**.
5. Alt-tab back and click **Import from Clipboard** — the image is pulled
   straight off the clipboard into the preview.
6. Or click **Browse...** to pick a file already on disk. Either way, nothing is
   written to `Client.NET` yet.
7. Click **Set as Splash Logo** to commit it. This is when the current live logo
   gets archived to history and the new one is copied in.
8. Click **Preview Splash Screen** to launch
   `LwL.ProfileUnity.Client.CtxInit.exe` from the target folder and see the logo
   in the actual splash screen.
9. To go back to an earlier logo, select it in the history grid and click
   **Restore Selected**.

### Optional: point it at a non-default path

If your Client.NET lives somewhere else (e.g. you're staging it before
deployment), pass `-TargetDir`:

```powershell
.\Set-ProfileUnitySplashScreenLogo.ps1 -TargetDir "D:\Staging\ProfileUnity\Client.NET"
```

## Version

The version is shown in the window title, as a tag in the header bar, and in the
**About** dialog. It comes from a single `$AppVersion` constant in the script;
`Build-Exe.ps1` reads that same value for the executable's file metadata, the
SBOM and the distributable's filename.

## External network access

Every function of this tool is local except one, and it needs no network access
to set, archive, restore or preview a logo.

| Host | When | What is sent | Notes |
|---|---|---|---|
| `www.google.com` | Only when you click **Search** | The search term you type, URL-encoded | Opened as a normal tab in your own default browser. The tool hands the URL to the shell and does nothing else — nothing is embedded, fetched, parsed or scraped by the tool. HTTPS. |
| `www.powershellgallery.com` | Build time only, and only if you run `Build-Exe.ps1` | Nothing beyond the module request | Fetches the pinned `ps2exe` version. Not used by the tool at runtime and not required to run it. |

There is no telemetry, no analytics, no error reporting and no update check. All
brand artwork is embedded in the script rather than loaded from a CDN, and no
fonts, images or scripts are fetched at runtime.

**In an air-gapped or egress-restricted environment**, the tool works normally;
only **Search** is affected, and it fails in your browser rather than in the
tool. If outbound web access is blocked by policy, use **Browse...** with a logo
file you already have.

## Notes and limitations

- This only touches the file-drop method (KB step 4a) — it does not edit
  `LwL.ProfileUnity.Client.Splash.exe.config` (step 4b: `LogoImageLocation`,
  `LogoSizeMode`, fore/back colors). That's a separate, rarer customization and
  wasn't in scope.
- No image resizing. Beyond a dimension warning and an is-this-really-an-image
  check, the file is copied byte-for-byte.
- Designed for interactive use on a single machine or image. If you want this
  driven from a console or pushed at scale, the core functions would split out
  into a non-interactive script (`-SourcePath` + `-TargetDir`, no GUI) without
  much work.
- The compiled `.exe` is **unsigned**, so Windows SmartScreen or your AV may flag
  it on first run. This is common for any unsigned ps2exe output and not
  specific to this script; code-signing removes the friction.

### About the image search flow

- **Search** opens a plain image search URL in your system's default browser —
  nothing embedded, nothing scraped, just a normal browser tab.
- Right-click any image there and choose **Copy image** (standard in
  Chrome/Edge/Firefox), then come back and click **Import from Clipboard**. The
  tool only ever reads what the browser itself puts on the clipboard.
- An in-app popout with an embedded browser was tried first and abandoned. The
  only browser engine available without adding external dependencies to a
  single-file script is the legacy Windows Forms `WebBrowser` control
  (IE11/Trident). It has no WebP decoder and incomplete modern CSS/JS support,
  so image search sites render as broken-image icons and garbled layouts — not
  something fixable with configuration. See `CHANGELOG.md` (v1.4–v1.7) for the
  full trail.

## Building the .exe

`Build-Exe.ps1` compiles the script with `ps2exe`, finalises `bom.cdx.json`, and
packages `ProfileUnitySplashScreenManager-<version>.zip`.

This has to be run **on a Windows machine, in Windows PowerShell** — ps2exe
compiles a native Windows executable that hosts the PowerShell engine, and the
tool itself is WPF/WinForms, so none of it can happen anywhere else.

```powershell
# ps2exe must be pinned to an exact version. To see what's available:
Find-Module ps2exe -AllVersions | Select-Object -First 5 Version, PublishedDate

.\Build-Exe.ps1 -Ps2ExeVersion '<version>'
```

If `ps2exe` is already installed, `-Ps2ExeVersion` can be omitted and whichever
version is installed is used and recorded. The script will **not** install an
unpinned "latest" version.

The build:

1. Reads `$AppVersion` from the script — the version is never hardcoded in the
   build script.
2. Compiles with `-iconFile app-icon.ico -noConsole -requireAdmin -STA`.
   `-requireAdmin` makes Windows prompt for elevation as the exe launches, which
   is cleaner than the script's own relaunch; `-STA` is required by WPF.
3. Rewrites `bom.cdx.json` with the ps2exe version actually resolved, the
   matching tool version, and the executable's SHA-256.
4. Packages the zip with `Spark_License.pdf` and `bom.cdx.json` side by side at
   the top level.

**After building, re-run the Grype scan against the regenerated
`bom.cdx.json`** — the SBOM changed, so an earlier scan no longer describes what
ships.

## Changelog

See `CHANGELOG.md`.
