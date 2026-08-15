# ProfileUnity MSP Licensing Console

**IMPORTANT: READ BEFORE DOWNLOADING OR USING.** This is a Liquidware
**Sparks Tool** — a community/field-contributed utility, **not a Liquidware
commercial product**. It is provided outside Liquidware's standard product
development lifecycle, **"AS IS" with no warranty, support, or maintenance**,
and used at your own risk. Read `Spark_License.pdf` (Liquidware Sparks Tool
License and Disclaimer) before use.

This tool also ships `bom.cdx.json` — a **CycloneDX 1.6** JSON inventory of
every third-party component it uses, so your security team can review it
against your own policy. It sits next to the license PDF for that reason;
see `THIRD-PARTY-NOTICES.txt` for the accompanying license texts. **Both
files are still placeholders** — a real, growing set of Go and npm
dependencies exists already, but per the project brief §11.8 the SBOM is
only ever regenerated for real as part of the compliance pass in Phase 8,
after CVE remediation, so it describes exactly what ships rather than an
intermediate state. See "Compliance" below.

## What this is

A multi-tenant web console that lets a Managed Service Provider see
ProfileUnity license consumption across all their registered customer
consoles, track it over time, and produce monthly reports. See the project
brief for the full functional and compliance spec; this README tracks
build-phase status and the things a reader needs before touching the code.

**Looking to install and use the console rather than build it?** See
[`docs/MANUAL.md`](docs/MANUAL.md) for the operator manual — installation,
configuration reference, and how to use every screen.

## Status: Phase 8 — Alerting and full compliance pass

Phases 1–3 (skeleton, the ProfileUnity API client, and the
tenant/snapshot/collector/scheduler backend) are done — see git history
and CHANGELOG.md for what each added. Phase 4 adds a real UI and the
console's own authentication:

- **Backend auth** (`internal/auth`, `internal/tlscert`): bcrypt-hashed
  operator/viewer accounts, server-side sessions with idle and absolute
  timeouts, a CSRF double-submit token required on every mutating
  request, and a self-signed TLS certificate generated at first startup
  (the server only ever serves HTTPS). The first operator account is
  bootstrapped from `PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD` if none
  exist yet, defaulting to a built-in `LiquidwareMSP`/`LiquidwareMSP`
  account when both are left unset.
- **`internal/legal`**: embeds `Spark_License.pdf`, `bom.cdx.json`, and
  `THIRD-PARTY-NOTICES.txt` into the binary (synced from the repo root by
  `scripts/sync-legal.sh`, same pattern as the version file) and serves
  them at fixed top-level paths, so the About screen can link to them
  directly regardless of the process's working directory.
- **Angular frontend** (`web/frontend/`): standalone components, PrimeNG
  (Aura preset, re-themed with the Liquidware tokens in
  `src/app/theme/liquidware-preset.ts` — see "Design system" below),
  Tailwind for layout/spacing only, and Transloco i18n (English + Dutch,
  runtime-switchable, no reload). A login screen, an authenticated shell
  (header, nav, language switcher, sign-out), an About screen (version +
  license/SBOM links + the required disclaimer text), and "Coming Soon"
  stubs for Dashboard/Tenants/History/Reports, which land in later
  phases. The Angular router owns client-side routes; the Go server
  falls back to `index.html` for any path that isn't a real static file.
- Session handling mirrors the reference project's pattern: a
  root-provided `SessionService`, an HTTP interceptor that resets state
  and redirects to `/login?expired=1` on any unexpected 401, and a CSRF
  interceptor that echoes the CSRF cookie back as a header on every
  mutating request.

Phase 5 adds tenant management and the dashboard itself:

- **`internal/dashboard`**: pure functions (`Compute`) deriving a
  tenant's `UsageStatus` (good/fair/poor, from utilization vs.
  `NearLimitThreshold`), `ExpiryStatus` (ok/expiring soon/expired, from
  `SupportEnds` vs. `ExpiringSoonDays`), and `DataStatus`
  (ok/stale/failing/never collected, from the most recent collection
  attempt vs. `StaleAfterDays`) — decoupled from HTTP/UI so a future PDF
  or spreadsheet export (§7.5) computes identical numbers. `BuildAll`
  aggregates every tenant in two queries total, not one per tenant.
- **Tenant CRUD API** (`GET/POST/PUT/DELETE /api/tenants[/{id}]`) and a
  **connectivity test** endpoint (`POST /api/tenants/test`) that reports
  precisely what happened — unauthenticated success, authenticated
  success, TLS failure, timeout, auth rejected/required, malformed
  response, or unreachable — never a boolean (project brief §7.1). Both
  require a session; the mutating routes and the test endpoint also
  require CSRF, since an unauthenticated test-connection call would
  otherwise be an open SSRF/port-scanning proxy through this server.
- **`GET /api/dashboard`**: every tenant's computed status, for the
  frontend's at-a-glance view.
- **Frontend**: a real Tenants screen (table, add/edit dialog covering
  every §7.1 field, Test Connection with the precise outcome displayed
  inline, delete confirmation) and a real Dashboard screen (sortable/
  filterable table, a shared `StatusBadgeComponent` rendering the
  Good/Fair/Poor language for usage/expiry — but data-trust states
  (stale/failing/never collected) always render in a neutral gray with a
  distinct icon, never GFP red/yellow/green, so an unreachable console
  can never look merely "poor" — see project brief §10).

Visually verified end to end with seeded data covering every state
(healthy, near-limit, at-limit-and-expired-and-stale, collection-failing-
with-an-older-success, and never-collected) — screenshots taken during
development, not committed.

Phase 6 adds the History screen (project brief §7.4):

- **`internal/dashboard/history.go`**: two more pure functions.
  `DetectEntitlementChanges` walks a tenant's successful snapshots in
  date order and reports every point where `TotalLicenses` differs from
  the last successful reading — failed/missing days never count as a
  change, and the first successful point never counts either (there is
  nothing to compare it to). `BuildPortfolioHistory` groups every
  tenant's successful snapshots by collection date and sums used/
  entitled across the portfolio for each day.
- **`GET /api/tenants/{id}/history`** and **`GET /api/history/portfolio`**:
  return the raw per-day points (including failed/unreachable days, with
  their status and error message, not just successes) plus the detected
  entitlement changes, so the frontend can render gaps rather than
  interpolate or zero-fill them.
- **Frontend History screen**: a per-tenant/portfolio toggle, a
  Chart.js line chart (via PrimeNG's `p-chart`) with `spanGaps: false`
  so a failed or missing collection day renders as a visible break in
  the line, never a flat interpolation and never a drop to zero — this
  is the single most safety-critical rendering detail in the whole
  screen, since the opposite (silently bridging a gap) would misrepresent
  unknown data as known. Entitlement changes are shown as a plain text
  list under the chart (date + from → to), not as chart annotations —
  simpler to implement correctly and just as legible; annotation plugins
  were not pulled in for this. The chart is only ever created via an
  Angular `@if` once real data has arrived and the element is genuinely
  on screen — never hidden behind `display:none` — since a canvas
  measured while hidden gets `width=0,height=0` and Chart.js never
  recovers from that even after the element becomes visible later.
  `buildContinuousSeries` (a pure function, `core/history-series.ts`)
  expands the sparse day-by-day API response into one entry per calendar
  day between the earliest and latest date, filling missing days with
  `null` so the gap actually renders.
- **Scope decision**: `PortfolioPoint.tenantsRegistered` is the *current*
  count of registered tenants, not a historical count as of that date —
  the schema doesn't track when a tenant was added/removed, so a true
  historical denominator isn't available yet. Documented here rather
  than silently assumed; revisit if portfolio history needs to explain
  "why did total entitlement drop" distinctly from "a tenant left".

Phase 7 adds monthly reporting (project brief §7.5):

- **`internal/dashboard/report.go`**: `BuildTenantMonthlyReport` and
  `BuildPortfolioMonthlyReport`, pure functions reusing
  `DetectEntitlementChanges` from Phase 6 so a report's entitlement-change
  list is always the same computation the History screen already showed —
  never a second, possibly-divergent implementation. Each report carries
  an explicit `CoverageStatus` (`complete`/`partial`/`none`) rather than
  presenting numbers as if every day had been collected: a month with
  gaps or failures says so up front, and — like the dashboard's
  stale/failing/never-collected states — is deliberately never rendered
  with the Good/Fair/Poor palette, since a partially-collected month
  means "we don't fully know", not "bad".
- **`internal/snapshot`**: added `ListByTenantInRange`/`ListAllInRange`
  (inclusive date-range queries, any status) — a report needs the
  failed/missing days too, to compute coverage, not just the successes
  `ListAllSuccess` already covered.
- **`GET /api/tenants/{id}/reports/monthly`** and
  **`GET /api/reports/portfolio/monthly`** (JSON), plus
  **`.../monthly.pdf`** variants that render the identical figures as a
  downloadable PDF via `github.com/go-pdf/fpdf` (MIT-licensed, pure Go,
  no CGO — a new dependency, confirmed with the user before adding it
  per the working agreement). All four require a session; a tenant's
  display name is sanitized before it reaches the PDF's
  `Content-Disposition` filename, since that header value would
  otherwise reflect attacker-controlled input straight into the
  response.
- **Frontend Reports screen**: a per-tenant/portfolio toggle (reusing the
  Phase 6 fix's pattern — a referentially-stable options array with an
  `ng-template pTemplate="item"` + the `transloco` pipe, never a getter
  rebuilding the array every change-detection cycle) plus a month/year
  picker, an on-screen summary using the same neutral-styled
  `StatusBadgeComponent` the dashboard uses for data-trust states (now
  extended with a `coverage` kind), and a plain `<a href>` "Download PDF"
  link — the PDF endpoints need only the session cookie, so this follows
  the same pattern the About screen already uses for
  `Spark_License.pdf`, no blob-fetch/CSP complexity required.
- Visually verified (Playwright) with a full seeded August across two
  tenants — a multi-day gap, a single failed day, a three-day outage, and
  an entitlement change — confirming the on-screen numbers, the neutral
  "Partial" coverage badges, and the downloaded PDFs (both per-tenant and
  portfolio, verified by decompressing their content streams) all agree
  with each other and with the seeded data. Also verified in Dutch.

Phase 8 is the last phase in the build plan: alerting, plus the full
compliance pass §11 requires before a real release.

**Alerting** (project brief §7.6, scoped with the user before building):
in-app only — no email/SMTP, no new outbound dependency. `internal/
dashboard/alert.go`'s pure `DetectAlerts` function flags a tenant when
usage is at/over its license limit, support has expired or is expiring
soon, or its data can't currently be trusted (failing/stale/never
collected) — a tenant can carry more than one reason at once, and all of
them are surfaced. `GET /api/alerts` serves the list; the frontend shows
a bell icon with a count badge in the header (`AlertBellComponent`),
refetched on every navigation, opening a popover listing each alertable
tenant and its reasons. `StatusBadgeComponent` gained no new visual
language for this — alerts reuse the dashboard's existing Good/Fair/Poor
and neutral data-trust colors, never inventing a fourth palette.

**Angular/PrimeNG upgraded 18 → 21, resolving every previously-flagged
CVE.** The Angular ≤18.2.x XSS/XSRF advisories flagged at the end of
Phase 5 required a real major-version upgrade, not a patch — the fix
ranges in the relevant GHSAs extend as high as "≤19.2.25", so Angular 19
alone wasn't enough either. Went through `ng update` one major at a time
(18→19→20→21), fixing real breaking changes at each step (PrimeNG's
`pButton`/`[label]` API moved to a dedicated `<p-button>` component,
`p-message`'s `[text]` input became content projection, several
component selectors' casing became stricter). Stopped at Angular 21/
PrimeNG 21.1.9 rather than continuing to 22: **PrimeNG 22 enforces a
client-side license-key check on the previously-free base package** — a
customer-visible "Invalid PrimeUI License" banner appeared immediately
during visual verification, the same failure mode this project already
rejected once for the `-lts` variant back in Phase 4. PrimeNG 21.1.9 has
no such gate (confirmed both by absence of the license-check code in the
installed package and by a clean visual pass) and is not itself the
subject of any of the flagged CVEs. `npm audit --omit=dev` now reports
**zero** vulnerabilities of any severity in the shipped frontend
dependency tree (down from 55 findings, 1 critical, pre-upgrade); the
handful of remaining `npm audit` findings are all `devDependencies`
(build tooling — webpack, vite, tar — never shipped in the compiled
bundle).

**A second, unrelated PrimeNG licensing trap, found and avoided while
upgrading:** `@primeng/themes` (used for the Aura preset since Phase 4)
is deprecated in favor of `@primeuix/themes`; the latest `@primeuix/
themes@3.0.0` transitively pulls in `@primeui/license-manager` via a
newer `@primeuix/styled@1.0.0` — a package whose own LICENSE.md imposes
revenue/developer-count/funding eligibility thresholds for free use.
Pinning `@primeuix/themes@2.0.3` (the version PrimeNG 21 itself expects,
via `@primeuix/styled@^0.7.4`) avoids that dependency entirely — verified
with `npm ls @primeui/license-manager` reporting nothing installed. The
plain `primeng` package's own license (`LICENSE.md` inside the package)
remains plain MIT with no eligibility conditions, same as every prior
phase's decision.

**Unicode/i18n compliance sweep (§11) found and fixed three real bugs**,
none present in earlier phases' visual verification because none of
them involve non-Latin-1 text or a live language switch:
- The PDF report generator wrote raw UTF-8 bytes into fpdf's default
  Helvetica font, which only supports single-byte Latin-1 — any tenant
  name with Cyrillic, Greek, Vietnamese, or CJK characters would render
  as mojibake in a downloaded report, not just missing glyphs. Fixed by
  embedding DejaVu Sans (regular + bold, Bitstream Vera-derived license,
  ~1.4MB, confirmed with the user before adding it) via `fpdf`'s
  `AddUTF8FontFromBytes` — correctly renders Latin Extended, Cyrillic,
  and Greek; CJK/Arabic/Hebrew still don't render (would need a
  much-larger CJK-capable font) and are a documented limitation.
- `<html lang>` was hardcoded to `"en"` in `index.html` and never
  updated on a runtime language switch — wrong for screen readers,
  browser spell-check, and CSS `:lang()` selectors. `AppComponent` now
  syncs `document.documentElement.lang` from Transloco's active
  language on load and on every switch.
- Angular's `number`/`DecimalPipe` formats using the app's static
  `LOCALE_ID` (fixed to en-US at bootstrap), so a Dutch-language session
  still showed "4.5" instead of "4,5" for averages on the Reports
  screen. Replaced with an `Intl.NumberFormat` call keyed off Transloco's
  active language (`core/locale-number.ts`), consistent with the
  month-name formatting the History/Reports screens already used.
- `MalformedPayloadError.Error()` (backend) truncated an API error
  snippet with a byte-index slice, which can split a multi-byte UTF-8
  rune in half if the ProfileUnity console's error page isn't pure
  ASCII — fixed to truncate by rune.

**SBOM and CVE gate.** `bom.cdx.json` is now a real, generated CycloneDX
1.6 SBOM (127 components: 10 Go modules via `cyclonedx-gomod`, 117
production npm packages via `@cyclonedx/cyclonedx-npm`, merged by
`scripts/merge-sbom.py` — see `scripts/generate-sbom.sh`) — no longer the
Phase 1 placeholder, and `THIRD-PARTY-NOTICES.txt` is likewise generated
from it (`scripts/generate-notices.py`). `npm audit --omit=dev` reports
zero vulnerabilities in the shipped frontend tree.

A live Grype/govulncheck CVE scan could not be run inside the sandbox
this project was originally built in — both tools fetch their
vulnerability feed from `vuln.go.dev`, which that sandbox's network
policy didn't allow (everything else, `proxy.golang.org` and
`registry.npmjs.org` included, worked fine). CI's new `compliance` job
(see below) runs `govulncheck` for real on every push, and its first run
found something real: **15 known CVEs, all in the Go standard library
itself** (`crypto/tls`, `crypto/x509`, `net/http`, `net/url`, `net`,
`os`, `encoding/asn1`, `net/textproto`), every one fixed only in Go
1.25.x — this project was pinned to `go 1.24.7` at the time. Bumped the
toolchain to `go 1.25.13` (`go.mod`'s `go` directive, plus CI's
`go-version` pin) and re-ran `go get -u`/`go mod tidy` now that the newer
Go unblocks dependency versions that previously required it (`golang.org/
x/crypto` v0.46.0 → v0.55.0, `modernc.org/sqlite` v1.34.4 → v1.56.0,
among others) — closing out every one of those 15 findings along with
whatever the dependency bumps themselves fixed.

**CI now enforces all of this on every push**, not just documents it: a
second `compliance` job runs `npm audit --omit=dev --audit-level=high`
and `govulncheck` as hard gates, then regenerates the SBOM and notices
and diffs them against what's checked in — so `bom.cdx.json` can never
silently drift from the actual dependency tree without CI catching it.
Grype itself is still not wired into CI: its installer fetches release
binaries from GitHub releases rather than a package registry, which
didn't work from the original sandbox and wasn't re-attempted once
`govulncheck` (functionally equivalent for this project's purposes) was
confirmed working on GitHub-hosted runners.

## Critical constraint: this app owns the time series

ProfileUnity stores no license-usage history — the API returns a
point-in-time count only. Consequences that shape every later phase:

- Day one of collection is day one of the data. There is no backfill.
- A missed collection is permanent data loss. The scheduler (Phase 3) is a
  load-bearing feature, not plumbing.
- **A failed poll is not zero usage.** An unreachable console must be
  stored and rendered as *unknown*, distinct from a genuine zero. This is
  the single most important correctness rule in the project.

## Known unknowns — do not assume

Documented rather than guessed, per the ProfileUnity API contract this
tool depends on:

- **Concurrent-license behavior** is untested; the reference environment
  is Named User. Surface license mode prominently; do not present
  Concurrent counts with the same confidence as Named User counts.
- **What `UsedLicenses` actually counts** is undocumented upstream. This
  tool never recomputes it — it reports exactly what the API returns.
- **Cross-version behavior** beyond ProfileUnity 6.9.5.9678 is unverified.
  `ConsoleVersion` is recorded with every snapshot; unexpected/missing
  fields must not crash collection.
- **Whether `/licenseinfo` stays unauthenticated** is an open question (it
  is an unauthenticated info-disclosure, reported upstream). The collector
  is designed to attempt unauthenticated first, fall back to an
  authenticated session, and record which path each collection used.
- **`LastKnownRunningLocal` semantics** are unconfirmed; the collector
  prefers the UTC heartbeat field.

## Design system

`web/frontend/src/styles/tokens.css` and `fonts.css` are the Liquidware
design tokens and fonts, copied verbatim from the style-guide bundle (see
"Design system reference" below) — colors and type sizes are not invented
or approximated. `web/frontend/src/app/theme/liquidware-preset.ts`
re-themes PrimeNG's Aura preset with the same primary/surface values, so
PrimeNG's own generated `--p-primary-*`/`--p-surface-*` variables match
the hand-written ones exactly. Fonts (Inter var, Material Symbols
Rounded) are vendored into `src/assets/fonts/`; `primeicons` is the npm
package, imported directly in `styles.scss` — neither is loaded from a
CDN. `tailwind.config.js` disables Tailwind's preflight reset, since
PrimeNG and the design tokens own base element styling; Tailwind is used
utility-first for layout and spacing only.

## Design system reference

`docs/design-system-reference/` holds the raw Liquidware style-guide
bundle the frontend above was built from. It is reference material only —
nothing under it is built or embedded by the shipped binary. It contains
two known CDN references (`unpkg.com`, in `primeicons-cdn.css` and the
preview harness `support.js`) that were **not** carried into the vendored
frontend; see `docs/design-system-reference/NOTE.md` and confirm for
yourself with `grep -r unpkg web/dist` after a build — it should find
nothing.

## Version

The version shown to the user, logged at startup, in the SBOM `metadata`,
and in the release artifact filename all come from one place: the root
`VERSION` file. Because `go:embed` cannot read outside its own package
directory, `scripts/sync-version.sh` copies `VERSION` into
`internal/version/VERSION_EMBED` (generated, git-ignored) before every
build. `VERSION_EMBED` does not exist in a fresh checkout, so `go build`
run directly fails loudly with a missing-file error instead of silently
shipping a stale or wrong version — always build through `make build` (or
run the sync script first) rather than calling `go build` by hand. See
`CHANGELOG.md` for what changed in each version.

## Building

```
scripts/sync-version.sh
scripts/sync-legal.sh
(cd web/frontend && npm ci && npm run build)
go build -o profileunity-msp-console ./cmd/server
```

or just `make build`. The frontend build is not optional: `internal/httpapi`
and `internal/legal` embed its output and the legal files via `go:embed`,
so `go build`/`go vet`/`go test` on those packages (and anything that
imports them, including `cmd/server`) fail outright without it — there is
no fallback content. `make test`/`make build`/`make run` all run the full
sequence for you. Go commands in CI and `scripts/release.sh` target
`./cmd/... ./internal/... ./web` rather than `./...`, because
`web/frontend/node_modules` ships at least one vendored `.go` file (from
an npm package) with no `go.mod` of its own to keep `./...` from trying
to build it.

## Configuration

Copy `.env.example` to `.env` (in the same directory the binary is run
from) and fill in values, or set the equivalent environment variables
directly — a real environment variable always wins over a `.env` entry
for the same key, so this composes with a deployment that already
exports `PUMC_*` itself. `internal/dotenv` loads it (a small in-house
parser, not a dependency — `KEY=VALUE` per line, `#` comments, optional
quotes, nothing fancier); a missing `.env` file is not an error, since
it's optional. `PUMC_HTTP_ADDR` defaults to `0.0.0.0:8443` (all
interfaces) if left unset — set it explicitly to bind to a specific
interface/port instead. See `.env.example` for the full list.

The server always serves HTTPS. If `PUMC_TLS_CERT_FILE`/`PUMC_TLS_KEY_FILE`
don't both already exist, a self-signed pair is generated there at first
startup (browsers will warn about it — replace it with a CA-signed pair
for anything beyond local/lab use). Likewise, if `PUMC_CREDENTIAL_ENCRYPTION_KEY`
is left unset, a key is generated on first startup and saved to
`PUMC_CREDENTIAL_ENCRYPTION_KEY_FILE` (default `./credential-encryption.key`)
— **back up that file**; losing or replacing it makes every previously
stored tenant credential permanently undecryptable. On first startup
(only ever when no operator accounts exist yet), a
`LiquidwareMSP`/`LiquidwareMSP` operator account is created automatically
if `PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD` are left unset — **change
that password from the account/change-password screen as soon as you
sign in**, since it's a fixed, publicly-known default, not a secret. Set
both env vars to use your own username/password from the first run
instead.

Logs go to both stderr and a file next to the binary (`PUMC_LOG_FILE`,
default `./profileunity-msp-console.log`; the file is appended to, not
rotated). Verbosity (`PUMC_LOG_LEVEL`: `debug`/`info`/`warn`/`error`)
defaults from `PUMC_ENVIRONMENT` — `debug` in development, `info`
otherwise — and can be set explicitly to override that default in either
environment, e.g. to turn on debug logging temporarily in a production
install while troubleshooting.

## Compliance

This project is built against the Sparks Tool Project Review Checklist
from day one (encoding, regional date/number formats, no undisclosed
external references, CycloneDX SBOM, zero Critical/High CVEs via Grype,
visible version, and license/SBOM packaging). The full compliance pass —
including SBOM regeneration and the Grype scan — happens as its own build
phase, per the checklist's own required ordering: clear CVEs, regenerate
the SBOM, sync the version, then package the SBOM next to the license PDF.
Audits are done read-only first, with a written summary of proposed
changes confirmed before anything is edited.

`scripts/release.sh` packages every release as two zips — one per
platform (`-linux-amd64`, `-windows-amd64`) — each containing that
platform's binary, `docs/MANUAL.md` rendered to `MANUAL.pdf` (via
`scripts/render-manual-pdf.sh` — pandoc to a self-contained HTML file,
then a headless Chromium prints that to PDF, avoiding a full LaTeX
toolchain for one document), `.env.example` (see "Configuration" above —
the binary reads `.env` from its own working directory, so this is what
an operator actually edits to get running), `CHANGELOG.md`,
`bom.cdx.json`, `THIRD-PARTY-NOTICES.txt`, and `Spark_License.pdf` —
everything an operator or a compliance reviewer needs, without also
needing this repository's build-status README.

The Windows binary is built by `scripts/build-windows.sh`
(`GOOS=windows GOARCH=amd64`, no cgo needed since `modernc.org/sqlite`
is pure Go), which also embeds a Windows version resource via
`goversioninfo` — the same `favicon.ico` already used for the web UI's
browser tab (so the `.exe` and the site show the same Liquidware mark
in Explorer/the taskbar), plus `CompanyName: Liquidware` and the
product name/version, so Explorer's Properties dialog identifies it
correctly instead of showing a bare Go binary. CI's `build-and-test`
job builds this on every push/PR too (uploaded as a workflow artifact)
so a Windows-specific regression fails the same way a Linux one would.

## Reference project

Architectural patterns carried over from `ProUVisualizer-Go` (proxy
whitelisting, CSRF/session handling, normalize-at-the-edge, pure business
logic, root-provided shared frontend services, reactive query params,
version-sync-as-a-build-step, hidden-canvas measurement) are documented
inline where each is used, not copied wholesale — this is a multi-user
server, not a local single-operator tool, so binding to `127.0.0.1` only
is explicitly **not** carried over.
