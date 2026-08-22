# Working agreements

## Versioning

Any new user request that changes behavior (a fix, a feature, anything
beyond docs/comments) bumps the version. Bump `VERSION` (and add a new
`## x.y.z` heading in `CHANGELOG.md` for it) as part of that request's own
commit — a patch bump for a fix, a minor bump for a new feature, following
normal semver judgement. Run `scripts/sync-version.sh` after changing
`VERSION` so the embedded copy stays in sync before building.

## Origin

This project's Go/Angular skeleton was copied from `../ProfileUnityMSPConsole/`
(same repo) and stripped down to fit a local, single-user, no-database,
no-auth tool. See `GO_ANGULAR_REWRITE_PLAN.md` for the rewrite plan, the
Python→Go component mapping, and the build order this project followed.

The original Python implementation (`../flexapp-vuln-scanner/`: Flask web
UI, PySide6 desktop app, and the Stage 1/Stage 2 reference logic this
project's Go code was ported from) was the reference implementation until
this project reached parity and was validated on a real Windows machine
(real scans, real UAC elevation, real file picker) — at that point it was
retired and deleted entirely, per the rewrite plan's own build order. It
no longer exists in this repo; do not assume it's there as a fallback or
reference when working on this project.
