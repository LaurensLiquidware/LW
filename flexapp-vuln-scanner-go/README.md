# FlexApp Vulnerability Scanner (Go + Angular)

Rebuild of `../flexapp-vuln-scanner/` (Python: Flask web UI + PySide6
desktop app) as a Go backend + Angular frontend, packaged as a native
Windows `.exe`. See `../flexapp-vuln-scanner/GO_ANGULAR_REWRITE_PLAN.md`
for the full plan.

**Status: skeleton only.** The server currently exposes `/healthz` and
`/api/version` and serves a placeholder page. Scan functionality (Stage 1
PowerShell invocation, OSV/NVD matching, coverage, SBOM/report
generation, the Angular screens) has not been ported yet — see the plan
document's build order for what's next.

## Build

```sh
make build   # builds the Angular frontend, then the server binary
make test    # go vet + go test
make run     # go run ./cmd/server
```

## Windows packaging

`scripts/build-windows.sh` cross-compiles `cmd/server` and `cmd/tray` as
Windows `.exe` files with embedded icon/version metadata, mirroring
`ProfileUnityMSPConsole`'s packaging. Not yet exercised for this project.

## Version

`VERSION` is the single source of truth. Run `scripts/sync-version.sh`
after changing it, before building.

## Legal / compliance (Sparks Tool checklist)

`Spark_License.pdf`, `bom.cdx.json`, and `THIRD-PARTY-NOTICES.txt` at the
repo root are embedded into the binary (`internal/legal`) and served at
fixed paths for the About screen. **`bom.cdx.json` and
`THIRD-PARTY-NOTICES.txt` here are still stale copies inherited from
`ProfileUnityMSPConsole`** — they need regenerating via
`scripts/generate-sbom.sh` / `scripts/generate-notices.py` once this
project's real dependency set is settled (see the rewrite plan's open
questions on the PDF library choice, which affects the dependency graph).
