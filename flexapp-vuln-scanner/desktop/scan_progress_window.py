"""Live progress view for a running ScanWorker: a determinate progress
bar once Stage 2's OSV/NVD phase starts (mirrors the web UI's progress
bar - see PLAN.md entry 13), an indeterminate one before that, and a
scrolling log. Purely a view - it connects to a ScanWorker's signals but
owns none of the scanning logic.
"""

from __future__ import annotations

from PySide6.QtWidgets import QLabel, QProgressBar, QPushButton, QTextEdit, QVBoxLayout, QWidget

_PHASE_LABELS = {
    "osv": "Querying OSV.dev for vulnerability details",
    "nvd": "Querying NVD for CVE matches",
}


class ScanProgressWindow(QWidget):
    def __init__(self, title: str, worker, parent=None) -> None:
        super().__init__(parent)
        self.setWindowTitle(title)
        self.resize(760, 480)
        self.worker = worker

        layout = QVBoxLayout(self)
        self.title_label = QLabel(f"<b>{title}</b>")
        layout.addWidget(self.title_label)

        self.phase_label = QLabel("Waiting to start…")
        layout.addWidget(self.phase_label)

        self.progress_bar = QProgressBar()
        self.progress_bar.setRange(0, 0)  # indeterminate until Stage 2's on_progress fires
        layout.addWidget(self.progress_bar)

        self.log_view = QTextEdit()
        self.log_view.setReadOnly(True)
        self.log_view.setStyleSheet("background: #18181b; color: #d4d4d8; font-family: Consolas, monospace;")
        layout.addWidget(self.log_view)

        self.close_button = QPushButton("Close")
        self.close_button.clicked.connect(self.close)
        layout.addWidget(self.close_button)

        worker.status_changed.connect(self._on_status_changed)
        worker.progress_changed.connect(self._on_progress_changed)
        worker.log_line.connect(self.log_view.append)

    def _on_status_changed(self, status: str) -> None:
        if status == "stage1":
            self.phase_label.setText("Stage 1: mounting the package and building the inventory…")
            self.progress_bar.setRange(0, 0)
        elif status == "stage2":
            self.phase_label.setText("Stage 2: loading inventory…")
            self.progress_bar.setRange(0, 0)
        elif status == "done":
            self.phase_label.setText("Done")
            self.progress_bar.setRange(0, 1)
            self.progress_bar.setValue(1)
        elif status == "error":
            self.phase_label.setText("Failed - see log below")
            self.progress_bar.setRange(0, 1)
            self.progress_bar.setValue(0)

    def _on_progress_changed(self, phase: str, done: int, total: int) -> None:
        if total <= 0:
            return
        self.progress_bar.setRange(0, total)
        self.progress_bar.setValue(done)
        label = _PHASE_LABELS.get(phase, phase)
        self.phase_label.setText(f"{label}: {done} / {total}")
