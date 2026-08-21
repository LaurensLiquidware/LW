import shutil
from pathlib import Path

from main_window import MainWindow
from recent_scans_store import RecentScansStore

FIXTURE = Path(__file__).parent.parent.parent / "stage2-resolve" / "tests" / "fixtures" / "sample.inventory.json"


def test_main_window_constructs_with_empty_store(qtbot, tmp_path):
    store = RecentScansStore(tmp_path / "recent-scans.json")
    window = MainWindow(store=store)
    qtbot.addWidget(window)

    assert window.table.rowCount() == 0


def test_main_window_shows_persisted_entries(qtbot, tmp_path):
    store = RecentScansStore(tmp_path / "recent-scans.json")
    store.add("App.vhdx", "/out/app")

    window = MainWindow(store=store)
    qtbot.addWidget(window)

    assert window.table.rowCount() == 1
    assert window.table.item(0, 1).text() == "App.vhdx"
    assert window.table.item(0, 2).text() == "queued"


def test_severity_summary_shows_dash_when_not_done():
    from main_window import _severity_summary
    assert _severity_summary({"status": "stage2", "severity_counts": None}) == "—"


def test_severity_summary_shows_no_vuln_data_when_done_without_counts():
    from main_window import _severity_summary
    assert _severity_summary({"status": "done", "severity_counts": None}) == "no vuln data"


def test_severity_summary_formats_counts():
    from main_window import _severity_summary
    entry = {"status": "done", "severity_counts": {"CRITICAL": 2, "HIGH": 1, "MEDIUM": 0, "LOW": 3}}
    assert _severity_summary(entry) == "C 2  H 1  M 0  L 3"


def test_view_results_for_done_entry_reloads_from_disk(qtbot, tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)

    store = RecentScansStore(tmp_path / "recent-scans.json")
    entry = store.add("sample.vhdx", str(tmp_path))
    store.update(entry["id"], status="done", inventory_path=str(inventory_path))

    window = MainWindow(store=store)
    qtbot.addWidget(window)

    window._handle_action_clicked(store.all()[0])

    assert entry["id"] in window._live_results
    assert window._open_windows  # a ResultsWindow was created and retained
