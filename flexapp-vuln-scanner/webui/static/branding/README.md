# Brand assets

Pulled from Liquidware's "Stratusphere UX" design system export (provided
directly by the project owner) and adapted for this tool's plain
server-rendered pages — see `../style.css`'s header comment and
`PLAN.md`/`CHANGELOG.md` for what was and wasn't carried over, and why.

- `assets/logo.svg` — app icon (flame mark in a circle). Used as favicon.
- `assets/logo-primary-light.svg` — full lockup, white wordmark, for the
  blue header bar.
- `assets/logo-primary.svg` — full lockup, gray wordmark, for light
  backgrounds (not currently used on any page here, kept for completeness).
- `fonts/Inter-roman-var.woff2` / `Inter-italic-var.woff2` — Inter
  variable font. SIL Open Font License 1.1.
- `fonts/material-symbols-rounded.woff2` — Google's Material Symbols
  Rounded icon font, used via `.material-icons` + a ligature name (e.g.
  `<span class="material-icons">folder</span>`). Apache License 2.0.

**PrimeIcons was deliberately not used**, even though it's the design
system's primary icon font: the actual `.woff2` binary isn't included in
the export (only CSS class definitions, meant to load from a CDN in
previews) — see the design system's own README "Substitutions & flags"
note. Pulling an icon font from a CDN would make every page in this local
tool depend on outbound network access it doesn't otherwise need. Material
Symbols Rounded is bundled locally and is the design system's own
documented fallback where PrimeIcons lacks a glyph, so it's a legitimate,
license-clean substitute rather than an approximation.
