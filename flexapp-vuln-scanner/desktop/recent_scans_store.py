"""Persists the dashboard's "Recent Scans" list across app restarts.

The web UI's JobRegistry (webui/jobs.py) is memory-only and resets every
time that process restarts - fine for a page you reload, wrong for a
desktop app someone closes and reopens. This stores just enough to
render the dashboard row without re-reading every report file: id,
package path, output dir, status, timestamp, and (once done) the
package name, coverage percent, and severity counts. Opening the full
results view still re-reads the real files via
flexapp_vuln.pipeline.load_existing_result() - this store is a shortcut
for the list, not a second source of truth.
"""

from __future__ import annotations

import json
import os
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def default_store_path() -> Path:
    """%APPDATA%\\FlexAppVulnScanner\\recent-scans.json on Windows, the
    XDG-ish equivalent elsewhere (this repo's dev/test environment is
    Linux - the app itself targets Windows, see NATIVE_APP_MIGRATION.md).
    """
    if sys.platform == "win32":
        base = Path(os.environ.get("APPDATA", Path.home() / "AppData" / "Roaming"))
    else:
        base = Path(os.environ.get("XDG_DATA_HOME", Path.home() / ".local" / "share"))
    return base / "FlexAppVulnScanner" / "recent-scans.json"


class RecentScansStore:
    """A flat JSON array of scan entries, newest first. Every mutating
    call rewrites the whole file - this list is expected to stay small
    (dozens to low hundreds of entries for a tool run interactively),
    so there's no real cost to keeping it simple rather than appending
    to a growing file or reaching for SQLite.
    """

    def __init__(self, path: Path | None = None) -> None:
        self.path = path or default_store_path()

    def _read(self) -> list[dict[str, Any]]:
        if not self.path.is_file():
            return []
        try:
            data = json.loads(self.path.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError):
            return []
        return data if isinstance(data, list) else []

    def _write(self, entries: list[dict[str, Any]]) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.path.write_text(json.dumps(entries, indent=2), encoding="utf-8")

    def all(self) -> list[dict[str, Any]]:
        return self._read()

    def add(self, package_path: str, output_dir: str, kind: str = "scan") -> dict[str, Any]:
        """kind is "scan" (a fresh Stage 1+2 run) or "refresh" (Stage 2
        only, against an existing inventory) - purely cosmetic, shown
        in the dashboard row so a refresh doesn't look like a full
        re-scan happened.
        """
        entry = {
            "id": uuid.uuid4().hex[:12],
            "kind": kind,
            "package_path": package_path,
            "output_dir": output_dir,
            "status": "queued",
            "created_at": datetime.now(timezone.utc).isoformat(),
            "error": None,
            "package_name": None,
            "coverage_percent": None,
            "severity_counts": None,
            "inventory_path": None,
        }
        entries = self._read()
        entries.insert(0, entry)
        self._write(entries)
        return entry

    def update(self, entry_id: str, **fields: Any) -> None:
        entries = self._read()
        for entry in entries:
            if entry["id"] == entry_id:
                entry.update(fields)
                break
        self._write(entries)

    def remove(self, entry_id: str) -> None:
        entries = [e for e in self._read() if e["id"] != entry_id]
        self._write(entries)
