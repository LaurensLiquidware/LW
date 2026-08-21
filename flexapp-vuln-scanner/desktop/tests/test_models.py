from PySide6.QtCore import Qt

from models import FindingsTableModel


def _row(**kwargs):
    base = {
        "severityLevel": "HIGH", "id": "CVE-1", "product": "a", "version": "1.0",
        "relativePaths": ["a.jar"], "summary": "x", "source": "nvd", "confidence": "exact-purl",
    }
    base.update(kwargs)
    return base


def test_row_and_column_counts():
    model = FindingsTableModel([_row(), _row()])
    assert model.rowCount() == 2
    assert model.columnCount() == 8


def test_affected_files_single_shows_filename_directly():
    model = FindingsTableModel([_row(relativePaths=["a.jar"])])
    index = model.index(0, 4)
    assert model.data(index, Qt.DisplayRole) == "a.jar"


def test_affected_files_multiple_shows_count():
    model = FindingsTableModel([_row(relativePaths=["a.jar", "b.jar", "c.jar"])])
    index = model.index(0, 4)
    assert model.data(index, Qt.DisplayRole) == "3 files"


def test_affected_files_tooltip_lists_every_path():
    model = FindingsTableModel([_row(relativePaths=["a.jar", "b.jar"])])
    index = model.index(0, 4)
    assert model.data(index, Qt.ToolTipRole) == "a.jar\nb.jar"


def test_affected_files_accessor_returns_full_list():
    model = FindingsTableModel([_row(relativePaths=["a.jar", "b.jar"])])
    assert model.affected_files(0) == ["a.jar", "b.jar"]


def test_no_affected_files_shows_dash():
    model = FindingsTableModel([_row(relativePaths=[])])
    index = model.index(0, 4)
    assert model.data(index, Qt.DisplayRole) == "—"


def test_severity_sort_role_ranks_critical_first():
    model = FindingsTableModel([_row(severityLevel="LOW"), _row(severityLevel="CRITICAL")])
    low_rank = model.data(model.index(0, 0), Qt.UserRole)
    critical_rank = model.data(model.index(1, 0), Qt.UserRole)
    assert critical_rank < low_rank


def test_header_data():
    model = FindingsTableModel([])
    assert model.headerData(0, Qt.Horizontal, Qt.DisplayRole) == "Severity"
    assert model.headerData(4, Qt.Horizontal, Qt.DisplayRole) == "Affected Files"


def test_set_rows_resets_model():
    model = FindingsTableModel([_row()])
    model.set_rows([_row(), _row()])
    assert model.rowCount() == 2
