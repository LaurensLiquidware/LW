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
