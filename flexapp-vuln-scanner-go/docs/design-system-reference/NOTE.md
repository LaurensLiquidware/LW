# Design system reference bundle — not shipped

This directory is the raw `Spark_Liquidware_style_guide_baseline.zip` extract, kept
for reference while building the Phase 4 (Frontend shell) work. Nothing under here
is built, imported, or embedded by the Go binary — it exists so the design tokens,
copy conventions, and component previews survive between build phases.

**Known problems in this bundle, to resolve when it is actually vendored into the
frontend build (Phase 4), per the project brief §8.1:**

- `liquidware-ui/fonts/primeicons-cdn.css` imports `primeicons` from `unpkg.com`.
  Use `liquidware-ui/fonts/primeicons.css` instead (class defs only) and vendor the
  `primeicons.woff2` file from the `primeicons` npm package. Delete/never reference
  `primeicons-cdn.css` in anything that ships.
- The top-level `support.js` (preview harness, sibling to this note, not copied in)
  loads React/ReactDOM/Babel from `unpkg.com`. It is preview tooling only and must
  never be reachable from the shipped frontend.

Both are tracked as open items for Phase 4 (see main `README.md` "Known unknowns /
deferred work"). Do not copy `fonts/primeicons-cdn.css` or any `unpkg.com` reference
into `web/frontend/` when the Angular app is scaffolded.
