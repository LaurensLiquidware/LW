"""Compares two single-package scan output folders: coverage change,
new findings, and resolved findings. Thin UI over
flexapp_vuln.pipeline.load_diff() - all the matching logic (including
what counts as "the same finding" across two scans) lives there,
shared with the web UI's identical feature.
"""

from __future__ import annotations

from pathlib import Path

from PySide6.QtWidgets import (
    QFileDialog,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPushButton,
    QTableView,
    QVBoxLayout,
    QWidget,
)

import paths  # noqa: F401 - sys.path setup, must run before flexapp_vuln imports

from flexapp_vuln.pipeline import DiffError, load_diff
from models import FindingsTableModel


class CompareDialog(QWidget):
    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self.setWindowTitle("Compare Scans")
        self.resize(1000, 700)

        layout = QVBoxLayout(self)

        pickers_row = QHBoxLayout()
        self.old_dir_edit, old_picker = _folder_picker("Older Scan Output Directory")
        self.new_dir_edit, new_picker = _folder_picker("Newer Scan Output Directory")
        pickers_row.addWidget(old_picker)
        pickers_row.addWidget(new_picker)
        layout.addLayout(pickers_row)

        compare_row = QHBoxLayout()
        self.error_label = QLabel()
        self.error_label.setStyleSheet("color: #dc2626;")
        compare_row.addWidget(self.error_label)
        compare_row.addStretch(1)
        self.compare_button = QPushButton("Compare")
        self.compare_button.clicked.connect(self._run_compare)
        compare_row.addWidget(self.compare_button)
        layout.addLayout(compare_row)

        self.summary_label = QLabel()
        layout.addWidget(self.summary_label)

        tables_row = QHBoxLayout()
        new_col = QVBoxLayout()
        new_col.addWidget(QLabel("<b>New Findings</b>"))
        self.new_model = FindingsTableModel()
        self.new_table = QTableView()
        self.new_table.setModel(self.new_model)
        self.new_table.setEditTriggers(QTableView.NoEditTriggers)
        new_col.addWidget(self.new_table)

        resolved_col = QVBoxLayout()
        resolved_col.addWidget(QLabel("<b>Resolved Findings</b>"))
        self.resolved_model = FindingsTableModel()
        self.resolved_table = QTableView()
        self.resolved_table.setModel(self.resolved_model)
        self.resolved_table.setEditTriggers(QTableView.NoEditTriggers)
        resolved_col.addWidget(self.resolved_table)

        tables_row.addLayout(new_col)
        tables_row.addLayout(resolved_col)
        layout.addLayout(tables_row)

    def _run_compare(self) -> None:
        self.error_label.setText("")
        old_dir = self.old_dir_edit.text().strip()
        new_dir = self.new_dir_edit.text().strip()
        if not old_dir or not new_dir:
            self.error_label.setText("Both an older and a newer scan output directory are required.")
            return

        try:
            diff = load_diff(Path(old_dir), Path(new_dir))
        except DiffError as exc:
            self.error_label.setText(str(exc))
            return

        old_pct = diff["old"]["coverage"]["coveragePercent"]
        new_pct = diff["new"]["coverage"]["coveragePercent"]
        old_pct_str = f"{old_pct:.1f}%" if old_pct is not None else "N/A"
        new_pct_str = f"{new_pct:.1f}%" if new_pct is not None else "N/A"
        unchanged = diff["unchanged_count"]
        self.summary_label.setText(
            f"Coverage: {old_pct_str} → {new_pct_str}  ·  "
            f"{unchanged} finding{'s' if unchanged != 1 else ''} unchanged"
        )

        self.new_model.set_rows(diff["new_findings"])
        self.resolved_model.set_rows(diff["resolved_findings"])

    def _browse(self, edit: QLineEdit) -> None:
        chosen = QFileDialog.getExistingDirectory(self, "Select a scan output folder", edit.text())
        if chosen:
            edit.setText(chosen)


def _folder_picker(label_text: str) -> tuple[QLineEdit, QWidget]:
    container = QWidget()
    layout = QVBoxLayout(container)
    layout.addWidget(QLabel(f"<b>{label_text}</b>"))
    row = QHBoxLayout()
    edit = QLineEdit()
    browse = QPushButton("Browse…")
    row.addWidget(edit)
    row.addWidget(browse)
    layout.addLayout(row)

    def do_browse() -> None:
        chosen = QFileDialog.getExistingDirectory(container, "Select a scan output folder", edit.text())
        if chosen:
            edit.setText(chosen)

    browse.clicked.connect(do_browse)
    return edit, container
