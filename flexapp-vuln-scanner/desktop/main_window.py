"""The dashboard: New Scan / Compare Scans actions and a Recent Scans
table that persists across restarts (see recent_scans_store.py) - unlike
the web UI's in-memory job list, which resets every time that process
restarts.
"""

from __future__ import annotations

from pathlib import Path

from PySide6.QtWidgets import (
    QHBoxLayout,
    QHeaderView,
    QLabel,
    QMainWindow,
    QMessageBox,
    QPushButton,
    QTableWidget,
    QTableWidgetItem,
    QVBoxLayout,
    QWidget,
)

import paths  # noqa: F401 - sys.path setup, must run before flexapp_vuln imports

from flexapp_vuln.pipeline import load_existing_result
from compare_dialog import CompareDialog
from new_scan_dialog import NewScanDialog
from recent_scans_store import RecentScansStore
from results_window import ResultsWindow
from scan_progress_window import ScanProgressWindow
from scan_worker import ScanWorker

_COLUMNS = ["Started", "Package", "Status", "Findings by Severity", ""]
_SEVERITY_COLORS = {"CRITICAL": "#7f1d1d", "HIGH": "#dc2626", "MEDIUM": "#ca8a04", "LOW": "#71717a"}


class MainWindow(QMainWindow):
    def __init__(self, store: RecentScansStore | None = None, parent=None) -> None:
        super().__init__(parent)
        self.setWindowTitle("FlexApp Vulnerability Scanner")
        self.resize(1100, 720)

        self.store = store or RecentScansStore()
        self._workers: dict[str, ScanWorker] = {}
        self._live_results: dict[str, dict] = {}
        self._open_windows: list[QWidget] = []  # keep references so Qt doesn't garbage-collect them

        central = QWidget()
        self.setCentralWidget(central)
        layout = QVBoxLayout(central)

        header_row = QHBoxLayout()
        title_col = QVBoxLayout()
        title_col.addWidget(_h1("FlexApp Vulnerability Scanner"))
        title_col.addWidget(_hint("Scan a FlexApp package, track its resolution coverage, and watch for new CVEs over time."))
        header_row.addLayout(title_col)
        header_row.addStretch(1)

        self.compare_button = QPushButton("Compare Scans")
        self.compare_button.clicked.connect(self._open_compare)
        header_row.addWidget(self.compare_button)

        self.new_scan_button = QPushButton("New Scan")
        self.new_scan_button.clicked.connect(self._open_new_scan_dialog)
        header_row.addWidget(self.new_scan_button)
        layout.addLayout(header_row)

        layout.addWidget(QLabel("<b>Recent Scans</b>"))
        self.table = QTableWidget(0, len(_COLUMNS))
        self.table.setHorizontalHeaderLabels(_COLUMNS)
        header = self.table.horizontalHeader()
        header.setSectionResizeMode(0, QHeaderView.ResizeToContents)
        header.setSectionResizeMode(1, QHeaderView.Stretch)
        header.setSectionResizeMode(2, QHeaderView.ResizeToContents)
        header.setSectionResizeMode(3, QHeaderView.ResizeToContents)
        header.setSectionResizeMode(4, QHeaderView.ResizeToContents)
        self.table.verticalHeader().setVisible(False)
        self.table.setEditTriggers(QTableWidget.NoEditTriggers)
        layout.addWidget(self.table)

        self._default_output_root = paths.REPO_ROOT / "scan-out"
        self._reload_table()

    # -- New scan -----------------------------------------------------------

    def _open_new_scan_dialog(self) -> None:
        dialog = NewScanDialog(self._default_output_root, self)
        if dialog.exec() != NewScanDialog.Accepted:
            return
        if not dialog.package_path or not dialog.output_dir:
            QMessageBox.warning(self, "New Scan", "Both a package path and an output folder are required.")
            return

        entry = self.store.add(dialog.package_path, dialog.output_dir, kind="scan")
        worker = ScanWorker(dialog.package_path, dialog.output_dir, nvd_api_key=dialog.nvd_api_key)
        self._start_worker(entry["id"], worker)

    # -- Refresh --------------------------------------------------------------

    def _start_refresh(self, entry_id: str, inventory_path: str, output_dir: str) -> None:
        worker = ScanWorker("", output_dir, refresh_inventory_path=inventory_path)
        self.store.update(entry_id, status="queued", error=None)
        self._start_worker(entry_id, worker)

    # -- Shared worker wiring --------------------------------------------------

    def _start_worker(self, entry_id: str, worker: ScanWorker) -> None:
        self._workers[entry_id] = worker
        progress_window = ScanProgressWindow(f"Scan: {entry_id}", worker, self)
        self._open_windows.append(progress_window)
        progress_window.show()

        worker.status_changed.connect(lambda status: self._on_status_changed(entry_id, status))
        worker.succeeded.connect(lambda result: self._on_succeeded(entry_id, result))
        worker.failed.connect(lambda message: self._on_failed(entry_id, message))
        worker.start()

    def _on_status_changed(self, entry_id: str, status: str) -> None:
        self.store.update(entry_id, status=status)
        self._reload_table()

    def _on_succeeded(self, entry_id: str, result: dict) -> None:
        self._live_results[entry_id] = result
        self.store.update(
            entry_id,
            status="done",
            package_name=result["package_name"],
            coverage_percent=result["coverage"]["coveragePercent"],
            severity_counts=result["severity_counts"],
            inventory_path=result["inventory_path"],
            output_dir=result["output_dir"],
        )
        self._reload_table()

    def _on_failed(self, entry_id: str, message: str) -> None:
        self.store.update(entry_id, status="error", error=message)
        self._reload_table()

    # -- Compare --------------------------------------------------------------

    def _open_compare(self) -> None:
        dialog = CompareDialog(self)
        self._open_windows.append(dialog)
        dialog.show()

    # -- Table ------------------------------------------------------------------

    def _reload_table(self) -> None:
        entries = self.store.all()
        self.table.setRowCount(len(entries))
        for row, entry in enumerate(entries):
            self.table.setItem(row, 0, QTableWidgetItem(entry["created_at"]))
            label = entry["package_path"] if entry["kind"] == "scan" else f"(refresh) {entry.get('inventory_path') or entry['output_dir']}"
            self.table.setItem(row, 1, QTableWidgetItem(label))
            self.table.setItem(row, 2, QTableWidgetItem(entry["status"]))
            self.table.setItem(row, 3, QTableWidgetItem(_severity_summary(entry)))

            action_button = QPushButton("View Results" if entry["status"] == "done" else "View Progress")
            action_button.clicked.connect(lambda _checked=False, e=entry: self._handle_action_clicked(e))
            self.table.setCellWidget(row, 4, action_button)

    def _handle_action_clicked(self, entry: dict) -> None:
        entry_id = entry["id"]
        if entry["status"] != "done":
            worker = self._workers.get(entry_id)
            if worker is not None:
                progress_window = ScanProgressWindow(entry["package_path"], worker, self)
                self._open_windows.append(progress_window)
                progress_window.show()
            else:
                QMessageBox.information(self, "Scan", f"Status: {entry['status']}\n{entry.get('error') or ''}")
            return

        result = self._live_results.get(entry_id)
        if result is None:
            inventory_path = entry.get("inventory_path")
            if not inventory_path:
                QMessageBox.warning(self, "View Results", "No inventory path recorded for this scan.")
                return
            result = load_existing_result(Path(inventory_path))
            self._live_results[entry_id] = result

        results_window = ResultsWindow(
            result, on_refresh=lambda: self._start_refresh(entry_id, result["inventory_path"], entry["output_dir"]),
        )
        self._open_windows.append(results_window)
        results_window.show()


def _severity_summary(entry: dict) -> str:
    counts = entry.get("severity_counts")
    if entry["status"] != "done":
        return "—"
    if not counts:
        return "no vuln data"
    return "  ".join(f"{level[0]} {counts[level]}" for level in ("CRITICAL", "HIGH", "MEDIUM", "LOW"))


def _h1(text: str) -> QLabel:
    label = QLabel(text)
    label.setStyleSheet("font-size: 20px; font-weight: 600;")
    return label


def _hint(text: str) -> QLabel:
    label = QLabel(text)
    label.setStyleSheet("color: #71717a; font-size: 11px;")
    return label
