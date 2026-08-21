# FlexApp Vulnerability Scanner (Go + Angular)

Rebuild of `../flexapp-vuln-scanner/` (Python: Flask web UI + PySide6
desktop app) as a Go backend + Angular frontend, packaged as a native
Windows `.exe`. See `../flexapp-vuln-scanner/GO_ANGULAR_REWRITE_PLAN.md`
for the full plan.

**Status:** the Stage 2 business logic (OSV/NVD matching, coverage,
SBOM, reports) is ported and wired into a real HTTP API, and the
Angular screens (Dashboard, New Scan, Scan Progress, Results, Compare,
About) call it end-to-end. No SSE/streaming progress yet (poll-only).

## Build

```sh
make build   # builds the Angular frontend, then the server binary
make test    # go vet + go test
make run     # go run ./cmd/server
```

## Windows packaging

`scripts/build-windows.sh` cross-compiles `cmd/server` and `cmd/tray` as
Windows `.exe` files with embedded icon/version metadata, mirroring
`ProfileUnityMSPConsole`'s packaging. Verified from this Linux dev
environment: both binaries cross-compile to valid Windows PE32+
executables (GUI subsystem for the tray, console for the server) via
`GOOS=windows GOARCH=amd64`. Not yet run/smoke-tested on a real Windows
machine.

## Version

`VERSION` is the single source of truth. Run `scripts/sync-version.sh`
after changing it, before building.

## Legal / compliance (Sparks Tool checklist)

`Spark_License.pdf`, `bom.cdx.json`, and `THIRD-PARTY-NOTICES.txt` at the
repo root are embedded into the binary (`internal/legal`) and served at
fixed paths for the About screen. `bom.cdx.json` (120 components: 3 Go
modules + 117 npm packages) and `THIRD-PARTY-NOTICES.txt` are generated
from this project's real dependency graph via `scripts/generate-sbom.sh`
(requires `cyclonedx-gomod` on `PATH`) and `scripts/generate-notices.py`
— re-run both, then `scripts/sync-legal.sh`, whenever dependencies
change.

**Not yet done**: a Grype scan of `bom.cdx.json` for the checklist's
"zero Critical/High CVEs" requirement — `grype.anchore.io` and GitHub
release downloads are blocked from this dev environment's network
egress policy (`go install`ing the Grype CLI itself also didn't
complete), the same restriction already documented for
`FlexAppOneDownloadMonitor`'s Sparks audit. Needs running from a
network-unrestricted machine, the same way Stage 1's PowerShell scripts
need a real Windows machine.
