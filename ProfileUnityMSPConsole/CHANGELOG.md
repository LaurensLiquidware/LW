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
