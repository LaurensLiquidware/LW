"""Runs a scan (or a refresh) on a background QThread, adapting Qt
signals to flexapp_vuln.pipeline's duck-typed ProgressSink interface
(a settable `status` attribute, `append_log(line)`, `set_progress`).
This replaces the web UI's ScanJob + HTTP-polling model with Qt's
native signal/slot mechanism - no polling endpoint needed, the main
window just connects to these signals.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import paths  # noqa: F401 - sys.path setup, must run before flexapp_vuln imports

from PySide6.QtCore import QThread, Signal

import flexapp_vuln.pipeline as pipeline


class ScanWorker(QThread):
    log_line = Signal(str)
    status_changed = Signal(str)
    progress_changed = Signal(str, int, int)  # phase ("osv"/"nvd"), done, total
    succeeded = Signal(dict)  # the result dict from pipeline.run_stage2/write_reports
    failed = Signal(str)  # a plain error message, never a raw traceback

    def __init__(
        self,
        package_path: str,
        output_dir: str,
        nvd_api_key: str | None = None,
        refresh_inventory_path: str | None = None,
        parent=None,
    ) -> None:
        """refresh_inventory_path, if given, skips Stage 1 entirely and
        re-runs Stage 2 against that existing inventory JSON - the
        desktop equivalent of the web UI's "Refresh Vulnerabilities".
        """
        super().__init__(parent)
        self.package_path = package_path
        self.output_dir = output_dir
        self.nvd_api_key = nvd_api_key
        self.refresh_inventory_path = refresh_inventory_path
        self._status = "queued"

    # -- ProgressSink protocol (see flexapp_vuln.pipeline) --------------

    @property
    def status(self) -> str:
        return self._status

    @status.setter
    def status(self, value: str) -> None:
        self._status = value
        self.status_changed.emit(value)

    def append_log(self, line: str) -> None:
        for sub in line.splitlines() or [""]:
            self.log_line.emit(sub)

    def set_progress(self, phase: str, done: int, total: int) -> None:
        self.progress_changed.emit(phase, done, total)

    # -- QThread ---------------------------------------------------------

    def run(self) -> None:  # noqa: D102 - QThread's entry point, not a public API
        try:
            if self.refresh_inventory_path is not None:
                self.append_log(
                    f"Refreshing vulnerability matches for {self.refresh_inventory_path} "
                    "(Stage 1 not re-run)"
                )
                inventory_path = Path(self.refresh_inventory_path)
            else:
                inventory_path = pipeline.run_stage1(self, self.package_path, self.output_dir)

            result: dict[str, Any] = pipeline.run_stage2(
                self, inventory_path, self.output_dir, self.nvd_api_key,
            )
            self.status = "done"
            self.succeeded.emit(result)
        except Exception as exc:  # noqa: BLE001 - surfaced via the failed signal, never swallowed
            self.status = "error"
            self.append_log(f"ERROR: {exc}")
            self.failed.emit(str(exc))
