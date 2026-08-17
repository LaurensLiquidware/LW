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
see `THIRD-PARTY-NOTICES.txt` for the accompanying license texts. Both are
regenerated as part of every release's compliance pass (SBOM generation,
then CVE remediation, then the SBOM/notices regeneration that ships), so
they always describe exactly what's in that release. See "Compliance"
below.

## What this is

A multi-tenant web console that lets a Managed Service Provider see
ProfileUnity license consumption across all their registered customer
consoles, track it over time, and produce monthly reports. See the project
brief for the full functional and compliance spec; this README covers the
things a reader needs before touching the code.

**Looking to install and use the console rather than build it?** See
[`docs/MANUAL.md`](docs/MANUAL.md) for the operator manual — installation,
configuration reference, and how to use every screen.

## Status

All planned build phases are complete — this is a finished, maintained
tool, not a work in progress. See `CHANGELOG.md` for what changed
release to release, and [`docs/BUILD_HISTORY.md`](docs/BUILD_HISTORY.md)
for the original phase-by-phase build narrative (API discovery, the
collector/scheduler, auth and the Angular frontend, dashboard/history/
reports, alerting, and the compliance pass), if that context is useful.

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

## Demo data

Dropping a `demo.db` file next to the real database (same directory as
`PUMC_DB_DSN`) makes a fresh install come up populated with ten fictional
MSP tenants and six months of daily license history, instead of an empty
one — useful for demos, screenshots, UI development, and onboarding
without pointing at a real customer environment or waiting six months for
a collector to build history.

**Generating it:**

```
go run ./cmd/gendemodb --out ./demo.db
```

Flags: `--out` (default `./demo.db`), `--seed` (fixed default, override for
a different but still-reproducible run), `--tenants` (default/max 10 — the
roster is a fixed set of named storylines, not randomly generated
entries), `--months` (default 6), `--end-date` (default today, `YYYY-MM-DD`
— history is generated *relative to this date*, so a regenerated file is
always "the last N months").

**Placing it:** put the generated file in the same directory as the real
database (next to wherever `PUMC_DB_DSN` points, `./profileunity-msp-console.db`
by default) and restart the server. It's detected automatically — no
config change needed. A startup log line makes it unmissable when demo
mode is active. On Windows, the tray launcher (`profileunity-msp-console.exe`)
detects it too: the first time it notices a `demo.db` for an install with
no port already configured, it seeds that install to listen on **8444**
instead of the normal **8443** — so a demo copy and a production copy can
run side by side out of the box (see "Starting on Windows" in
`docs/MANUAL.md` for the **Change Port** button, which still works
normally afterward) — and its window title, tray tooltip, and dialogs all
read "... — Demo Mode" for as long as that install has a demo.db.

**Disabling it** without removing the file: set `PUMC_DEMO_MODE=off`. The
server then always uses the real database regardless of whether `demo.db`
is present.

**Removing it:** delete `demo.db` (and any `demo.db-wal`/`demo.db-shm`
files that may have appeared alongside it during use — sqlite's WAL mode
creates these while a database is open). The install then behaves exactly
as it did before demo.db was ever added.

**What demo mode changes:** the background collection scheduler is
disabled entirely, and the "Collect Now"/"Test Connection" actions return
a demo-mode error instead of attempting a network call — demo tenants'
hostnames (`*.example.com`) are fictional and must never actually be
dialed. PDF report exports carry a "DEMO DATA" watermark, and a persistent
badge in the app's header makes demo mode visually obvious everywhere.
Every screen otherwise renders demo data through the exact same query
paths as real data — there is no separate demo UI.

**`demo.db` is disposable and must never be treated as a backup.** It is
never migrated in place — if its schema doesn't match what a newer binary
expects (e.g. after an update that added a migration), the server logs a
clear error and falls back to the real database instead, rather than
silently upgrading (and thereby permanently changing) what's supposed to
be a reproducible, regenerable artifact. Regenerate it with `cmd/gendemodb`
or remove it to use demo mode again.

**Staleness:** since history is generated relative to `--end-date`
(defaulting to the day it's generated), a `demo.db` built today will read
as "6 months ending today" forever — it does not advance on its own.
Regenerate it per release (see the release process) so the shipped
artifact always looks current when someone first opens it.

`demo.db` itself is not committed to this repository — it's built as a
release artifact, not a checked-in file.

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
needing this repository itself.

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
