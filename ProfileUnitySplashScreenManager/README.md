# ProfileUnity SplashScreen Logo Manager

A single-file PowerShell + WPF GUI tool for setting the ProfileUnity client
splash screen logo, per Liquidware KB
[12914471137293](https://support.liquidware.com/hc/en-us/articles/12914471137293-ProfileUnity-adding-custom-logo-to-splash-screen).

Branded to the **Liquidware / Stratusphere UX design system** (from the
supplied style guide export):

- **Colors:** brand blue `#0061A0` (600) header bar, `#005084`/`#003F67`
  hover/press states, zinc-based neutral surfaces, and the Good/Poor
  (`#16A34A` / `#DC2626`) semantic colors reused for success/error status
  text — same GFP vocabulary the product uses for data quality.
- **Layout:** fixed 48px header bar (matches the app shell spec exactly),
  flat cards with 8px radius and a whisper-soft shadow, 4px radius on
  buttons/fields, 1px `surface-300` borders.
- **Brand marks:** the Liquidware wordmark (white, for the dark header) and
  the flame app icon are embedded as PNGs rasterized from the style guide's
  SVGs. The pale hex-honeycomb brand texture bleeds subtly from the top-left
  corner, same treatment the guide describes for the login/loading screens.
- **Type:** base 14px / header 16px per the guide's scale. Font is **Segoe
  UI**, not Inter — WPF can't load the variable **`.woff2`** the guide ships,
  and Segoe UI is the guide's own documented fallback in its `--font-sans`
  stack, so this isn't a deviation from spec.
- **Copy:** Title Case for labels/buttons, terse impersonal system messages
  ("Splash logo updated", "An error occurred: …"), no emoji — matching the
  product's documented voice.

## What it does

- Lets you browse for a logo image (`.bmp`, `.jpg`/`.jpeg`, `.gif`, `.png`, `.tif`/`.tiff`).
- Drops it into `C:\Program Files\ProfileUnity\Client.NET` as
  `client-custom-logo-300x86.<ext>` (`.jpeg` → `.jpg`, `.tiff` → `.tif`, to
  match the exact names ProfileUnity looks for).
- Before overwriting, moves whatever logo is currently live into a **history**
  folder so nothing is ever lost.
- Shows a history list (date set, original filename) with **Restore** and
  **Delete** buttons, so you can flip back to a previous logo in one click.
- Warns (non-blocking) if the selected/current image isn't exactly 300×86,
  since that's the recommended splash size.

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
2. Run it: `powershell -ExecutionPolicy Bypass -File .\Set-ProfileUnitySplashScreenLogo.ps1`
   - It will self-elevate (UAC prompt) since writing to `Program Files`
     requires admin rights.
3. Type a term into **Search Images** and click **Search** (or press Enter)
   — this opens an image search in your **default browser** (Chrome, Edge,
   whatever it's set to). Right-click an image and choose **Copy image**.
4. Alt-tab back to this app and click **Import from Clipboard** — the image
   is pulled straight off the clipboard into the preview.
5. Alternatively, click **Browse...** to pick a file already saved on disk —
   either way, nothing is written to `Client.NET` yet at this point.
6. Click **Set as Splash Logo** to commit it (this is when the current live
   logo gets archived to history and the new one gets copied in).
7. Click **Preview Splash Screen** to launch
   `LwL.ProfileUnity.Client.CtxInit.exe` from the target folder, so you can see
   the logo in the actual splash screen without waiting for a real logon.
8. To go back to an earlier logo, select it in the history grid and click
   **Restore Selected**.

### Optional: point it at a non-default path

If your Client.NET lives somewhere else (e.g. you're staging it before
deployment), pass `-TargetDir`:

```powershell
.\Set-ProfileUnitySplashScreenLogo.ps1 -TargetDir "D:\Staging\ProfileUnity\Client.NET"
```

## Notes / limitations

- This only touches the file-drop method (KB step 4a) — it does not edit
  `LwL.ProfileUnity.Client.Splash.exe.config` (step 4b: `LogoImageLocation`,
  `LogoSizeMode`, fore/back colors). That's a separate, rarer customization
  and wasn't in scope here — say the word if you want a config-editing mode
  added too.
- No image resizing/validation beyond a dimension warning — it copies
  whatever file you pick, byte-for-byte.
- Designed for interactive use on a single machine/image. If you want this
  driven from your MSP console or pushed at scale across environments
  (KA/HAAGNET, DEVENTER, etc.), it'd be straightforward to split the core
  functions out into a script you call non-interactively (`-SourcePath` +
  `-TargetDir`, no GUI) — let me know if that's useful.

## Changelog

- v1.0 — Initial version: browse/set logo, history with restore/delete,
  self-elevation, dimension warning.
- v1.1 — Rebranded to the Liquidware / Stratusphere UX design system: brand
  colors, header bar with wordmark, hex texture accent, card/button/grid
  styling, Title Case copy.
- v1.2 — Split Browse from Set: picking a file now previews it directly (read
  straight from the source path, nothing written until you commit). The old
  "Refresh" button now launches `LwL.ProfileUnity.Client.CtxInit.exe` from the
  target folder to preview the real splash screen.
- v1.3 — Added a "Search Google Images" box: opens a Google Images search for
  your term in the default browser so you can find and save a logo, then
  Browse it in as before. No API key, no scraping — just a plain search URL.
- v1.4 — The search now opens an in-app popout with an embedded browser
  instead of your default browser. Right-click an image → "Copy image" →
  "Import Selected Image" pulls it from the clipboard directly into the
  preview, no manual save/browse round-trip needed. Falls back to opening
  your default browser if the embedded control can't load.
- v1.5 — Fixed Google showing a bot-check page ("problemen met de toegang
  tot Google Zoeken...") inside the popout: the embedded control's default
  user agent identifies as a very old Internet Explorer build, which Google
  blocks. Now spoofs a modern Chrome/Edge user agent before navigating. Also
  added a small address bar with Back/Reload to the popout, so you can retry
  or switch to another image search (e.g. Bing Images) without closing it.
- v1.6 — User-agent spoofing wasn't enough — Google kept blocking the
  popout, most likely fingerprinting the underlying legacy engine itself
  rather than just its user-agent string. Switched the popout's default
  search engine to **Bing Images**, which doesn't hard-block it. Google is
  still reachable by typing a URL into the popout's address bar if you want
  to try it on your network.
- v1.7 — Removed the embedded-browser popout entirely. It turned out to be a
  dead end: the legacy IE11/Trident engine behind it has no WebP decoder at
  all, so most modern image thumbnails just showed as broken-image icons,
  and its CSS/JS support is too far behind to lay out a page like Bing/Google
  Images correctly (the cookie-consent banner rendered as garbled overlapping
  text). No amount of tweaking fixes a missing image codec or engine-level
  layout gaps. **Search now opens your actual default browser** instead
  (reliable, modern rendering, no dead ends), and a new **Import from
  Clipboard** button in the main window lets you pull in whatever you
  right-click-copied there — same "search, select, import" flow, just built
  on a browser that actually works.

### About the image search flow

- **Search** opens a plain image search URL in your system's default
  browser — nothing embedded, nothing scraped, just a normal browser tab.
- Right-click any image there and choose **Copy image** (standard in
  Chrome/Edge/Firefox), then come back to this app and click **Import from
  Clipboard**. The tool only ever reads whatever the browser itself puts on
  the clipboard — it doesn't parse or scrape the search results page.
- We tried an in-app popout with an embedded browser control first (so you'd
  never have to leave the app), but the only browser engine available
  without adding external dependencies to a single-file script is the legacy
  Windows Forms `WebBrowser` control (IE11/Trident). That engine has no WebP
  image support and incomplete modern CSS/JS support, so sites like Google
  or Bing Images don't render correctly in it — broken image icons and
  garbled layouts, not something fixable with configuration. Opening your
  real browser sidesteps that entirely.

## Building the .exe (optional)

If you'd rather hand out a single `.exe` than a `.ps1`, use `Build-Exe.ps1`
(included) together with `app-icon.ico` (also included, the Liquidware flame
mark rendered at 16/32/48/64/128/256px for a crisp icon at every size).

This has to be run **on a Windows machine, in Windows PowerShell** — ps2exe
works by compiling a native Windows executable that hosts the PowerShell
engine, and the tool itself is WPF/WinForms, so none of this can happen on
anything other than Windows.

```powershell
# From the folder containing Set-ProfileUnitySplashScreenLogo.ps1, app-icon.ico,
# and Build-Exe.ps1:
.\Build-Exe.ps1
```

This will:
1. Install the `ps2exe` module from the PowerShell Gallery if it isn't
   already present (current-user scope, no admin needed for this step).
2. Compile `Set-ProfileUnitySplashScreenLogo.ps1` into
   `ProfileUnitySplashScreenLogoManager.exe`, using:
   - `-iconFile app-icon.ico` — so the exe (and its taskbar/window icon) uses
     the Liquidware mark instead of the generic PowerShell icon.
   - `-noConsole` — since it's a GUI app, no console window should flash up.
   - `-requireAdmin` — embeds a manifest so Windows itself prompts for
     elevation (UAC) right when the exe launches, which is cleaner than the
     script's own self-elevation relaunch trick. (That internal logic is
     still in the script as a fallback and won't conflict — it just checks
     "am I already admin?" and does nothing if so.)
   - `-STA` — WPF requires single-threaded apartment; without this flag some
     ps2exe versions default to MTA and the UI won't load.

A couple of things worth knowing:
- The result is an **unsigned** executable. Windows SmartScreen or your AV
  may flag it on first run (common for any unsigned ps2exe output, not
  specific to this script) — if you're distributing it beyond your own
  machine, code-signing it removes that friction.
- Everything else about the tool is unchanged — same self-elevation,
  history, search-and-import flow, and branding.
