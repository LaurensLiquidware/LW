# Liquidware Stratusphere UX — Design System

A design system for **Liquidware's Stratusphere UX** product: an enterprise
**digital experience monitoring (DEM)** platform. Stratusphere UX measures,
scores, and reports on the end-user computing experience across physical,
virtual, and cloud desktops (VDI / DaaS / RDSH) — tracking machines, users,
applications, login/logoff timing, resource consumption (CPU, RAM, disk,
network), and remote-session display quality.

This system captures the product's visual language so design agents can produce
on-brand interfaces, mockups, slides, and prototypes for Liquidware.

---

## Company & product context

- **Company:** Liquidware Labs, Inc. (© 2010–present). Brandname "Liquidware".
- **Product:** **Stratusphere™ UX** — the user-experience monitoring half of the
  Stratusphere line (the companion product is Stratusphere FIT for assessment).
- **Core idea:** Every metric in the product is reduced to a **Good / Fair / Poor**
  ("GFP") rating, so administrators can instantly triage experience quality
  across a fleet. GFP green/yellow/red is the defining data-language of the UI.
- **Primary surfaces (this codebase):** A single-page **web application** (the
  "Stratusphere Hub" / Stratusphere UX web console). Major areas, taken from the
  app's own navigation:
  - **Dashboards** → Overview (Summary, CPU, RAM, Disk, Network, Applications &
    Processes, Login & Logoff, Remote Session Display, Events & Alerts, Browser,
    Trending), Custom Dashboards.
  - **Environment** → Individual Views, Detailed Views, Reports, plus three
    "Legacy" tools (Legacy Search, Legacy Spot Checks, Legacy Advanced Inspector).
  - **Administration** → Status, Configuration, Inventory.
  - Plus **Recent**, **Starred**, **Settings**, and a **Login** screen.

### Sources used to build this system
- **Codebase (read-only):** an Angular application mounted at `src/`. Key files:
  - `src/@template/theme/lwl-theme.ts` — PrimeNG **Aura** preset override; the
    single source of truth for the color scales, radii, and component theming.
  - `src/@template/theme/theme-styles.scss`, `header-custom-styles.scss` — CSS
    variables, shadows, frost/glass, grid layout, GFP colors.
  - `src/styles.scss`, `src/index.html` — fonts, dark-mode hex background, loader.
  - `src/app/pages/**` — page components (login, overview dashboard, etc.).
  - `src/@template/components/**` — app shell (header, side-nav, layout-frame).
  - `src/assets/i18n/en.json` — all UI copy strings.
- **Stack:** Angular (standalone components, signals) · **PrimeNG** component
  library with the **Aura** theme preset · **Tailwind CSS** utilities ·
  PrimeIcons + Material Symbols Rounded icon fonts.

> Note: the reader may not have access to the codebase mount. Everything needed
> to design on-brand has been extracted into this project.

---

## CONTENT FUNDAMENTALS

How Liquidware writes copy inside Stratusphere UX:

- **Voice:** terse, functional, administrator-facing. This is an operations tool,
  not a consumer app — copy names things and triggers actions. No marketing
  fluff, no personality, no jokes.
- **Casing:** **Title Case for everything UI** — navigation items, buttons, menu
  entries, tab labels, field labels. Examples: "Detailed Views", "Custom
  Dashboards", "Login & Logoff", "Auto Refresh", "Schedule Report", "Clear all
  Filters" (note the occasional lowercase connective — mostly consistent Title
  Case).
- **Verbs for actions:** buttons are imperative verbs — "Apply", "Cancel",
  "Save", "Run", "Modify", "Rename", "Refresh", "Share", "Sign In".
- **Person:** essentially impersonal. The product addresses the *data*, not the
  user. There is almost no "you" / "your"; copy reads "Remember me", "No data
  available", "Retrieving data…". The rare second person appears only in error
  recovery: "Please undo your last change and try again…".
- **Status language:** the GFP triad — **"Good" / "Fair" / "Poor"** — is the
  vocabulary for quality. Toggles read "Show Good", "Show Fair", "Show Poor".
- **System / progress messages:** sentence case, present-progressive, ellipsis-
  terminated — "Generating data dictionary…", "Retrieving inspector information…",
  "Finalizing…", "Retrieving data…". Completion: "Ready", "Data dictionary
  generation completed."
- **Errors:** plain, blunt, no apology theatrics — "Incorrect Username or
  Password", "An error has occurred", "No data available".
- **Emoji:** **never.** None anywhere in the product. Do not introduce them.
- **Abbreviations & domain terms:** comfortable with acronyms and jargon —
  "UX", "CPU", "RAM", "GFP", "VDI", "API Options", "Spot Checks", "Inspector",
  "Data Dictionary". The audience is technical IT/EUC admins.
- **Trademark:** the product is written "**Stratusphere™**" / "Stratusphere UX".
  Company line: "Copyright © 2010-{year} Liquidware Labs, Inc".
- **Vibe:** dense, dependable, data-forward enterprise tooling. Think
  monitoring/observability console — closer to a NOC dashboard than a SaaS
  landing page.

---

## VISUAL FOUNDATIONS

### Color
- **Brand blue is the spine.** Primary scale runs `#f2f8fc` (50) → `#0072bc`
  (500, base) → `#001d2f` (950). The primary *action* color is **600 `#0061a0`**
  in the light scheme, hover **700 `#005084`**, active **800 `#003f67`**.
- **Neutrals are Tailwind "zinc"** (cool gray, faintly blue-leaning), 0–950.
  The app canvas is **surface-50 `#fafafa`**; text is **surface-800 `#27272a`**;
  muted text **surface-500 `#71717a`**.
- **GFP semantic colors** are the only other saturated colors used as data:
  Good = green-600 `#16a34a`, Fair = yellow-600 `#ca8a04`, Poor = red-600
  `#dc2626` (each has a brighter `-500` "chart" variant for fills and a `-700`
  hover). In dark mode these lighten to the `-400` steps.
- **Two data-viz extensions of GFP** appear in tables (calibrated to real
  product screenshots): (1) a **UX letter-grade chip** — the 0–3 UX score shown
  as A+/A/A-/B+/… in a fill that walks green → yellow → red (e.g. A- `#01DF01`,
  B+ `#FFFF99`); (2) **heatmap cells** that shade each metric by severity using
  the literal API hexes `#FFFF99` (mild), `#FFCC66` (moderate), `#ff6666`
  (severe, white text). See the Detailed Views screen in the UI kit.
- **Accent flame** `#03a9f4` lives in the logo droplet only — a brighter cyan-
  blue than the brand 500. Use sparingly, mostly via the logo.
- Imagery skews **cool and corporate**: blue monochrome, clean, no warmth, no
  grain, no photography in the app chrome.

### Typography
- **Inter** (shipped as the variable font "Inter var", weights 100–900). Single
  family for everything; no serif, no separate display face.
- **Small root:** `html { font-size: 14px }`, so 1rem = 14px and the whole UI is
  dense. Header runs at 16px. Card titles 1rem / 500. Big metric numbers ~24px,
  bold, in primary blue. Table/dense cells ~12px.
- **Weights:** body 400, button labels **400** (deliberately not bold), card
  titles 500, metric labels & emphasis 600, only headline numbers hit 700.

### Spacing & layout
- **Grid gap of `0.857rem` (12px)** between dashboard widgets and summary tiles.
- **Canonical widget = 350px wide × 17.5rem (245px) tall**; dashboards tile these
  responsively from 1 up to 8 columns. Pinned side-nav is **250px**.
- **Fixed app chrome:** a **48px** header bar pinned top, a collapsible left nav
  (icon-rail when unpinned, 250px when pinned), breadcrumbs row, then a scrolling
  content region. The shell never scrolls; only the content area does.
- Form fields are **compact** (padding 0.5rem × 0.25rem) to match the density.

### Backgrounds & the hex motif
- **The signature brand texture is a honeycomb of pale-blue hexagons** (`lwl-hex.png`)
  that bleeds diagonally in from the **left edge** and fades to nothing. It is the
  one decorative element. It appears on: the **login page** (over `primary-950`),
  the **app loading screen** (over black), and the **dark-mode app canvas**.
- Light mode otherwise = flat `surface-50`. **No gradients** anywhere in product
  chrome (avoid purple/blue SaaS gradients entirely). No repeating patterns
  besides the hex.

### Elevation, cards & borders
- **Cards are flat with whisper-soft shadows**, radius **0.5rem (lg)**, 1rem body
  padding, 1rem/500 titles. Light-mode card shadow:
  `0 0 1px rgba(0,0,0,.6), 0 1px 2px rgba(0,0,0,.2)` — barely-there.
- **Widgets** add a slightly firmer two-layer shadow keyed to surface-400/500.
- **Borders** are 1px in `surface-300` (light) / `surface-700` (dark) — the
  `--frame-border-color`. Dividers (e.g. nav edge) are 1px hairlines.
- **Radii:** buttons & fields use **sm (0.25rem)**; cards use **lg (0.5rem)**; the
  login panel uses **2xl (1rem)**; the login inputs are fully rounded pills
  (`rounded-3xl`).

### Transparency, blur & "frost"
- **Frost/glass** is reserved for floating layers: context menus and the login
  form panel use `rgba(255,255,255,.75)` + `backdrop-filter: blur(5px)` (login
  uses blur(10px) over a darker tint) with a 1px hairline border. In dark mode
  frost flips to `rgba(0,0,0,.75)`.
- Dark-mode cards are themselves translucent — a primary-600 tint at 20% opacity
  over the hex background.

### Motion
- **Restrained.** The only defined animations are **fade in / fade out at
  150ms and 250ms** (`@keyframes fadeIn/fadeOut`) and the login loader's 4s bar.
  No bounces, no springs, no parallax. Default easing is `ease-in-out`; the
  side-nav width transitions at `0.25s ease-in-out`. Respect this calm: prefer
  short opacity/position fades.

### Hover & press states
- **Buttons (text/outlined):** hover = a faint primary wash
  (`color-mix(primary, transparent ~84–92%)`); active uses the same/slightly
  stronger wash. Primary filled buttons darken by one step on hover
  (600→700) and another on active (→800).
- **Nav / menu items:** hover/focus background = `surface-200` (light) /
  `surface-800` (dark); active route link gets a primary-tinted treatment.
- **Tabs:** the active tab is marked by a **2px primary-500 bottom border** and a
  shift to the primary text color; hover only nudges the border color.
- **Icons in the header:** white by default, switch to inherit/colored on hover.
- No shrink/scale press effects — state is communicated by color, not transform.

---

## ICONOGRAPHY

Stratusphere UX uses **two icon fonts**, both copied into `fonts/`:

1. **PrimeIcons** (PrimeNG's built-in set) — the default. Class pattern
   `pi pi-{name}` (e.g. `pi pi-chart-line`, `pi pi-search`, `pi pi-cog`,
   `pi pi-refresh`, `pi pi-filter`, `pi pi-sort`, `pi pi-star`, `pi pi-clock`,
   `pi pi-sign-out`, `pi pi-user`, `pi pi-lock`, `pi pi-bell`). Stroke-light,
   outline-style, monochrome — they inherit `currentColor`.
2. **Material Symbols Rounded** (Google) — used where PrimeIcons lacks a glyph.
   Class `material-icons`, glyph by ligature text (e.g. `devices`, `table_view`,
   `summarize`, `pageview`, `dns`, `inventory`, `web`, `disc_full`,
   `remember_me`, `event_note`). Rounded optical style, `OPSZ 24`.

The app maps navigation/inspector concepts to icons in `page-icons.ts`. A custom
Angular `[lwlIcon]` directive picks PrimeIcons when the value starts with `pi `,
otherwise treats it as a Material Symbol ligature.

- **No SVG icon system** beyond the logos. Icons are font glyphs, not inline SVG.
- **No emoji, no Unicode dingbats** used as icons anywhere.
- **Icon sizing:** default `iconSize` is `1.143rem` (~16px); header icons run at
  16px; nav icons ~16–19px.

**Usage in this design system:** PrimeIcons and Material Symbols are loaded from
the local `fonts/` CSS (`fonts/primeicons.css`, `fonts/material-icons.css`). The
PrimeIcons web font file itself is **not** in the codebase mount, so previews and
UI kits load PrimeIcons from its CDN (flagged below). Match the existing sets —
do **not** introduce a third icon family or hand-draw SVG icons.

### Logos & brand marks (`assets/`)
- **`logo-primary.svg`** — flame/droplet glyph (flame in flame-blue `#03a9f4`) +
  "Liquidware" wordmark in gray `#636466`. **For light backgrounds.**
- **`logo-primary-light.svg`** — same lockup with a **white** wordmark. **For dark
  / blue backgrounds** (used in the app header and login). 601×203 viewBox.
- **`logo.svg`** — the standalone **app icon**: a `#0b72ba` circle containing the
  white flame mark. 1920×1920, ideal as favicon/avatar/marque.
- **`lwl-hex.png`** — the pale-blue **hex honeycomb** background texture.
- **`loading-spinner.svg`**, **`hc.svg`**, **`favicon.ico`** — misc app assets.

---

## File index / manifest

Root of this project:
- **`README.md`** — this file: context, content & visual foundations, iconography.
- **`colors_and_type.css`** — all design tokens as CSS variables (color scales,
  GFP semantics, radii, shadows, spacing, type) + semantic type helper classes.
- **`SKILL.md`** — Agent-Skills front-matter wrapper so this system can be used
  as a downloadable skill in Claude Code.
- **`assets/`** — logos (primary, primary-light, icon mark), hex background,
  spinner, favicon.
- **`fonts/`** — Inter variable fonts + CSS, PrimeIcons CSS, Material Symbols
  Rounded font + CSS.
- **`preview/`** — small HTML cards rendered in the Design System tab (color
  scales, GFP, type specimens, spacing, radii, shadows, components, logos).
- **`ui_kits/stratusphere-ux/`** — high-fidelity recreation of the web console
  (`index.html` clickable prototype + JSX components). See its own `README.md`.

### Substitutions & flags
- **PrimeIcons web font** is loaded from CDN in previews/UI kits because the
  `.woff2` is not present in the codebase mount. The CSS class names match the
  product exactly. *(Inter and Material Symbols Rounded fonts ARE bundled
  locally.)* If you can provide `primeicons.woff2`, drop it in `fonts/` and
  repoint `fonts/primeicons.css`.
