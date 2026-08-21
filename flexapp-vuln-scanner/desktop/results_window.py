"""Shows one scan's coverage + findings. Unlike the web UI (which serves
downloads through a Flask route because a browser can't touch the local
filesystem), every export button here just opens the already-written
file with the OS's default app - PDF viewer, spreadsheet app, whatever
findings.csv is associated with.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

from PySide6.QtCore import QSortFilterProxyModel, QUrl, Qt
from PySide6.QtGui import QDesktopServices
from PySide6.QtWidgets import (
    QHBoxLayout,
    QHeaderView,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPushButton,
    QTableView,
    QVBoxLayout,
    QWidget,
)

from models import FindingsTableModel

_SEVERITY_COLORS = {"CRITICAL": "#7f1d1d", "HIGH": "#dc2626", "MEDIUM": "#ca8a04", "LOW": "#71717a"}


class ResultsWindow(QWidget):
    def __init__(self, result: dict[str, Any], on_refresh=None, parent=None) -> None:
        """on_refresh, if given, is called with no args when the user
        clicks "Refresh Vulnerabilities" - the caller owns starting the
        actual worker and calling set_result() when it completes, since
        this widget has no opinion on how a scan is run.
        """
        super().__init__(parent)
        self._on_refresh = on_refresh
        self.setWindowTitle("Scan Results")
        self.resize(1000, 700)

        self._layout = QVBoxLayout(self)
        self.header_label = QLabel()
        self.header_label.setStyleSheet("font-size: 18px; font-weight: 600;")
        self._layout.addWidget(self.header_label)

        self.path_label = QLabel()
        self.path_label.setStyleSheet("color: #71717a; font-size: 11px;")
        self._layout.addWidget(self.path_label)

        self._file_paths: dict[str, str | None] = {}

        actions_row = QHBoxLayout()
        self.pdf_button = QPushButton("PDF")
        self.sbom_button = QPushButton("SBOM")
        self.csv_button = QPushButton("CSV")
        self.refresh_button = QPushButton("Refresh Vulnerabilities")
        self.pdf_button.clicked.connect(lambda: self._open_file("pdf"))
        self.sbom_button.clicked.connect(lambda: self._open_file("sbom"))
        self.csv_button.clicked.connect(lambda: self._open_file("findings_csv"))
        for button in (self.pdf_button, self.sbom_button, self.csv_button):
            actions_row.addWidget(button)
        actions_row.addStretch(1)
        actions_row.addWidget(self.refresh_button)
        self._layout.addLayout(actions_row)
        self.refresh_button.clicked.connect(self._handle_refresh_clicked)

        cards_row = QHBoxLayout()
        self.coverage_card = _stat_card("RESOLUTION COVERAGE", "#0072bc")
        self.severity_cards = {
            level: _stat_card(level, _SEVERITY_COLORS[level]) for level in ("CRITICAL", "HIGH", "MEDIUM", "LOW")
        }
        cards_row.addWidget(self.coverage_card)
        for card in self.severity_cards.values():
            cards_row.addWidget(card)
        self._layout.addLayout(cards_row)

        toolbar_row = QHBoxLayout()
        self.filter_edit = QLineEdit()
        self.filter_edit.setPlaceholderText("Filter findings…")
        toolbar_row.addWidget(self.filter_edit)
        toolbar_row.addStretch(1)
        self.count_label = QLabel()
        self.count_label.setStyleSheet("color: #71717a;")
        toolbar_row.addWidget(self.count_label)
        self._layout.addLayout(toolbar_row)

        self.model = FindingsTableModel()
        self.proxy = QSortFilterProxyModel()
        self.proxy.setSourceModel(self.model)
        # Qt.UserRole carries the model's severity-rank sort key (critical
        # first) - without this, the proxy's default sort compares
        # Qt.DisplayRole text alphabetically, which puts "HIGH" before
        # "CRITICAL".
        self.proxy.setSortRole(Qt.UserRole)
        self.proxy.setFilterCaseSensitivity(Qt.CaseInsensitive)
        self.proxy.setFilterKeyColumn(-1)  # filter across every column
        self.filter_edit.textChanged.connect(self.proxy.setFilterFixedString)

        self.table = QTableView()
        self.table.setModel(self.proxy)
        self.table.setSortingEnabled(True)
        self.table.horizontalHeader().setSectionResizeMode(5, QHeaderView.Stretch)  # Summary column
        self.table.setSelectionBehavior(QTableView.SelectRows)
        self.table.setEditTriggers(QTableView.NoEditTriggers)
        self.table.doubleClicked.connect(self._show_affected_files_if_multiple)
        self._layout.addWidget(self.table)

        self.set_result(result)

    def set_result(self, result: dict[str, Any]) -> None:
        self.result = result
        self.header_label.setText(result["package_name"])
        self.path_label.setText(f"{result['inventory_path']} → {result['output_dir']}")

        pct = result["coverage"]["coveragePercent"]
        self.coverage_card.value_label.setText(f"{pct:.1f}%" if pct is not None else "N/A")
        for level, card in self.severity_cards.items():
            card.value_label.setText(str(result["severity_counts"][level]))

        rows = result["confirmed_rows"] + result["heuristic_rows"]
        self.model.set_rows(rows)
        self.count_label.setText(f"{len(rows)} finding{'s' if len(rows) != 1 else ''}")

        files = result["files"]
        self._file_paths = files
        self.pdf_button.setEnabled(bool(files.get("pdf")))
        self.sbom_button.setEnabled(bool(files.get("sbom")))
        self.csv_button.setEnabled(bool(files.get("findings_csv")))

    def _open_file(self, kind: str) -> None:
        path_str = self._file_paths.get(kind)
        if path_str:
            QDesktopServices.openUrl(QUrl.fromLocalFile(str(Path(path_str))))

    def _handle_refresh_clicked(self) -> None:
        if self._on_refresh is not None:
            self._on_refresh()

    def _show_affected_files_if_multiple(self, proxy_index) -> None:
        if proxy_index.column() != 4:  # Affected Files column
            return
        source_index = self.proxy.mapToSource(proxy_index)
        paths = self.model.affected_files(source_index.row())
        if len(paths) <= 1:
            return
        QMessageBox.information(self, "Affected Files", "\n".join(paths))


def _stat_card(label_text: str, value_color: str) -> QWidget:
    card = QWidget()
    card.setStyleSheet("background: white; border: 1px solid #e4e4e7; border-radius: 8px;")
    layout = QVBoxLayout(card)
    label = QLabel(label_text)
    label.setStyleSheet("color: #a1a1aa; font-size: 10px; font-weight: 600;")
    value = QLabel("—")
    value.setStyleSheet(f"color: {value_color}; font-size: 22px; font-weight: 700;")
    layout.addWidget(label)
    layout.addWidget(value)
    card.value_label = value  # type: ignore[attr-defined]
    return card
