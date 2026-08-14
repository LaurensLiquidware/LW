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
files are still placeholders** — real dependencies exist already (SQLite,
`google/uuid`), but per the project brief §11.8 the SBOM is only ever
regenerated for real as part of the compliance pass in Phase 8, after CVE
remediation, so it describes exactly what ships rather than an
intermediate state. See "Compliance" below.

## What this is

A multi-tenant web console that lets a Managed Service Provider see
ProfileUnity license consumption across all their registered customer
consoles, track it over time, and produce monthly reports. See the project
brief for the full functional and compliance spec; this README tracks
build-phase status and the things a reader needs before touching the code.

## Status: Phase 3 — Collector and scheduler

Phase 1 (repo layout, config, migrations, health endpoint, version
single-source-of-truth) and Phase 2 (the `internal/profileunity` API
client — §3's wire contract, `Type`-not-status checks, the `/Date(ms)/`
format, explicit US-date parsing, the unauthenticated/authenticated
fallback) are done.

Phase 3 wires that client into a running server:

- `internal/tenant`: registered-console CRUD. Credentials are encrypted
  at rest (AES-256-GCM, key held outside the database per §9) and never
  travel through the `Tenant` type once saved — only a collector-only
  `GetCredentials` call ever produces a plaintext password.
- `internal/snapshot`: one row per tenant per collection day, upserted —
  re-running collection the same day updates that row instead of
  duplicating it. A failed poll stores nil license figures, never a zero.
- `internal/collector`: runs one tenant's poll, retries transient
  failures (unreachable, timeout) with backoff, and classifies every
  other outcome (TLS failure, auth rejected/required, malformed
  response) into a distinct stored status. The raw response body is
  retained alongside the parsed fields.
- `internal/scheduler`: an in-process ticker (no external cron
  dependency) that polls every enabled tenant concurrently, capped, with
  a per-tenant timeout so one dead tenant can never stall the run. Manual
  "Collect Now" is the same code path the ticker uses. `/healthz` reports
  live scheduler state — running/idle, last run's outcome, tenant/success
  counts.

There is still no HTTP API or UI for registering tenants or viewing
results — that is Phase 4 (frontend shell) and Phase 5 (dashboard). Tenant
registration for now happens only by calling `internal/tenant`'s Go API
directly (e.g. from a short-lived script), which is enough to exercise
the collector and scheduler end to end.

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

## Design system reference

`docs/design-system-reference/` holds the raw Liquidware style-guide
bundle used to build the Phase 4 frontend. It is reference material only —
nothing under it is built or embedded by the shipped binary. It contains
two known CDN references (`unpkg.com`, in `primeicons-cdn.css` and the
preview harness `support.js`) that must **not** be carried into the
vendored frontend; see `docs/design-system-reference/NOTE.md`.

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
go build -o profileunity-msp-console ./cmd/server
```

or `make build`.

## Configuration

Copy `.env.example` to `.env` and fill in values, or set the equivalent
environment variables directly. There is deliberately no baked-in
`localhost` default for the listen address — this is a continuously
running, multi-user server, not a local single-operator tool, and it must
be bound to an address you chose on purpose. See `.env.example` for the
full list.

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
