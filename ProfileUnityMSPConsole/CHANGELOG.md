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
