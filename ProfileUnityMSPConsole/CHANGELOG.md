# Changelog

## 0.1.0 — unreleased

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

No further phases remain beyond Phase 8.
