# Changelog

Versions are `MAJOR.MINOR.PATCH`. The version shown in the tool's title bar,
header tag and About dialog, the version in `bom.cdx.json`'s
`metadata.component.version`, the version stamped into the executable's file
metadata, and the version in the distributable's filename all come from the
single `$AppVersion` constant in `Set-ProfileUnitySplashScreenLogo.ps1`.

## 0.2.0

First release prepared for submission as a Liquidware Sparks Tool.

**Version scheme reset.** Pre-submission builds were numbered v1.0 through v1.7
in the README's own changelog (kept below). Those were internal iterations, not
released products. This submission restarts at `0.2.0` to match the numbering
used by the other Sparks Tools and to reflect field-tool maturity rather than
product maturity. Nothing about the tool regressed.

Added for the Sparks Tool review:

- `Spark_License.pdf` (Sparks Tool License and Disclaimer v1.0) and
  `bom.cdx.json` (CycloneDX 1.6 SBOM) now ship at the top level, side by side.
- `THIRD-PARTY-NOTICES.txt` covering ps2exe, whose generated host stub is
  embedded in the compiled `.exe`.
- An **About** dialog, reachable from the header bar, showing the version,
  reproducing the license's core disclaimers, and opening the license PDF and
  the SBOM directly.
- The version is now visible in the window title and as a header tag, without
  opening anything.
- `Build-Exe.ps1` is now the build authority: it reads `$AppVersion` from the
  script rather than hardcoding a version, refuses to install an unpinned
  ps2exe, rewrites `bom.cdx.json` with the ps2exe version actually resolved at
  build time plus the executable's SHA-256, and packages the distributable zip.

Fixed:

- **Filenames containing wildcard metacharacters no longer break the tool.**
  Every filesystem call now uses `-LiteralPath`. Previously a file such as
  `logo[1].png` — the name browsers give repeat downloads, which is exactly what
  the search-and-import flow produces — was treated as a wildcard pattern: the
  preview silently showed nothing and the copy failed *after* the live logo had
  already been archived and deleted, leaving no splash logo in place.
- **History no longer breaks on non-US locales.** Timestamps are written and
  parsed with the invariant culture. `:` in a .NET custom format string is the
  culture's time-separator placeholder rather than a literal, so on a locale
  whose separator is `.` the manifest was written as `14.39.00` and then failed
  to parse back, throwing inside the history grid's sort. Entries written by
  earlier builds still load.
- **Deleting the last history entry no longer leaves a phantom row.** Piping an
  empty collection to `ConvertTo-Json` sent nothing downstream, so `Set-Content`
  was never given a value and the manifest kept its previous contents — the
  deleted entry stayed in the list while its backing file was already gone, and
  restoring it failed. The manifest is now always written as a JSON array.
- **Setting the live logo as its own source is refused** instead of destroying
  it. Archiving deletes the live file before the copy runs, so choosing the live
  logo as the source left the machine with no splash logo at all.
- **Stray logo files are no longer silently ignored.** If more than one
  `client-custom-logo-300x86.*` file is present — possible after an interrupted
  run — all of them are archived rather than just the first, and the UI warns,
  because ProfileUnity may read a different one than the tool is managing.
- **Files that are not really images are rejected before they are applied.** A
  file with an image extension that fails to decode no longer enables
  **Set as Splash Logo**; previously it could be copied into `Client.NET`, where
  ProfileUnity would render no logo.
- Both `.ps1` files are saved as UTF-8 with BOM, and the manifest and metadata
  JSON files are read with an explicit encoding.
- Image dimensions are compared as integers rather than as formatted strings,
  and an undecodable file now says so instead of reporting its size as
  "unknown".
- Clipboard imports use a GUID-suffixed temp filename, so two imports within the
  same second no longer overwrite each other.

Changed:

- Type sizes now use the style guide's scale: grid cells 12px (`--text-sm`),
  body and status text 14px (`--text-base`), card titles 16px (`--text-md`),
  version tag 10.5px (`--text-xs`). The previous 13px is not a token in the
  guide.
- The README now discloses the one external endpoint the tool uses.

## Pre-submission history

Kept verbatim from the README's original changelog, for continuity.

- **v1.0** — Initial version: browse/set logo, history with restore/delete,
  self-elevation, dimension warning.
- **v1.1** — Rebranded to the Liquidware / Stratusphere UX design system: brand
  colors, header bar with wordmark, hex texture accent, card/button/grid
  styling, Title Case copy.
- **v1.2** — Split Browse from Set: picking a file now previews it directly
  (read straight from the source path, nothing written until you commit). The
  old "Refresh" button now launches `LwL.ProfileUnity.Client.CtxInit.exe` from
  the target folder to preview the real splash screen.
- **v1.3** — Added a "Search Google Images" box: opens a Google Images search
  for your term in the default browser so you can find and save a logo, then
  Browse it in as before. No API key, no scraping — just a plain search URL.
- **v1.4** — The search now opens an in-app popout with an embedded browser
  instead of your default browser. Right-click an image → "Copy image" →
  "Import Selected Image" pulls it from the clipboard directly into the preview.
  Falls back to opening your default browser if the embedded control can't load.
- **v1.5** — Fixed Google showing a bot-check page inside the popout: the
  embedded control's default user agent identifies as a very old Internet
  Explorer build, which Google blocks. Now spoofs a modern Chrome/Edge user
  agent before navigating. Also added a small address bar with Back/Reload.
- **v1.6** — User-agent spoofing wasn't enough — Google kept blocking the
  popout, most likely fingerprinting the underlying legacy engine itself rather
  than just its user-agent string. Switched the popout's default search engine
  to Bing Images, which doesn't hard-block it.
- **v1.7** — Removed the embedded-browser popout entirely. The legacy
  IE11/Trident engine behind it has no WebP decoder at all, so most modern image
  thumbnails showed as broken-image icons, and its CSS/JS support is too far
  behind to lay out a page like Bing/Google Images correctly. No amount of
  tweaking fixes a missing image codec or engine-level layout gaps. Search now
  opens your actual default browser, and a new **Import from Clipboard** button
  pulls in whatever you right-click-copied there.
