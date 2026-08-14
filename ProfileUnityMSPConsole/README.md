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

## Status: Phase 4 — Frontend shell

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
  exist yet.
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

Tenant CRUD still has no HTTP API or screen — that's Phase 5 alongside
the dashboard. For now, exercising the collector/scheduler still means
calling `internal/tenant`'s Go API directly.

**Known CVEs carried by this phase, to resolve in Phase 8:** `npm audit`
on `web/frontend` reports several real high-severity Angular XSS
advisories (e.g. GHSA-g93w-mfhg-p222, GHSA-rgjc-h3x7-9mwg) affecting
Angular ≤18.2.x, fixed only in later Angular majors. Angular 18 was
chosen here because it's the last major both PrimeNG 18 (Aura preset,
matching the project brief's stack) and this build support without
further compatibility work. Upgrading is deferred to the Phase 8
compliance pass, where the SBOM/Grype ordering in §11.8 applies anyway —
noting it here per the working agreement to flag known CVEs immediately
rather than waiting.

**PrimeNG licensing note:** the supplied commercial PrimeNG license key
(§10) does not match the format PrimeNG's own `-lts` package variant
expects (that variant verifies an AES-GCM-encrypted PrimeNG-specific
token; the supplied key decodes to an unrelated `{product: "primeui",
tier: "commercial"}` JWT-shaped claim, likely for a different Prime
offering such as premium templates). Using the `-lts` package without a
matching key displays a customer-visible "invalid license" banner, which
is unacceptable for a Sparks Tool — so this build uses the plain
(non-`-lts`), MIT-licensed `primeng`/`@primeng/themes` releases instead,
which need no key at all. The key was not consumed anywhere in the build.
Flagging both open questions from §10 for the reviewer, unresolved:
whether the key is meant for something this project doesn't currently
use, and — if a licensed dependency is ever added — the dev-tier/SBOM
questions §10 already calls out.

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

Copy `.env.example` to `.env` and fill in values, or set the equivalent
environment variables directly. There is deliberately no baked-in
`localhost` default for the listen address — this is a continuously
running, multi-user server, not a local single-operator tool, and it must
be bound to an address you chose on purpose. See `.env.example` for the
full list.

The server always serves HTTPS. If `PUMC_TLS_CERT_FILE`/`PUMC_TLS_KEY_FILE`
don't both already exist, a self-signed pair is generated there at first
startup (browsers will warn about it — replace it with a CA-signed pair
for anything beyond local/lab use). No operator account exists until you
set `PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD` for the first run; the
server logs a warning and stays otherwise unusable if nobody can sign in.

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

## Reference project

Architectural patterns carried over from `ProUVisualizer-Go` (proxy
whitelisting, CSRF/session handling, normalize-at-the-edge, pure business
logic, root-provided shared frontend services, reactive query params,
version-sync-as-a-build-step, hidden-canvas measurement) are documented
inline where each is used, not copied wholesale — this is a multi-user
server, not a local single-operator tool, so binding to `127.0.0.1` only
is explicitly **not** carried over.
