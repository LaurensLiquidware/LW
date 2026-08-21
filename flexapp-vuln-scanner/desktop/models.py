"""Qt table models over flexapp_vuln's already-computed row data. A
QTableView + QSortFilterProxyModel (both stock Qt, no custom code
needed) gets sortable/filterable columns for free - see
NATIVE_APP_MIGRATION.md's "what's gained" list.
"""

from __future__ import annotations

from typing import Any

from PySide6.QtCore import QAbstractTableModel, QModelIndex, Qt

# Mirrors reporting._SEVERITY_RANK's ordering (critical first) - kept as
# a small local copy rather than importing a private helper, since this
# is purely a *sort key* for the Qt view, not scanning logic.
_SEVERITY_SORT_RANK = {"CRITICAL": 0, "HIGH": 1, "MODERATE": 2, "MEDIUM": 2, "LOW": 3, "NONE": 4}


def _severity_rank(level: str | None) -> int:
    return _SEVERITY_SORT_RANK.get((level or "").upper(), 5)


class FindingsTableModel(QAbstractTableModel):
    """One row per build_finding_rows() entry. Affected Files shows a
    short summary ("outer-app.jar" or "3 files") with the full list
    available via Qt.ToolTipRole (hover) and a dedicated accessor the
    view uses to show a details popup on double-click - the desktop
    equivalent of the web UI's <details> disclosure, without needing a
    custom item delegate for something this simple.
    """

    _COLUMNS = ["Severity", "ID", "Component", "Version", "Affected Files", "Summary", "Source", "Confidence"]

    def __init__(self, rows: list[dict[str, Any]] | None = None, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict[str, Any]] = rows or []

    def set_rows(self, rows: list[dict[str, Any]]) -> None:
        self.beginResetModel()
        self._rows = rows
        self.endResetModel()

    def row_data(self, row: int) -> dict[str, Any]:
        return self._rows[row]

    def affected_files(self, row: int) -> list[str]:
        return self._rows[row].get("relativePaths", [])

    # -- QAbstractTableModel ----------------------------------------------

    def rowCount(self, parent: QModelIndex = QModelIndex()) -> int:
        return 0 if parent.isValid() else len(self._rows)

    def columnCount(self, parent: QModelIndex = QModelIndex()) -> int:
        return 0 if parent.isValid() else len(self._COLUMNS)

    def headerData(self, section: int, orientation: Qt.Orientation, role: int = Qt.DisplayRole) -> Any:
        if orientation == Qt.Horizontal and role == Qt.DisplayRole:
            return self._COLUMNS[section]
        return None

    def data(self, index: QModelIndex, role: int = Qt.DisplayRole) -> Any:
        if not index.isValid():
            return None
        row = self._rows[index.row()]
        col = index.column()

        if role == Qt.DisplayRole:
            return self._display(row, col)
        if role == Qt.ToolTipRole and col == 4:
            paths = row.get("relativePaths", [])
            return "\n".join(paths) if paths else None
        if role == Qt.UserRole:
            # Sort key, not display text - QSortFilterProxyModel's default
            # sort compares Qt.DisplayRole (alphabetical, which puts HIGH
            # before CRITICAL); a severity-aware view sorts on this role
            # instead for the Severity column specifically.
            if col == 0:
                return _severity_rank(row.get("severityLevel"))
            return self._display(row, col)
        return None

    def _display(self, row: dict[str, Any], col: int) -> str:
        if col == 0:
            return row.get("severityLevel") or "UNKNOWN"
        if col == 1:
            return row.get("id") or ""
        if col == 2:
            return row.get("product") or ""
        if col == 3:
            return row.get("version") or ""
        if col == 4:
            paths = row.get("relativePaths", [])
            if not paths:
                return "—"
            if len(paths) == 1:
                return paths[0]
            return f"{len(paths)} files"
        if col == 5:
            return row.get("summary") or ""
        if col == 6:
            return row.get("source") or ""
        if col == 7:
            return row.get("confidence") or ""
        return ""
