# ProfileUnity SplashScreen Logo Manager

**Version 0.3.0**

> **IMPORTANT: READ BEFORE DOWNLOADING OR USING.** This is a Liquidware
> **Sparks Tool** — a community/field-contributed utility, **not a Liquidware
> commercial product**. It is provided outside Liquidware's standard product
> development lifecycle, **"AS IS" with no warranty, support, or maintenance**,
> and is used at your own risk. See `Spark_License.pdf` for the full license and
> disclaimer.

A Windows desktop tool for setting the ProfileUnity client splash screen logo,
per Liquidware KB
[12914471137293](https://support.liquidware.com/hc/en-us/articles/12914471137293-ProfileUnity-adding-custom-logo-to-splash-screen).

A single self-contained `.exe`: a Go service does the privileged work, and an
Angular user interface — built with PrimeNG and the Liquidware design system —
renders in a native WebView2 window. Everything, including fonts, is embedded in
the executable.

## Files in the distributable

| File | Purpose |
|---|---|
| `ProfileUnitySplashScreenManager.exe` | The application. Self-contained; no installer, no runtime to deploy except WebView2 (below) |
| `Spark_License.pdf` | **Liquidware Sparks Tool License and Disclaimer — read this before use** |
| `bom.cdx.json` | Software Bill of Materials (CycloneDX 1.6, JSON) — an inventory of the third-party components in this tool, provided so your security team can review it against your own policy |
| `THIRD-PARTY-NOTICES.txt` | Third-party license texts and attribution notices |
| `README.md` | This file |
| `CHANGELOG.md` | Version history |

The license PDF, the SBOM and the notices also open directly from the tool's
**About** dialog, in the header bar.

## Requirements

- **Windows** x64.
- **Administrator rights.** The splash logo lives under `Program Files`, so the
  executable carries a `requireAdministrator` manifest and Windows prompts for
  elevation as it launches.
- **Microsoft Edge WebView2 Runtime.** Included with Windows 11 and with fully
  patched Windows 10. If it is missing the tool says so plainly and exits rather
  than showing a blank window. On a locked-down image without it, install
  Microsoft's Evergreen Standalone Installer, then start the tool again.

## What it does

- Browse for a logo image (`.bmp`, `.jpg`/`.jpeg`, `.gif`, `.png`, `.tif`/`.tiff`),
  or pull one straight off the Windows clipboard.
- Writes it into `C:\Program Files\ProfileUnity\Client.NET` as
  `client-custom-logo-300x86.<ext>` (`.jpeg` → `.jpg`, `.tiff` → `.tif`, matching
  the exact names ProfileUnity looks for).
- Before overwriting, archives whatever logo is currently live, so nothing is
  ever lost.
- Shows a history list with **Restore Selected** and **Delete Selected From
  History**, so you can go back to a previous logo in one click.
- Warns (without blocking) when an image is not the recommended 300×86, and
  refuses files that are not really images.
- Launches the real splash screen so you can check the logo in context without
  waiting for a logon.

## Where things are stored

| What | Location |
|---|---|
| Live splash logo (what ProfileUnity reads) | `C:\Program Files\ProfileUnity\Client.NET\client-custom-logo-300x86.<ext>` |
| History files and manifest | `C:\ProgramData\Liquidware\ProfileUnitySplashScreenLogoManager\` |

History lives under `ProgramData` rather than inside `Client.NET` so it survives
a ProfileUnity client reinstall or upgrade. A history manifest written by the
earlier PowerShell version of this tool (0.2.0 and before) is read as-is, so
upgrading does not lose existing history.

## Usage

1. Copy the distributable's contents to the machine or golden image where the
   ProfileUnity Client.NET folder lives, keeping the files together.
2. Run `ProfileUnitySplashScreenManager.exe` and accept the UAC prompt.
3. Type a term into **Search Images** and press Enter — this opens an image
   search in your **default browser**. Right-click an image and choose **Copy
   image**.
4. Alt-tab back and choose **Import From Clipboard**. Or choose **Browse…** to
   pick a file already on disk. Either way nothing is written to `Client.NET`
   yet — the image is only previewed.
5. Choose **Set As Splash Logo** to apply it. This is when the current logo is
   archived and the new one is written.
6. Choose **Preview Splash Screen** to launch
   `LwL.ProfileUnity.Client.CtxInit.exe` and see the logo in the real splash
   screen.
7. To go back, select an entry in the history table and choose **Restore
   Selected**.

### Command-line options

```
ProfileUnitySplashScreenManager.exe [options]

  -target-dir <path>   ProfileUnity Client.NET folder to write into. Point this
                       elsewhere to stage a logo before deployment.
                       Default: %ProgramFiles%\ProfileUnity\Client.NET
  -search-url <tmpl>   Image-search URL template; %s is the encoded query.
                       Pass an empty string to disable image search entirely,
                       for air-gapped or policy-restricted sites.
  -no-elevate          Do not attempt to relaunch elevated; fail instead.
  -version             Print the version and exit.
```

## Version

The version is shown in the window title, as a tag in the header bar, and in the
**About** dialog, and `-version` prints it. It comes from a single constant,
`AppVersion` in `internal/version/version.go`; the build reads that same value
for the executable's file metadata, the SBOM and the distributable's filename.

## External network access

Every function of this tool is local. It needs no network access to set,
archive, restore or preview a logo, and it makes no network requests of its own.

| Host | When | What is sent | Notes |
|---|---|---|---|
| `www.google.com` | Only when you use **Search Images** | The search term you type, URL-encoded | Opened as a normal tab in your own default browser. The tool hands the URL to the Windows shell and does nothing else — nothing is embedded, fetched, parsed or scraped by the tool. HTTPS. |

There is no telemetry, no analytics, no error reporting and no update check. All
scripts, styles, brand artwork and the Inter webfont are embedded in the
executable, so nothing is fetched from a CDN at runtime.

**In an air-gapped or egress-restricted environment** the tool works normally.
Only **Search Images** is affected, and it fails in your browser rather than in
the tool. To remove the feature entirely, start with `-search-url ""` — the
button disappears and the About dialog reports that the tool makes no network
requests.

### Local listener

The user interface talks to the Go service over HTTP on `127.0.0.1` with an
ephemeral port. This is not reachable from the network. Every API call must
present a token generated fresh at each start and handed to the page by the
WebView; requests carrying a foreign `Origin` or `Referer` are refused, and no
CORS headers are ever sent. The interface cannot name a file to apply — browse
and clipboard import record the candidate inside the service, and the interface
applies "the pending file" — so the API is never a way to copy an arbitrary file
into `Program Files`.

## PrimeNG licensing

**This needs a decision before the tool is distributed.**

PrimeNG was MIT-licensed through 17.x. **Version 18.0 and later are proprietary**,
under the PrimeUI Commercial License from PrimeTek Informatics. This project uses
PrimeNG 22.

PrimeNG takes its license key as a client-side configuration value, so **the key
is compiled into the JavaScript bundle and therefore into the shipped
executable**. That is PrimeTek's intended mechanism for distributing an
application — the developer is licensed and end users do not need their own key —
but it does mean a per-developer key belonging to Liquidware becomes extractable
from any copy of the binary.

The key is therefore never committed and never defaulted. It is supplied at build
time or not at all:

```powershell
$env:PRIMEUI_LICENSE_KEY = '<key>'   # embedded in the build
.\build\Build.ps1
```

Build without it and the tool works, but PrimeNG injects a red **"Invalid
PrimeUI License"** banner into the window. The banner sits in a closed shadow
root and cannot be styled away.

Section 3 of the Sparks Tool Project Review Checklist forbids credentials in
shipped configuration, so **embedding the key requires a written reviewer
exception.** See `SPARKS-AUDIT.md`.

## Building

The application is CGO-free, so the Windows executable **cross-compiles from
Linux or macOS** as well as building natively on Windows.

Requirements:

- **Go** 1.24 or newer.
- **Node.js 22.22.3 or newer** (the Angular CLI's floor). Only needed to rebuild
  the user interface; `-skip-ui` reuses the existing output.

```bash
./build/build.sh                 # Linux/macOS
```
```powershell
.\build\Build.ps1                # Windows
```

The build:

1. Checks the toolchain, including the Node floor.
2. Installs web dependencies from the lockfile (`npm ci`).
3. Builds the Angular application.
4. **Scans the built UI for external references** and fails if any runtime asset
   points at a host — this is checklist section 3, enforced rather than assumed.
5. Embeds the UI into the Go binary, then restores the committed placeholder so
   the working tree stays clean.
6. Compiles the `.exe` with the icon, version metadata and the
   `requireAdministrator` manifest, as a GUI subsystem binary so no console
   window appears.
7. Regenerates `bom.cdx.json` and `THIRD-PARTY-NOTICES.txt` from what was
   actually linked and bundled.
8. Packages `ProfileUnitySplashScreenManager-<version>.zip` with the license PDF
   and the SBOM side by side.

**After building, run Grype against the regenerated `bom.cdx.json`** — the SBOM
changed, so an earlier scan no longer describes what ships:

```bash
grype db update
grype sbom:./bom.cdx.json --fail-on high
```

### Tests

```bash
go test ./...            # core logic and the HTTP API, including a live server
go vet ./...
```

The Windows build additionally needs `go vet -unsafeptr=false`: reading a
clipboard bitmap requires a `uintptr`-to-`unsafe.Pointer` conversion that the
check flags, and the conversion is isolated in one documented function
(`copyFromNative`). Everything else is checked normally.

## Notes and limitations

- This only implements the file-drop method (KB step 4a). It does not edit
  `LwL.ProfileUnity.Client.Splash.exe.config` (step 4b: `LogoImageLocation`,
  `LogoSizeMode`, fore/back colours).
- No image resizing. Beyond the dimension warning and a decode check, the file is
  copied byte for byte.
- The executable is **unsigned**, so SmartScreen or your AV may flag it on first
  run. Code-signing removes the friction.
- Designed for interactive use on one machine or image.

## Changelog

See `CHANGELOG.md`.
