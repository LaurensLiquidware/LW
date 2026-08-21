"""The "New Scan" dialog: package path + output folder (auto-derived,
editable) + a collapsible Advanced section for the optional NVD API key.
Both path fields use the OS's real file/folder picker (QFileDialog) -
this is the whole reason a native app doesn't need anything like the
web UI's browse.py: UNC paths and network drives just work.
"""

from __future__ import annotations

from pathlib import Path, PureWindowsPath

from PySide6.QtCore import Qt
from PySide6.QtWidgets import (
    QDialog,
    QDialogButtonBox,
    QFileDialog,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QPushButton,
    QToolButton,
    QVBoxLayout,
    QWidget,
)

import paths  # noqa: F401 - sys.path setup, must run before flexapp_vuln imports

PACKAGE_FILTER = "FlexApp packages (*.vhdx *.exe *.flexapp)"


class NewScanDialog(QDialog):
    def __init__(self, default_output_root: Path, parent=None) -> None:
        super().__init__(parent)
        self.setWindowTitle("New Scan")
        self.setMinimumWidth(520)
        self._default_output_root = default_output_root
        self._output_dir_edited_by_hand = False

        layout = QVBoxLayout(self)

        layout.addWidget(QLabel("<b>Package Path</b> (.vhdx, .exe, or .flexapp)"))
        package_row = QHBoxLayout()
        self.package_path_edit = QLineEdit()
        self.package_path_edit.setPlaceholderText(r"D:\FlexAppShare\Example.vhdx")
        package_browse = QPushButton("Browse…")
        package_browse.clicked.connect(self._browse_package)
        package_row.addWidget(self.package_path_edit)
        package_row.addWidget(package_browse)
        layout.addLayout(package_row)

        layout.addWidget(QLabel("<b>Output Folder</b>"))
        output_row = QHBoxLayout()
        self.output_dir_edit = QLineEdit()
        self.output_dir_edit.textEdited.connect(self._on_output_dir_edited_by_hand)
        output_browse = QPushButton("Browse…")
        output_browse.clicked.connect(self._browse_output_dir)
        output_row.addWidget(self.output_dir_edit)
        output_row.addWidget(output_browse)
        layout.addLayout(output_row)
        layout.addWidget(_hint("Auto-filled from the package name - edit for a different folder."))

        self.package_path_edit.textChanged.connect(self._auto_fill_output_dir)

        self.advanced_toggle = QToolButton()
        self.advanced_toggle.setText("Advanced")
        self.advanced_toggle.setToolButtonStyle(Qt.ToolButtonTextBesideIcon)
        self.advanced_toggle.setCheckable(True)
        self.advanced_toggle.setArrowType(Qt.RightArrow)
        self.advanced_toggle.toggled.connect(self._toggle_advanced)
        layout.addWidget(self.advanced_toggle)

        self.advanced_panel = QWidget()
        advanced_layout = QVBoxLayout(self.advanced_panel)
        advanced_layout.addWidget(QLabel("NVD API Key (optional)"))
        self.nvd_api_key_edit = QLineEdit()
        self.nvd_api_key_edit.setPlaceholderText("Leave blank to use the NVD_API_KEY environment variable")
        advanced_layout.addWidget(self.nvd_api_key_edit)
        self.advanced_panel.setVisible(False)
        layout.addWidget(self.advanced_panel)

        buttons = QDialogButtonBox(QDialogButtonBox.Ok | QDialogButtonBox.Cancel)
        buttons.button(QDialogButtonBox.Ok).setText("Start Scan")
        buttons.accepted.connect(self.accept)
        buttons.rejected.connect(self.reject)
        layout.addWidget(buttons)

    def _browse_package(self) -> None:
        path, _ = QFileDialog.getOpenFileName(self, "Select a FlexApp package", "", PACKAGE_FILTER)
        if path:
            self.package_path_edit.setText(path)

    def _browse_output_dir(self) -> None:
        start = self.output_dir_edit.text() or str(self._default_output_root)
        chosen = QFileDialog.getExistingDirectory(self, "Select an output folder", start)
        if chosen:
            self.output_dir_edit.setText(chosen)
            self._output_dir_edited_by_hand = True

    def _on_output_dir_edited_by_hand(self) -> None:
        self._output_dir_edited_by_hand = True

    def _auto_fill_output_dir(self, package_path: str) -> None:
        if self._output_dir_edited_by_hand:
            return
        # Always a Windows-style path (this app only ever runs on Windows,
        # see NATIVE_APP_MIGRATION.md) - PureWindowsPath parses "\" as a
        # separator regardless of the host platform this code happens to
        # run on, unlike plain Path which uses the host's own flavor.
        stem = PureWindowsPath(package_path).stem if package_path else ""
        suggested = str(self._default_output_root / stem) if stem else str(self._default_output_root)
        self.output_dir_edit.blockSignals(True)
        self.output_dir_edit.setText(suggested)
        self.output_dir_edit.blockSignals(False)

    def _toggle_advanced(self, checked: bool) -> None:
        self.advanced_toggle.setArrowType(Qt.DownArrow if checked else Qt.RightArrow)
        self.advanced_panel.setVisible(checked)

    @property
    def package_path(self) -> str:
        return self.package_path_edit.text().strip()

    @property
    def output_dir(self) -> str:
        return self.output_dir_edit.text().strip()

    @property
    def nvd_api_key(self) -> str | None:
        text = self.nvd_api_key_edit.text().strip()
        return text or None


def _hint(text: str) -> QLabel:
    label = QLabel(text)
    label.setStyleSheet("color: #71717a; font-size: 11px;")
    return label
