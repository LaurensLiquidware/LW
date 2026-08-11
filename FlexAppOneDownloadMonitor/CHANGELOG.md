# Changelog

## 0.2

- Initial Sparks Tool submission.
- System-tray monitor for FlexApp One package downloads: live progress, elapsed time, speed, ETA, and history.
- Version number now surfaced in the flyout title bar, tray tooltip (idle state), log startup line, and the Diagnostics dialog.
- Sparks Tool License and SBOM packaged alongside the tool (`Spark_License.pdf`, `bom.cdx.json`).
- Encoding hardening: source file saved as UTF-8 with BOM, config file read with an explicit encoding, tray tooltip truncation made safe for double-byte/combining characters.
- Flyout panel and tray icon colors re-pointed to the Liquidware style guide's dark-scheme palette.
