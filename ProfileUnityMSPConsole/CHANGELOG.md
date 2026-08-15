# Changelog

## 0.2.0 — unreleased

Surfaces ProfileUnity's own license **Product** name (e.g.
"ProU+FlexApp") everywhere License Mode is already shown — it was
already being collected and stored on every poll, and the Dashboard API
was already returning it, but nothing displayed it.

- **Dashboard**: added a **Product** column right next to License Mode.
  The API (`/api/dashboard`) already returned `licenseProduct` and the
  search box already filtered by it — this was a pure UI gap, and also
  fixes a pre-existing mismatch where the manual described the search
  box as filtering by license product with no column to show what
  matched.
- **Monthly Reports (JSON + PDF, tenant and portfolio)**: added
  `licenseProductAtMonthEnd` — the license product as of the last
  successful collection in the reporting month, using the same "last
  known value this month" convention as `entitledAtMonthEnd`/
  `maximumUsersAtMonthEnd`. This was a genuine gap: report data never
  carried it at all. Shows as a metric tile on the tenant report view, a
  column in the portfolio's per-tenant detail table, and a line in both
  PDF exports (which the portfolio PDF picks up automatically, since it
  reuses the same per-tenant rendering).
- No database migration needed — `license_product` has been a stored
  snapshot column since Phase 3; this only threads an already-collected
  value through to two more places it wasn't reaching yet.
- Verified end-to-end against the built server: a snapshot with a real
  `LicenseProduct` value shows up correctly in the Dashboard column, the
  tenant/portfolio report JSON, and both PDF exports.

## 0.1.0

Phase 1 (Skeleton) of the build plan:

- Repo layout for a Go backend that embeds an Angular frontend via `go:embed`.
- Config loading from environment variables (`internal/config`), with no
  baked-in `localhost` default — a bind address must be set explicitly.
- Single source-of-truth version: root `VERSION` file, synced into the binary
  via `scripts/sync-version.sh` ahead of every build (see README, "Version").
- SQLite storage (`internal/db`) with an embedded, ordered migration runner
  and the first migration (`0001_init.sql`).
- `/healthz` endpoint reporting process status, version, and scheduler state
  (scheduler itself lands in Phase 3).
- Placeholder frontend (`web/dist/index.html`) embedded into the binary so
  the `go:embed` wiring is exercised end to end; the real Angular app lands
  in Phase 4.
- CI workflow (build, vet, test) and a release script skeleton documenting
  the required SBOM → Grype → notices → version ordering (§11.8 of the
  project brief) for later phases to fill in.
- Legal packaging placed at top level from the start: `Spark_License.pdf`
  and a placeholder `bom.cdx.json` / `THIRD-PARTY-NOTICES.txt`, to be
  regenerated as real dependencies and features are added.

No functional license-collection, dashboard, or reporting behavior yet —
this commit is infrastructure only, per the phased build plan in the
project brief (each phase ends in a checkpoint).

Phase 2 (API client) of the build plan:

- `internal/profileunity`: a client for the ProfileUnity API contract in
  the project brief §3, talking only to `/licenseinfo`, `/authenticate`,
  `/api/server/licensing`, and `/api/licenseserver` — no generic
  "call any path" method exists.
- Raw transport types keep each endpoint's exact field spelling (e.g.
  `UsedLicenses` vs. `UsedLicensed`); `flexString` defensively coerces an
  unexpected JSON number/boolean into text instead of crashing decode.
- A `/Date(1234567890)/` legacy ASP.NET date deserializer, with and
  without a trailing timezone offset.
- Success is decided by the envelope's `Type` field, never the HTTP
  status — the console returns HTTP 200 on failure, including
  authentication failure.
- `SupportEnds` is parsed with the explicit US `M/D/YYYY` format only
  (never a locale-dependent parser) and normalized to ISO 8601 on ingest.
- `CollectLicenseInfo` attempts the unauthenticated call first and falls
  back to an authenticated session only if the console demands one and
  credentials were configured, recording which path (`AuthPath`) actually
  produced the result.
- Transport failures are classified into distinct error types
  (unreachable, timeout, TLS, auth rejected, auth required, malformed
  payload) so a failed poll is never confused with a successful zero.
- Unit tests use the §3.2 reference payload verbatim (asserting the
  independently-confirmed 1-of-5 result) plus fixtures for the `Type:
  "error"`/HTTP 200 case, missing/unexpected-type fields, Concurrent
  license mode, used-exceeds-total, an ambiguous US/EU date, non-Latin
  `RegisteredTo` text, an unknown field with a differing `ConsoleVersion`,
  an HTML error page, connection refused, timeout, and TLS failure with
  and without the verification override.

Phase 3 (Collector and scheduler) of the build plan:

- `internal/crypto`: AES-256-GCM encrypt/decrypt for tenant credentials at
  rest, key supplied externally (`PUMC_CREDENTIAL_ENCRYPTION_KEY`, never
  stored alongside the data it protects).
- `internal/tenant`: registered-console CRUD. `Tenant` never carries a
  password, even after creation; only a separate, collector-only
  `GetCredentials` call ever decrypts one. Username/password must be both
  set or both empty, and storing a password without an encryption key
  configured is a hard error, never a silent plaintext write.
- `internal/snapshot`: one row per tenant per collection day
  (`UNIQUE(tenant_id, collection_date)`), upserted so re-running
  collection on the same day updates that row rather than duplicating it.
  A failed poll stores nil license figures — never a zero — plus a
  distinct status and the raw response body.
- `internal/collector`: builds a `profileunity.Client` per tenant from its
  stored config (TLS-verify toggle, optional credentials), retries
  transient failures (unreachable, timeout) with backoff, and classifies
  every other failure (TLS, auth rejected/required, malformed response)
  without retrying it.
- `internal/scheduler`: an in-process ticker (no external cron
  dependency) that collects from every enabled tenant concurrently, capped
  at a configurable concurrency, each bounded by a per-tenant timeout so
  one dead tenant can never stall the run. `CollectNow` is the same code
  path the ticker uses, ready for a future manual "Collect Now" control.
  Reports live status (running/idle, last outcome, tenant/success counts)
  for `/healthz`.
- `internal/db`: SQLite now opens with `busy_timeout` and WAL mode, since
  concurrent tenant polling means concurrent writers; without it,
  concurrent snapshot writes failed outright with `SQLITE_BUSY`.
- New config: `PUMC_COLLECTION_INTERVAL`, `PUMC_COLLECTION_TIMEZONE`,
  `PUMC_COLLECTION_CONCURRENCY`, `PUMC_COLLECTION_TENANT_TIMEOUT`,
  `PUMC_CREDENTIAL_ENCRYPTION_KEY`.
- `cmd/server`: starts the scheduler at boot and stops it on
  SIGINT/SIGTERM with a graceful HTTP shutdown; `/healthz` now reports
  real scheduler state instead of `not_implemented`.

Still no HTTP API or UI for registering tenants or viewing results — that
is Phase 4 (frontend shell) and Phase 5 (dashboard).

Phase 4 (Frontend shell) of the build plan:

- `internal/auth`: bcrypt-hashed operator/viewer accounts (constant-time
  authentication regardless of whether the username exists), server-side
  sessions with independent idle and absolute timeouts, and a CSRF
  double-submit token required (with `X-Requested-With`) on every
  mutating request.
- `internal/tlscert`: generates a self-signed TLS cert/key pair at first
  startup if none is supplied; never overwrites an existing pair. The
  server now serves HTTPS only.
- `internal/legal`: embeds `Spark_License.pdf`, `bom.cdx.json`, and
  `THIRD-PARTY-NOTICES.txt` (synced from the repo root by the new
  `scripts/sync-legal.sh`, same pattern as the version file) and serves
  them at fixed top-level paths.
- `cmd/server`: bootstraps the first operator account from
  `PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD` if none exist; new config for
  session timeouts, bootstrap credentials, and TLS cert/key paths.
- `web/frontend`: a real Angular 18 standalone app — PrimeNG (Aura preset
  re-themed with the Liquidware tokens), Tailwind for layout only,
  Transloco i18n (English + Dutch, runtime-switchable). Login screen,
  authenticated shell (header/nav/language-switcher/sign-out), About
  screen (version, license/SBOM links, required disclaimer text), and
  "Coming Soon" stubs for Dashboard/Tenants/History/Reports. The Go
  server falls back to `index.html` for any path that isn't a real
  static file, so the Angular router's client-side routes survive a hard
  refresh or direct link.
- Fonts (Inter var, Material Symbols Rounded) vendored locally;
  `primeicons` via the npm package. Confirmed zero `unpkg.com` (or any
  other CDN) references in the built artifact.
- Switched from the `-lts` PrimeNG package variant to the plain
  MIT-licensed release after discovering the `-lts` variant displays a
  customer-visible "invalid license" banner without a matching license
  token — the supplied commercial key's format doesn't match what that
  variant's own license verification expects. See README.md "Status" for
  the full note and the open questions flagged for the reviewer.
- Known-CVE flag (not fixed this phase; see README.md "Status"): Angular
  18.2.x carries several real, fixed-in-later-majors high-severity XSS
  advisories. Deferred to the Phase 8 compliance pass per the required
  SBOM/Grype ordering.
- CI/Makefile/release script now build the frontend before any Go step
  that touches `internal/httpapi`/`internal/legal`/`web`, and target
  `./cmd/... ./internal/... ./web` rather than `./...` — `node_modules`
  ships a stray vendored `.go` file with no `go.mod` to bound it out.

Tenant CRUD still has no HTTP API or screen — that's Phase 5 alongside
the dashboard.

Phase 5 (Dashboard and tenant management) of the build plan:

- `internal/dashboard`: pure `Compute`/`BuildAll` functions deriving
  usage (good/fair/poor), expiry (ok/expiring soon/expired), and data
  (ok/stale/failing/never collected) status per tenant from its
  registration plus its latest and latest-successful snapshots — decoupled
  from HTTP/UI so a future report export reuses the exact same logic.
- `internal/snapshot`: added `GetLatest`, `GetLatestSuccess`,
  `LatestForAllTenants`, `LatestSuccessForAllTenants` (two queries total
  for the whole dashboard, not one per tenant).
- `internal/collector`: added `TestConnection` — one attempt, no
  retries, classifying the outcome precisely (unauthenticated/
  authenticated success, TLS failure, timeout, auth rejected/required,
  malformed response, unreachable) for the tenant form's "Test
  Connection" button.
- `internal/httpapi`: tenant CRUD (`/api/tenants[/{id}]`), connectivity
  test (`/api/tenants/test`), and dashboard (`/api/dashboard`) endpoints,
  all session-gated; mutating routes and the test endpoint are also
  CSRF-gated, since test-connection makes outbound requests to whatever
  host:port is in the request body and must never be reachable
  anonymously.
- `web/frontend`: real Tenants (CRUD table, add/edit dialog, Test
  Connection, delete confirmation) and Dashboard (sortable/filterable
  table, `StatusBadgeComponent`) screens replacing their "Coming Soon"
  stubs. Data-trust states are deliberately never rendered with the
  Good/Fair/Poor palette — distinct neutral gray + icon, so a console
  that's unreachable never looks merely "poor".
- Visually verified end to end (Playwright) with seeded tenants covering
  every state: healthy, near-limit + expiring-soon, at-limit + expired +
  stale, collection-failing-with-an-older-success, and never-collected.

History and Reports still show "Coming Soon" — Phases 6 and 7.

Phase 6 (History and graphs) of the build plan:

- `internal/dashboard/history.go`: pure `DetectEntitlementChanges`
  (walks a tenant's successful snapshots in date order, reporting every
  point where `TotalLicenses` changes from the last successful reading —
  failed/missing days never count, and the first successful point never
  counts either) and `BuildPortfolioHistory` (groups every tenant's
  successful snapshots by collection date, summing used/entitled across
  the portfolio per day).
- `internal/snapshot`: added `ListAllSuccess`, the raw material for the
  portfolio-wide history view, fetched in one query rather than one per
  tenant.
- `internal/httpapi`: `GET /api/tenants/{id}/history` and
  `GET /api/history/portfolio`, both session-gated, returning every
  day's point (including failed/unreachable days with their status and
  error message, not just successes) plus detected entitlement changes.
- `web/frontend`: a History screen with a per-tenant/portfolio toggle
  and a Chart.js line chart (`p-chart`, `chart.js@4`) with
  `spanGaps: false` so a failed or missing collection day renders as a
  visible break — never an interpolated line and never a drop to zero.
  `buildContinuousSeries` (pure function) expands the sparse API
  response into one point per calendar day, filling gaps with `null` so
  Chart.js actually breaks the line there. The chart is only ever
  created via `@if` once genuinely on screen, avoiding the classic
  hidden-canvas (`display:none` → `width=0,height=0`) pitfall. Entitlement
  changes render as a plain text list under the chart, not chart
  annotations — a deliberate scope simplification.
- Fixed a real bug found during visual verification: the mode toggle's
  labels were computed once from `TranslocoService.translate()` in a
  class field, before Transloco's async-loaded translation files
  arrived, so they showed the raw `history.perTenant`/`history.portfolio`
  keys instead of translated text. The first fix attempt (a getter
  recomputing the array every change-detection cycle) traded that bug
  for a worse one: PrimeNG's `p-selectButton` saw a new `[options]`
  array reference on every cycle, which triggered an internal state
  update → `markForCheck` → another cycle, forever — an infinite loop
  that hung the page. Fixed properly by keeping `modes` a stable,
  referentially-unchanging array of translation *keys* and rendering the
  translated label via an `ng-template pTemplate="item"` with the
  `transloco` pipe, the same pattern the rest of the app uses for
  dynamic translated text.
- Visually verified (Playwright) with 20 days of seeded snapshot data
  covering a true multi-day gap, a failed collection day, and an
  entitlement change — confirmed both gaps render as visible breaks (not
  interpolated, not zero), the entitlement change shows as a visible
  step in the "Entitled" line plus the text list below it, and the mode
  toggle renders correctly in both English and Dutch.

Reports still shows "Coming Soon" — Phase 7.

Phase 7 (Monthly reporting) of the build plan:

- `internal/dashboard/report.go`: `BuildTenantMonthlyReport` and
  `BuildPortfolioMonthlyReport` pure functions, computing peak/average
  usage, entitlement at month end, entitlement changes (reusing
  `DetectEntitlementChanges` from Phase 6), and an explicit
  `CoverageStatus` (complete/partial/none) so a report says up front how
  much of the month's data can be trusted, rather than presenting
  numbers as if every day had been collected.
- `internal/snapshot`: added `ListByTenantInRange`/`ListAllInRange`
  (inclusive date-range queries across any status), the raw material a
  report needs to count failed/missing days, not just successes.
- `internal/httpapi`: `GET /api/tenants/{id}/reports/monthly` and
  `GET /api/reports/portfolio/monthly` (JSON), plus `.../monthly.pdf`
  variants rendering the same figures as a downloadable PDF via the new
  `github.com/go-pdf/fpdf` dependency (MIT, pure Go, no CGO). Tenant
  display names are sanitized before use in a `Content-Disposition`
  filename.
- `web/frontend`: a Reports screen (tenant/portfolio toggle, month/year
  picker, on-screen summary, "Download PDF" link) replacing the
  "Coming Soon" stub. `StatusBadgeComponent` gained a `coverage` kind,
  rendered with the same neutral (non-GFP) styling as data-trust states
  — a partially-collected month must never look merely "poor". The mode
  toggle reuses the Phase 6 fix's pattern from the start (a stable
  options array + item template + `transloco` pipe), avoiding a repeat
  of that phase's infinite change-detection-loop bug.
- Visually verified (Playwright) with a full seeded August across two
  tenants covering a multi-day gap, a single failed day, a three-day
  outage, and an entitlement change; downloaded PDFs' content streams
  were decompressed and checked to confirm they match the on-screen
  figures exactly. Verified in Dutch as well.

Phase 8 (Alerting and full compliance pass) of the build plan — the last
phase in the build plan:

- `internal/dashboard/alert.go`: pure `DetectAlerts` function flagging a
  tenant when usage is at/over its license limit, support has expired or
  is expiring soon, or data can't currently be trusted; a tenant can
  carry more than one reason. `GET /api/alerts` and a header bell-icon
  popover (`AlertBellComponent`, refetched on every navigation) surface
  this in-app only, per the scope confirmed with the user — no email/
  SMTP, no new outbound dependency.
- Upgraded Angular/PrimeNG 18 → 21 (one major at a time via `ng update`)
  to resolve every CVE flagged since Phase 5, fixing real breaking
  changes at each step (`pButton`/`[label]` → `<p-button>`, `p-message`'s
  `[text]` → content projection, stricter selector casing). Stopped at
  21 rather than 22: PrimeNG 22 enforces a license-key check on the
  previously-free base package (a customer-visible "Invalid PrimeUI
  License" banner appeared in visual verification — the same failure
  mode already rejected once for the `-lts` variant in Phase 4). `npm
  audit --omit=dev` now reports zero vulnerabilities in the shipped
  frontend tree (down from 55, 1 critical). Also found and avoided a
  second, unrelated licensing trap: the latest `@primeuix/themes`
  transitively pulls in `@primeui/license-manager` (revenue/developer-
  count eligibility gate) via a newer `@primeuix/styled`; pinned
  `@primeuix/themes@2.0.3` (matching what PrimeNG 21 itself expects)
  avoids it entirely.
- Unicode/i18n compliance sweep (project brief §11) found and fixed
  three real bugs: the PDF report generator wrote raw UTF-8 through
  fpdf's Latin-1-only default font (mojibake for non-Latin-1 tenant
  names) — fixed by embedding DejaVu Sans via `AddUTF8FontFromBytes`
  (confirmed with the user before adding the ~1.4MB font asset);
  `<html lang>` never updated on a runtime language switch; Angular's
  `number` pipe used the app's static en-US `LOCALE_ID` regardless of
  the active language, so Dutch sessions still saw "4.5" instead of
  "4,5" (replaced with `Intl.NumberFormat` keyed off Transloco's active
  language). Also fixed a byte-index (not rune-index) string truncation
  in `MalformedPayloadError.Error()` that could split a multi-byte
  UTF-8 rune.
- `bom.cdx.json` is now a real, generated CycloneDX 1.6 SBOM (127
  components — 10 Go modules, 117 production npm packages — merged by
  `scripts/merge-sbom.py`; see `scripts/generate-sbom.sh`), and
  `THIRD-PARTY-NOTICES.txt` is generated from it (`scripts/
  generate-notices.py`), both replacing their Phase 1 placeholders. Go
  dependencies (`golang.org/x/crypto` and others) bumped to the newest
  versions still compatible with the `go 1.24.7` pin. A live Grype/
  govulncheck scan could not be completed in this build environment —
  both fetch their vulnerability feed from hosts (`vuln.go.dev`,
  `github.com/anchore/grype` releases) unreachable through this
  sandbox's network policy; documented in README as a required
  follow-up before treating this as a real release.

Post-Phase-8 follow-up: CI compliance enforcement, closing out the
deferred CVE gate.

- Added a `compliance` job to CI (`.github/workflows/
  profileunity-msp-console-ci.yml`) running `npm audit --omit=dev
  --audit-level=high` and `govulncheck` as hard gates on every push, then
  regenerating the SBOM/notices and diffing them against what's checked
  in (component lists only, so the timestamp/serialNumber `generate-sbom.sh`
  stamps fresh each run doesn't produce a false failure).
- That job's first real run (with actual network access to
  `vuln.go.dev`, unlike the original build sandbox) found 15 genuine
  CVEs, all in the Go standard library, all fixed only in Go 1.25.x —
  this project was still pinned to `go 1.24.7`. Bumped to `go 1.25.13`
  (`go.mod` and CI's `go-version` pin) and re-ran `go get -u`/
  `go mod tidy`, which also picked up newer dependency versions
  (`golang.org/x/crypto`, `modernc.org/sqlite`, and others) that had
  previously been capped at go1.24-compatible releases. Regenerated
  `bom.cdx.json`/`THIRD-PARTY-NOTICES.txt` to match.

Post-Phase-8 follow-up: release packaging now includes the user manual.

- Added `scripts/render-manual-pdf.sh`, converting `docs/MANUAL.md` to a
  standalone PDF (pandoc to a self-contained HTML file with embedded
  print CSS, then a headless Chromium/Chrome binary prints that to PDF —
  no LaTeX toolchain, no external CDN references).
- `scripts/release.sh`'s packaging stage now bundles `MANUAL.pdf`,
  `CHANGELOG.md`, `bom.cdx.json`, `THIRD-PARTY-NOTICES.txt`, and
  `Spark_License.pdf` alongside the binary in every release zip.
  `README.md` (written for someone building from source, not running the
  console) is no longer included — `MANUAL.pdf` replaces it as the
  release's operator-facing document.

Post-Phase-8 follow-up: every build now also produces a Windows binary,
branded with the Liquidware icon.

- Added `scripts/build-windows.sh`, cross-compiling a `windows/amd64`
  build (`CGO_ENABLED=0` — `modernc.org/sqlite` is pure Go, so no cgo
  toolchain is needed) and embedding a Windows version resource via
  `goversioninfo`: the app icon (reusing `web/frontend/public/
  favicon.ico`, the same mark already shown on the web UI's browser
  tab), `CompanyName: Liquidware`, and the product name/version, so
  Explorer/the taskbar/Properties dialog show a genuine branded
  product instead of a bare Go binary.
- `.github/workflows/profileunity-msp-console-ci.yml`'s
  `build-and-test` job now also runs this cross-compile on every
  push/PR and uploads the result as a workflow artifact, so a
  Windows-specific build break is caught the same way a Linux one
  would be.
- `scripts/release.sh` now produces two zips per release
  (`-linux-amd64`, `-windows-amd64`) instead of one, each with that
  platform's binary alongside the same manual/changelog/SBOM/notices/
  license bundle as before.

Post-Phase-8 follow-up: manual "Collect Now" (project brief §7.2), closing
out a deferred feature.

- `internal/scheduler.Scheduler.CollectNow` already existed (it's the
  same code path the ticker in `Run` uses) but was never reachable from
  outside the process. Added `POST /api/collect/run`
  (`internal/httpapi/collection.go`), session- and CSRF-protected like
  every other mutating endpoint, which calls it and returns the
  resulting scheduler status.
- Added a **Collect Now** button to the Dashboard toolbar
  (`TenantsService.collectNow()`), which blocks until the run finishes
  and then refreshes the table — so a newly-added tenant doesn't have
  to sit at **Never Collected** until the next scheduled tick.

Post-Phase-8 follow-up: the server now actually reads `.env`.

- README and `.env.example` always described "copy `.env.example` to
  `.env` and fill it in" as the setup workflow, but nothing ever read
  that file — only real exported environment variables worked, which
  meant setting five-plus `PUMC_*` variables by hand in the shell
  before every start (easy to forget one, and easy to lose track of
  what's set once the terminal closes).
- Added `internal/dotenv` (a small in-house `KEY=VALUE` parser, not a
  dependency) and wired `dotenv.Load(".env")` into `cmd/server`'s
  startup, before `config.Load()` reads the environment. A real
  environment variable for a given key always wins over the file, so
  a deployment that already exports `PUMC_*` (e.g. a Windows Service
  definition) needs no changes.
- Verified end to end: started the binary with `env -i` (a completely
  empty environment) and only a `.env` file present, and it came up
  correctly; then repeated it with one real environment variable set
  alongside a conflicting `.env` entry and confirmed the real one won.
- `scripts/release.sh`'s zips now include `.env.example` alongside the
  binary — the `.env` feature above is only actually usable out of the
  box if the release bundle ships the template to copy and fill in.

Post-Phase-8 follow-up: nav polish, an About popup instead of a dead-end
page, and a developer credit.

- The left nav's plain text links now render as icon + label buttons
  (`shell.component.css`), with a hover state and a stronger highlight
  for the active route.
- The in-app "About" link used to navigate to a standalone route with
  no header/nav and no way back except the browser's own back button.
  It's now a `p-dialog` popover over whatever screen you were on (the
  same `AboutComponent`, reused via a new `showHeading` input so its
  title isn't shown twice against the dialog's own header) — closing
  it just closes the dialog, nothing to navigate back from. The
  pre-login `/about` route (linked from the login screen, where there's
  no console screen behind it to pop up over) is unchanged.
- Added a developer credit ("Developed by Laurens van Duijn") to the
  About content.

Post-Phase-8 follow-up: branded the monthly report PDF export.

- The PDF export (`internal/httpapi/report_pdf.go`, `github.com/go-pdf/
  fpdf`) was plain black-on-white text with no logo or color — unlike
  every other surface of the console. Added a repeating brand-blue
  header band (matching the web console header's `--header-bg` exactly)
  with the Liquidware wordmark, colored section headings, and a footer
  with page numbers, via fpdf's `SetHeaderFunc`/`SetFooterFunc` so it's
  identical on every page of a multi-page portfolio report, not just
  the first.
- The logo is a PNG (`internal/httpapi/images/liquidware-logo-white.png`,
  rendered from the same `logo-primary-light.svg` the web UI uses) since
  fpdf embeds raster images only, not SVG.
- Fixed a real bug found while building this: fpdf's `AddPage` restores
  the font/color active before it was called once `SetHeaderFunc`'s
  callback returns, but not the text cursor position — without an
  explicit reset at the end of the header callback, body content
  started writing from wherever the header's own title text left the
  cursor (inside the band), overlapping it, on every page.
- Verified against real generated PDFs, not just the code: rendered a
  single-tenant report and a 6-page portfolio report (25+ tenants) via
  `pdftoppm`, and visually confirmed the header/footer repeat correctly
  on an interior page reached via fpdf's automatic page-break (not a
  manual `AddPage` call) and on the final page, with `{nb}` correctly
  resolving to the true page count.

Post-Phase-8 follow-up: operators can change their own password.

- There was no way to change a password after the initial
  `PUMC_BOOTSTRAP_ADMIN_PASSWORD` short of deleting the database and
  losing every tenant and snapshot. Added `UserRepo.ChangePassword`
  (`internal/auth/user.go`), which verifies the current password
  against the stored bcrypt hash before replacing it (same 12-character
  minimum `CreateUser` enforces), and `POST /api/auth/change-password`
  (session- and CSRF-protected like every other mutating endpoint).
- Added a "Change Password" dialog (`shared/change-password-form.
  component.ts`), opened by clicking the username in the header (now a
  button with a key icon), reusing the same `p-dialog` pattern as
  About/Collect Now.
- Still no administrator password-reset path — an operator can only
  ever change their own password by proving they know the current one.
  Documented the forgot-password situation (and its only real fallback,
  which loses all data) in `docs/MANUAL.md`'s Troubleshooting section.
- Verified end to end against the real running app, not just unit
  tests: logged in via curl, changed the password, confirmed the old
  one now fails login and the new one succeeds, and confirmed the
  dialog itself renders correctly (screenshotted the real authenticated
  app, same cookie-injection technique used for earlier UI changes).

Post-Phase-8 follow-up: login screen brought to the actual brand spec.

- The login screen had drifted from the Liquidware design system: it was
  a plain `p-card` with stock PrimeNG inputs on the app's ordinary light
  background, rather than the hex-honeycomb backdrop, frosted-glass
  panel, and pill-shaped inputs the design-system reference
  (`docs/design-system-reference/liquidware-ui/ui_kits/stratusphere-ux/
  kit.css`, `.login`/`.login-form`/`.login-logo`/`.login-field`) actually
  specifies. Rewrote `login.component.html`/`.ts` and added a new
  `login.component.css` matching those rules: full-viewport hex
  background over the dark primary color, a blurred/translucent form
  panel, a large centered wordmark, and pill inputs with a lead icon
  (user/lock) and, for the password field, a trailing show/hide toggle.
- Removed the login screen's "About" link. (The in-app nav's About
  dialog, and the standalone `/about` route it and the dialog share, are
  unaffected and still reachable — only the login page stopped linking
  to it.)
- Added the missing `login.togglePassword` key to both `en.json` and
  `nl.json` for the new toggle button's `aria-label`, and confirmed by
  diffing both files' key sets that EN/NL remain fully aligned (118 keys
  each, none only in one file).
- Verified visually against the real running app (headless Chromium
  screenshot of `/login`, no auth needed since it's pre-login): hex
  background, frosted panel, and pill inputs all render as specified,
  the logo fits its box without distortion despite being a wide
  wordmark rather than the squarer mark the spec's own CSS assumed, and
  no About link remains on the page.
- Added the product name ("ProfileUnity MSP Licensing Console", the
  existing `app.title` i18n key) under the logo, so the login screen
  identifies the app rather than showing only the Liquidware wordmark.

Post-Phase-8 follow-up: the portfolio report can now email itself.

- Added automatic monthly emailing of the portfolio PDF report — the
  same PDF the Reports screen's "Download PDF" link already produces,
  sent on a configurable day of the month (default the 1st) for the
  month that just ended, with no external cron job or scheduler needed.
  Off by default: setting `PUMC_SMTP_HOST` is the single switch that
  turns it on, alongside the now-required `PUMC_SMTP_FROM` and
  `PUMC_REPORT_RECIPIENTS` (new `internal/config` fields/env vars —
  `PUMC_SMTP_PORT`/`USERNAME`/`PASSWORD`/`SECURITY` and
  `PUMC_REPORT_EMAIL_DAY` round it out with sane defaults).
- New `internal/mailer` package talks SMTP directly via the standard
  library (`net/smtp`, `crypto/tls` — starttls/tls/none) and hand-builds
  the multipart/mixed MIME message, rather than pulling in a third-party
  mail dependency (this project has none beyond fpdf/uuid/x-crypto/
  sqlite; `go.mod` didn't need to change for this).
- New `internal/reportmail` package runs an in-process ticker (the same
  approach `internal/scheduler` already uses for collection) that checks
  hourly whether today is on or after the configured send day and, if
  so, builds and emails that month's portfolio report — "on or after",
  not "exactly on", so a server that's down on the configured day still
  catches up once it's back, rather than silently skipping that month.
  A new `report_emails` table (migration `0004`, `UNIQUE(year, month)`)
  is the idempotency guard, mirroring the `snapshots` table's
  `UNIQUE(tenant_id, collection_date)` pattern from migration `0002` —
  only a successful send gets a row, so a failed attempt is retried on
  the next tick instead of being permanently (and silently) suppressed.
- Refactored the portfolio-report-loading logic that lived inline in
  `internal/httpapi/reports.go` out into
  `dashboard.LoadPortfolioMonthlyReport`, and moved `report_pdf.go` (plus
  its embedded DejaVu Sans fonts and Liquidware logo PNG) out of
  `internal/httpapi` into a new `internal/reportpdf` package with its
  render functions exported — both the HTTP report handlers and the new
  scheduler now share the exact same report-building and PDF-rendering
  code, so the emailed PDF and the one you'd download from the UI can
  never drift apart.
- Verified end to end against the real running binary, not just unit
  tests (which include a real fake-SMTP-server test in both
  `internal/mailer` and `internal/reportmail`, exercising the actual
  EHLO/MAIL/RCPT/DATA/QUIT wire protocol): ran the server with SMTP
  pointed at a local debug mail server, confirmed it computed the
  correct target month, attempted STARTTLS against a server that didn't
  support it (a real connection-level failure, not a mock), then
  re-verified with plain SMTP that the previous month's portfolio PDF
  arrived as a real email with the report text as the body and a valid,
  correctly branded PDF attachment.

Post-Phase-8 follow-up: a Settings screen for everything that isn't
needed to boot the process in the first place.

- Added a **Settings** screen (new nav entry) covering SMTP/report-email,
  collection interval/timezone/concurrency/tenant-timeout, operator
  session idle/absolute timeouts, and the active TLS certificate.
  Bootstrap-only settings — listen address, DB driver/DSN, the
  credential encryption key, the initial admin account — stay env-var
  only, exactly as before; everything else is now editable from the UI,
  takes effect **immediately** (no restart), and is persisted so it
  survives one.
- New `internal/settings` package: a `runtime_settings` singleton row
  (migration `0005`), seeded once from the `PUMC_*` environment variables
  on a fresh database and the sole source of truth after that. Env vars
  for these fields are now seed values only — see the rewritten
  `.env.example` and `docs/MANUAL.md`'s "Configuration reference" for
  exactly which settings stay env-only versus which are seed-only.
- Made `scheduler.Scheduler`, `reportmail.Scheduler`, and
  `auth.SessionRepo` accept live updates via new `SetTunables`/
  `SetConfig`/`SetTimeouts` methods instead of fixed-at-construction
  fields. `scheduler.Scheduler.Run`'s wait loop needed a wake-up channel,
  not just an atomic value swap, to make a shortened interval apply
  immediately rather than only once whatever wait was already in flight
  happened to expire — caught by a test that deliberately started with
  an hour-long interval and asserted a second collection run within
  seconds of calling `SetTunables`.
- New `internal/tlscert.Holder`: an atomically-swappable
  `tls.Config.GetCertificate` backing, so an operator-uploaded
  certificate hot-swaps into the running HTTPS listener with zero
  downtime — `cmd/server/main.go` now builds an explicit `tls.Config`
  and calls `ListenAndServeTLS("", "")` instead of handing file paths to
  the standard library. Verified with a real TLS handshake test
  (`internal/tlscert`) that dials the listener before and after a swap
  and confirms the presented certificate actually changed.
- New settings endpoints: `GET`/`PUT /api/settings`,
  `POST /api/settings/tls-cert` (validates the pair with
  `tls.X509KeyPair` before accepting anything), and
  `POST /api/settings/test-email` (sends using whatever's currently
  typed into the form, not the saved settings, the same way tenant Test
  Connection tests unsaved values).
- Verified end to end against the real running binary: logged in,
  shortened the collection interval via the API and watched
  `/healthz`'s `lastRunAtUtc` advance on the new cadence, shrank the
  session idle timeout and confirmed an already-authenticated session
  got invalidated on its very next request, and uploaded a fresh
  self-signed certificate and confirmed — via a real `openssl s_client`
  handshake against the live listener — that its fingerprint changed
  with no restart in between.

Post-Phase-8 follow-up: fixed field overlap on Settings and a
horizontal scrollbar on Add Tenant.

- Root cause: every `<p-inputnumber>` in the app had `styleClass="w-full"`
  (sizing the component's outer wrapper) but not `inputStyleClass="w-full"`
  — PrimeNG's Aura `p-inputnumber` doesn't propagate that width onto its
  own inner native `<input>`, which then rendered at its intrinsic
  ~244px regardless of the wrapper/label around it. On the Settings
  screen this meant the number inputs visually overran their columns
  and overlapped neighboring fields; in the Add Tenant dialog the Port
  field's real width (244px) inside its 8rem-wide label forced the
  whole form 130px past the dialog's content area, producing a
  horizontal scrollbar regardless of window size. Added the missing
  `inputStyleClass` (matching the existing `styleClass`) to every
  `p-inputnumber` in `settings.component.html`, `tenant-form.component.
  html`, and `reports.component.html`.
- Also removed the tenant form's hard `min-width: 28rem` (which could
  itself exceed the dialog's content width after padding) and widened
  the Add Tenant dialog from `32rem` to `36rem` for breathing room, and
  added `flex-wrap` to the fixed-multi-column rows on both screens so
  they degrade to stacked rows on a narrow window instead of overflowing.
- Verified with real DOM measurements (`scrollWidth`/`clientWidth` via
  CDP), not just a screenshot: before the fix the Add Tenant dialog's
  content had `scrollWidth 514` vs `clientWidth 502`; after, they're
  exactly equal (502/502, 364/364) at a realistic 1024px window width,
  and the Settings screen's number fields no longer spill into their
  neighbors.

Post-Phase-8 follow-up: tightened the Add Tenant dialog and turned
Timezone into a real dropdown.

- Reduced the Add Tenant dialog from `36rem` to `28rem` and let the
  tenant form fill it (`width: 100%` instead of a fixed `26rem`) —
  the dialog was comfortably wider than its content needed, leaving a
  lot of unused space around short fields like Display Name and Port.
- The Settings screen's Timezone field was a free-text input (just a
  placeholder hint of "UTC") — an operator could type anything,
  including a typo IANA name that would only surface as an error on
  save. Replaced it with a searchable `p-select` populated from
  `Intl.supportedValuesOf('timeZone')` (419 real IANA zones in a
  Chromium-based browser), with a short hardcoded fallback list for
  browsers old enough not to support that API.
- Verified end to end: rebuilt, logged in, opened the Timezone dropdown
  and confirmed all 419 zones list correctly with "UTC" pre-selected
  and highlighted, and reconfirmed via real DOM measurements
  (`scrollWidth`/`clientWidth`, exactly equal) that the smaller Add
  Tenant dialog still has zero overflow.

Post-Phase-8 follow-up: fixed the Timezone dropdown discarding a typed
selection on Save.

- Root cause: the Timezone `p-select`'s filter box lets an operator type
  to narrow ~419 IANA zones, but PrimeNG only commits a selection on an
  explicit click or Enter-on-highlighted-row — `autoOptionFocus` (which
  auto-highlights the first filtered match so Enter works) defaults to
  `false`. An operator who typed a zone name and went straight to Save
  (never clicking the row or pressing Enter) had their typed filter text
  silently discarded; the control's real value hadn't changed, so it
  reappeared as "UTC" (or whatever it was before) after saving — reported
  as "the timezone is not saved... changes back to utc".
- Fixed by (1) setting `[autoOptionFocus]="true"` so Enter now selects the
  top filtered match, and (2) tracking the filter query via `(onFilter)`
  and, on `(onHide)`, auto-committing it if it uniquely narrows the list
  to exactly one zone — so typing a specific zone and clicking Save
  directly (without an explicit click/Enter) now works too. An ambiguous
  filter (matching more than one zone) is left alone rather than guessing.
- Verified via a real headless-browser session driving the actual
  `p-select` DOM: typing "Los_Angeles" into the filter and clicking Save
  with no intervening click/Enter now persists `America/Los_Angeles`
  (previously silently kept the prior value); typing an ambiguous query
  like "America" correctly leaves the value unchanged rather than
  picking one of many matches.

Post-Phase-8 follow-up: "Entitled At Month End" was reporting the wrong
number; added "Maximum Users" alongside it.

- `TotalLicenses` from `/licenseinfo` is a license's Maximum Users
  ceiling, not what's actually entitled/in-use — "Entitled At Month
  End" was sourced from it, so the report showed the license's cap
  rather than actual usage. `UsedLicenses` is what ProfileUnity itself
  reports as in use.
- "Entitled At Month End" (tenant and portfolio-total) is now sourced
  from `UsedLicenses` on the last successful collection of the month,
  matching what an MSP actually bills against.
- Added a new "Maximum Users" figure (tenant `MaximumUsersAtMonthEnd`,
  portfolio-total `TotalMaximumUsersAtMonthEnd`), sourced from
  `TotalLicenses` on the last successful collection of the month — the
  number "Entitled At Month End" used to (incorrectly) show — so that
  ceiling is still visible, just correctly labeled. Added to the JSON
  API, the PDF report, and the Reports screen (metric tiles and the
  per-tenant portfolio table) in both languages.
- Updated `dashboard` unit tests for the corrected values.

Post-Phase-8 follow-up: fixed "Could not save settings" when picking a
non-UTC Timezone on Windows.

- Root cause: `Settings.Validate()` calls `time.LoadLocation` to check
  the Timezone field, which resolves IANA zone names by reading
  zoneinfo files from the OS (`/usr/share/zoneinfo` on Linux, or a
  `ZONEINFO`/`%GOROOT%` lookup on Windows) — a plain Windows deployment
  has none of these, so anything other than `"UTC"` failed to resolve
  and the save was rejected with a 400, surfaced to the operator as
  "Could not save settings." This was invisible before the Timezone
  field became a real dropdown (today's earlier follow-ups): as a
  free-text input defaulting to the `UTC` placeholder, nobody had
  reason to type a different zone, so the gap never got exercised on a
  real Windows install.
- Fixed by blank-importing `time/tzdata` in `cmd/server/main.go`, which
  embeds the full IANA zone database directly in the compiled binary —
  `time.LoadLocation` now works the same on Windows, a minimal Linux
  container, or anywhere else, independent of what the host OS has
  installed.

Post-Phase-8 follow-up: added file-based logging, with verbosity tied
to the development/production environment switch.

- New `internal/logging` package builds a single `log/slog` logger for
  the whole process, writing every line to both stderr (unchanged
  console behavior) and a log file next to the binary/database
  (`PUMC_LOG_FILE`, default `./profileunity-msp-console.log`). Not
  rotated in this pass — a known limitation, not a blocker.
- `PUMC_LOG_LEVEL` (debug/info/warn/error — already existed as a
  config field but was previously dead, unread anywhere) now actually
  controls verbosity, and defaults from `PUMC_ENVIRONMENT` when unset:
  `debug` in development, `info` otherwise — answering "should the
  normal/debug switch match the dev/production switch" with "yes, by
  default, but still explicitly overridable" (e.g. to turn on debug
  logging temporarily on a production install while troubleshooting).
- Replaced every `log.Print*` call across `cmd/server/main.go`,
  `internal/scheduler`, and `internal/reportmail` with the shared
  `slog` logger, and added new debug-only lines at points that were
  previously silent (per-tenant collection attempt/outcome, report-mail
  check decisions) — this is what makes the debug tier meaningfully
  more verbose than normal, not just a level number.
- Verified end to end: development mode shows debug lines in both
  stderr and the file; production mode suppresses them but keeps info
  lines in both; an explicit `PUMC_LOG_LEVEL=debug` re-enables debug
  logging even in production; `PUMC_LOG_FILE` relocates the file.

Post-Phase-8 follow-up: a built-in default admin account, so the
console works with zero bootstrap configuration.

- Previously, if `PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD` were both
  left unset, no operator account was created at all and the server
  logged "nobody can sign in yet." Now, leaving both unset creates a
  built-in `LiquidwareMSP`/`LiquidwareMSP` operator account instead —
  the console is usable immediately after install with no `.env`
  editing required, and the operator changes the password afterward
  from the existing self-service change-password screen.
- **Security tradeoff, made deliberately**: this is a fixed, predictable
  default credential, not a randomly-generated one-time password — the
  simpler, more discoverable option was chosen explicitly over
  generating and logging a random password on first boot. The server
  logs a prominent warning (visible in both the console and the log
  file added earlier today) every time the built-in default account is
  created, telling the operator to change its password immediately.
  Setting `PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD` explicitly still
  works exactly as before and skips the built-in default entirely.
- Setting exactly one of the two env vars (as opposed to leaving both
  unset) is still a configuration error, unchanged from before — this
  only changes the both-unset case.

Post-Phase-8 follow-up: two more zero-config defaults — an
auto-generated credential encryption key, and a default listen address.

- `PUMC_CREDENTIAL_ENCRYPTION_KEY` (encrypts stored tenant credentials
  at rest) previously had to be generated by hand (`openssl rand
  -base64 32`) and pasted into `.env`, or tenant credentials simply
  couldn't be stored at all. Left unset, a key is now generated on
  first boot and persisted to `PUMC_CREDENTIAL_ENCRYPTION_KEY_FILE`
  (default `./credential-encryption.key`, `0600` permissions) —
  `internal/crypto.EnsureKey` mirrors `internal/tlscert.EnsureSelfSigned`'s
  exact generate-once-then-reuse-forever pattern: subsequent boots reuse
  the same file rather than regenerating it, since a changed key would
  make every previously stored tenant credential permanently
  undecryptable. The server logs a prominent warning to back up that
  file whenever it's generated. Setting `PUMC_CREDENTIAL_ENCRYPTION_KEY`
  explicitly still bypasses file generation entirely, unchanged.
- `PUMC_HTTP_ADDR` now defaults to `0.0.0.0:8443` (all interfaces) when
  unset, reversing an earlier deliberate decision (no baked-in listen
  address, "must be bound to an address you chose on purpose") for the
  same zero-config reasoning as the rest of today's defaults. Still
  fully overridable by setting it explicitly.

Post-Phase-8 follow-up: a Windows tray launcher, so double-clicking the
console gives a real app instead of a bare console window.

- Windows can't make one `.exe` behave differently for a double-click
  vs. a terminal launch, so this is now two binaries in the Windows zip:
  `profileunity-msp-console.exe` (new) is a small system-tray launcher —
  Start/Stop/Restart buttons, a status indicator, Liquidware branding,
  and a "Show Log" window that live-tails the server's log file. It
  collapses to a tray icon while running (click the [x] to hide, not
  quit; Exit from the tray menu actually quits) and starts the server
  automatically on launch. `profileunity-msp-console-server.exe` is the
  actual headless server, byte-for-byte the same `cmd/server` as before
  under a new name — running it directly (PowerShell, a Scheduled Task,
  a Windows Service) is completely unaffected.
- New `cmd/tray` package, pure-Go (`github.com/lxn/walk`, Win32 bindings
  over `syscall` — no cgo, so this still cross-compiles from a plain
  Linux build machine with zero C toolchain, exactly like the existing
  server build). The launcher spawns the server with
  `CREATE_NEW_PROCESS_GROUP|CREATE_NO_WINDOW` (no stray console window)
  and stops it by sending `CTRL_BREAK_EVENT` to trigger the server's
  existing graceful shutdown, falling back to a hard kill after 10s if
  that doesn't work — Stop always eventually works even if graceful
  signal delivery has some environment-specific quirk.
- `scripts/build-windows.sh` now builds and version-stamps both
  executables; `scripts/release.sh`'s Windows zip includes both. Linux
  packaging is unaffected (single headless binary, no GUI).
- `docs/MANUAL.md` documents the two Windows executables and what each
  is for.
- **Known limitation**: this was built and cross-compiled from a Linux
  sandbox that cannot run a live Windows GUI — the interactive behavior
  (buttons, tray icon, live log viewer, the graceful-stop signal) needs a
  real smoke test on Windows before being fully verified.

Post-Phase-8 follow-up: the tray launcher silently did nothing on a
real Windows test — made startup failures visible instead of invisible.

- Reported: double-clicking `profileunity-msp-console.exe` produced no
  window, no tray icon, nothing. Root cause: the launcher is built
  `-ldflags "-H=windowsgui"` (no console, by design — that's what makes
  it a real double-click app), but its startup error handling still did
  `fmt.Println(...)` before exiting — with no console attached, that
  output goes nowhere visible, so *any* single startup failure looked
  exactly like "nothing happens," with no way to tell what actually
  broke.
- Fixed: every startup failure path (including an actual Go panic, now
  caught via `recover()`) shows a native Windows message box with the
  failing step and the underlying error instead of printing to a
  nonexistent console. This doesn't fix a specific bug — it turns the
  next failure, if any, from invisible into diagnosable.
- Also embedded the Windows application manifest
  `github.com/lxn/walk`'s own examples ship (comctl32 v6 + per-monitor
  DPI awareness) into the tray exe via `goversioninfo`'s `ManifestPath`
  field, matching the library's documented convention — this is cosmetic
  (themed controls, correct DPI scaling), not the fix for the reported
  symptom, but free and correct to add regardless.
- Root cause of the original report is still unconfirmed — this sandbox
  cannot run a Windows GUI to reproduce it, so what the new message box
  (if it appears) actually says is what will pin it down.

Post-Phase-8 follow-up: the tray launcher now starts and works on a
real Windows machine, confirming the previous fix — but its window
opened nearly full-screen with large gaps between the title, status,
and button row. `buildMainWindow()` only set a *minimum* window size
(`SetMinMaxSize`), never an explicit initial one, so it opened at
`walk.MainWindow`'s (large) default and `VBoxLayout` spread the unused
space evenly across every child, since none of them had a stretch
factor set. Fixed by explicitly sizing the window
(`mw.SetSize(walk.Size{Width: 420, Height: 210})`) to fit its actual
content.

Post-Phase-8 follow-up: the tray launcher now shows the actual
Liquidware wordmark and a clickable link straight to the console.

- The window previously only had the hex-drop mark as its title-bar
  icon and a plain text title — added the real Liquidware wordmark
  (rasterized from the same `web/frontend/src/assets/images/logo-primary.svg`
  the login screen uses, embedded as a PNG) above the title text.
- Added a clickable link (`https://<host>:<port>`, resolved from
  `PUMC_HTTP_ADDR`/`.env` the same way the log viewer resolves its log
  file) that opens the console directly in the default browser —
  `PUMC_HTTP_ADDR`'s default host, `0.0.0.0` (all interfaces), isn't
  itself browsable, so that (and an empty host) is shown/opened as
  `localhost` instead, which is always correct from the same machine
  the launcher runs on.

Post-Phase-8 follow-up: the tray launcher's wordmark logo showed up
inside a visible white box instead of blending with the window
background. The PNG had been rasterized without asking the renderer to
omit its background, so it carried an opaque white fill baked into
every pixel rather than real transparency — `walk.NewBitmapFromImage`
and the toolkit's `AlphaBlend`-based drawing both already handle
per-pixel alpha correctly, so this was purely an asset problem.
Re-rasterized `cmd/tray/logo.png` with a transparent background
preserved, so the wordmark now blends into the window like the title
text next to it.
- New shared `config.DefaultHTTPAddr` constant (mirroring
  `DefaultLogFile`) so this doesn't duplicate the literal.
- Window height bumped slightly (260px) to fit the new logo and link
  rows without cramping.

Post-Phase-8 follow-up: Settings screen SMTP fixes.

- Saving SMTP settings silently appeared to do nothing while "Send Test
  Email" worked with the identical values. Root cause: once an SMTP
  host is set, `internal/settings.Settings.Validate()` requires a From
  address and at least one report recipient before it will persist
  anything, rejecting the PUT with a 400 and its reason in the response
  body — but the Settings screen's `save()` discarded that body and
  always showed the generic "Could not save settings.", so an operator
  who filled in just the relay fields had no way to know why the save
  was rejected (the DB row was untouched, which is why the fields
  looked "reverted" after a reload). The test-email endpoint has no
  such validation, which is why it always worked regardless.
  - The Settings form now carries the same cross-field rule
    client-side: the Save button disables and an inline hint appears
    under the From Address / Recipients fields the moment an SMTP host
    is set without them, instead of a rejected round trip.
  - On a 400 response, the real backend message is now shown verbatim
    in the error banner instead of the generic fallback.
- Added a Port dropdown (25 / 465 / 587) in place of the free-text
  number field, and picking a Security mode now sets the conventional
  port for it (`starttls` → 587, `tls` → 465, `none` → 25) — the port
  stays editable afterward for a relay using something else, and a
  previously-saved non-standard port is kept as an extra option rather
  than silently replaced.

Post-Phase-8 follow-up: added a **Send Now** button to the Monthly
Portfolio Report card on the Settings screen, so an operator can trigger
the report email immediately instead of waiting for the scheduled send
day — useful for verifying SMTP/recipients actually work, or sending
early.

- `reportmail.Scheduler.SendNow` reuses the automatic scheduler's own
  `send`/`previousMonth` logic to build and email last calendar month's
  report on demand, bypassing the day-of-month gate. It still marks the
  month as sent (same as the automatic path), so the scheduler won't
  re-send it later once the configured day arrives.
- New `POST /api/settings/send-report-now` endpoint (`SendReportNowHandler`
  in `internal/httpapi/settings.go`), wired the same way as every other
  mutating settings route (session + CSRF).
- Since this emails every configured recipient immediately — unlike Send
  Test Email, which only ever goes to an address just typed into the
  form — the button asks for confirmation first, the same treatment this
  screen's tenant-deletion confirm dialog already established as this
  codebase's pattern for "an action with a real effect outside this
  screen."
- Verified end-to-end against a real local SMTP listener: configuring
  SMTP + recipients, saving, then Send Now actually delivers an email
  with the expected subject and PDF attachment for last month, and the
  button correctly surfaces a clear error (without touching the form's
  Save state) when SMTP isn't configured yet.

Post-Phase-8 follow-up: added a **Users** screen (new left-nav entry) so
an operator can add and remove other console login accounts from the
UI — until now the only way in was the single bootstrapped account plus
self-service Change Password, with no way to add a second account at
all.

- New `auth.UserRepo.List`/`Delete` methods and a new
  `auth.SessionRepo.RevokeAllForUser`, backing three new endpoints
  (`GET/POST /api/users`, `DELETE /api/users/{id}`,
  `internal/httpapi/users.go`) wired with the same session+CSRF
  protection as every other mutating route.
- Every account created here is a plain operator — there's no role
  picker, since nothing in this app enforces any difference between the
  existing operator/viewer roles today; adding one without any actual
  effect would only be misleading.
- Two hard server-side guards on delete, neither of which existed
  before this: an operator can't delete their own account, and can't
  delete the last remaining account — either would make the console
  impossible to sign into, with no way back short of the database-reset
  procedure in the manual's Troubleshooting section. The frontend also
  hides the delete button on the signed-in operator's own row, but the
  server-side checks are what actually matter.
- Deleting an account now immediately invalidates any of its active
  sessions. `sessions.user_id` declares `ON DELETE CASCADE`, but this
  database never enables `PRAGMA foreign_keys`, so that constraint was
  never actually enforced — `RevokeAllForUser` closes that gap
  explicitly rather than relying on it.
- `/api/auth/me` (and login) now also returns the signed-in account's
  `id`, needed by the Users screen to identify "this is you."
- Verified end-to-end against the built server: created a second
  account, signed in as it to confirm it actually works, confirmed only
  the other account's row (never the signed-in operator's own) has a
  delete button, deleted it, and confirmed both that its row disappears
  and that a direct API call attempting to delete your own account (and,
  via a dedicated unit test, the last remaining account) is rejected.

No further phases remain beyond Phase 8.
