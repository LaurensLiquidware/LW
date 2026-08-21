#!/usr/bin/env python
"""Entry point for the FlexApp Vulnerability Scanner desktop app.

Run with `python main.py` (or the packaged FlexAppVulnScanner.exe - see
README.md for PyInstaller packaging). Requires pwsh (PowerShell 7) on
PATH for Stage 1, same as the web UI.
"""

from __future__ import annotations

import sys

from PySide6.QtWidgets import QApplication

import paths  # noqa: F401 - sys.path setup, must run before flexapp_vuln imports
from main_window import MainWindow


def main() -> int:
    app = QApplication(sys.argv)
    app.setApplicationName("FlexApp Vulnerability Scanner")
    window = MainWindow()
    window.show()
    return app.exec()


if __name__ == "__main__":
    sys.exit(main())
