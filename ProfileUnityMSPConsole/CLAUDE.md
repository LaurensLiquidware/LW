# Working agreements

## Versioning

The current batch of work (tray launcher fixes, SMTP settings fixes,
Settings "Send Now" button, Users management screen) stays on version
`0.1.0` (see `VERSION`, `CHANGELOG.md`'s `## 0.1.0 — unreleased`
heading).

**From this point on: any new user request that changes behavior (a
fix, a feature, anything beyond docs/comments) bumps the version.**
Bump `VERSION` (and add a new `## x.y.z` heading in `CHANGELOG.md` for
it) as part of that request's own commit — a patch bump (`0.1.0` ->
`0.1.1`) for a fix, a minor bump (`0.1.0` -> `0.2.0`) for a new feature,
following normal semver judgement. Run `scripts/sync-version.sh` after
changing `VERSION` so the embedded copy stays in sync before building.
