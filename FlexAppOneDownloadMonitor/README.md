# FlexApp One Download Monitor - Install Guide

**Version 0.2**

> **IMPORTANT: READ BEFORE DOWNLOADING OR USING.** This is a Liquidware **Sparks Tool** - a community/field-contributed utility, **not a Liquidware commercial product**. It is provided outside Liquidware's standard product development lifecycle, **"AS IS" with no warranty, support, or maintenance**, and used at your own risk. See `Spark_License.pdf` for the full license and disclaimer.

A system-tray tool that watches for FlexApp One downloads and shows live progress, speed, ETA, and history.

## Files

| File | Purpose |
|---|---|
| `FlexAppOneDownloadMonitor.ps1` | The application itself |
| `Start-FlexAppOneDownloadMonitor.vbs` | Silent launcher (no console window) |
| `Spark_License.pdf` | Liquidware Sparks Tool License and Disclaimer - read this before use |
| `bom.cdx.json` | Software Bill of Materials (CycloneDX 1.6, JSON) - an inventory of third-party components, provided so your security team can review it against your own policy. This tool currently has none. |
| `CHANGELOG.md` | Version history |

## 1. Install

1. Create a folder: `C:\FlexAppOneDownloadMonitor`
2. Copy both files into it.
3. Unblock the script (it came from outside the machine):
   ```powershell
   Unblock-File C:\FlexAppOneDownloadMonitor\FlexAppOneDownloadMonitor.ps1
   ```

## 2. Run it

**Quick test (visible console, easiest to debug):**
```powershell
powershell.exe -NoProfile -STA -ExecutionPolicy Bypass -File C:\FlexAppOneDownloadMonitor\FlexAppOneDownloadMonitor.ps1
```
A tray icon (blue circle with a down arrow) should appear within a second or two.

**Normal use (silent, no window):**
Double-click `Start-FlexAppOneDownloadMonitor.vbs`.

> If you move the files, edit the `scriptPath` line inside the `.vbs` to match.

## 3. Run automatically at logon

Drop a shortcut to `Start-FlexAppOneDownloadMonitor.vbs` into the Startup folder:
- Press `Win+R`, type `shell:startup`, press Enter.
- Place the shortcut there.

(For environments where the Startup folder is restricted by policy, use a Scheduled Task triggered "At log on" instead, running the same PowerShell command as above with `-WindowStyle Hidden`.)

## 4. Using it

- **Pops up automatically** when a new FlexApp package starts downloading.
- **Left-click** the tray icon to toggle the panel manually.
- **Right-click** the tray icon for the menu:
  - **Show downloads** - open the panel
  - **Refresh now** - force an immediate check
  - **Diagnostics...** - shows the monitored process, current speed reading, and download details (useful if something looks wrong)
  - **Open log file** - opens the activity log in Notepad
  - **Exit** - closes the app

## 5. Configuration

A config file is created automatically on first run:
```
C:\FlexAppOneDownloadMonitor\FlexAppOneDownloadMonitor.config.json
```
It holds one setting - `CacheDir`, the folder being watched (defaults to `C:\ProgramData\Liquidware\ProfileUnity\Cache\FlexAppOne\`). Edit it directly if that path differs in your environment, then restart the app.

## 6. Troubleshooting

- **Log file**: `C:\FlexAppOneDownloadMonitor\FlexAppOneDownloadMonitor.log` (also reachable via the tray menu). Records every download tracked, every raw file-system event seen in the cache folder, and why any event was skipped. Auto-rotates at 5MB.
- **Diagnostics**: shows the running version, the license/SBOM file paths, whether the write-speed process was found, whether the watcher is active, and the last raw file event seen.
- **To stop it**: right-click the tray icon → Exit, or log off/reboot.
