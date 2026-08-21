import shutil
from pathlib import Path
from unittest.mock import patch

import flexapp_vuln.pipeline as pipeline
from scan_worker import ScanWorker

FIXTURE = Path(__file__).parent.parent.parent / "stage2-resolve" / "tests" / "fixtures" / "sample.inventory.json"


def test_scan_worker_status_property_emits_signal(qtbot):
    worker = ScanWorker("pkg.vhdx", "/tmp/out")
    statuses = []
    worker.status_changed.connect(statuses.append)

    worker.status = "stage1"
    worker.status = "stage2"

    assert statuses == ["stage1", "stage2"]
    assert worker.status == "stage2"


def test_scan_worker_append_log_splits_multiline(qtbot):
    worker = ScanWorker("pkg.vhdx", "/tmp/out")
    lines = []
    worker.log_line.connect(lines.append)

    worker.append_log("line one\nline two")

    assert lines == ["line one", "line two"]


def test_scan_worker_refresh_mode_skips_stage1_and_succeeds(qtbot, tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)

    worker = ScanWorker(
        "", str(tmp_path), refresh_inventory_path=str(inventory_path),
    )
    logs = []
    worker.log_line.connect(logs.append)

    with patch.object(pipeline, "resolve_vuln_matches", return_value=None):
        with qtbot.waitSignal(worker.succeeded, timeout=10000) as blocker:
            worker.start()

    result = blocker.args[0]
    assert result["has_vuln_matches"] is False
    assert any("Stage 1 not re-run" in line for line in logs)
    assert worker.status == "done"


def test_scan_worker_emits_failed_on_error(qtbot, tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)

    worker = ScanWorker("", str(tmp_path), refresh_inventory_path=str(inventory_path))

    with patch.object(pipeline, "resolve_vuln_matches", side_effect=RuntimeError("boom")):
        with qtbot.waitSignal(worker.failed, timeout=10000) as blocker:
            worker.start()

    assert blocker.args == ["boom"]
    assert worker.status == "error"


def test_scan_worker_progress_changed_signal(qtbot, tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)
    worker = ScanWorker("", str(tmp_path), refresh_inventory_path=str(inventory_path))
    progress_events = []
    worker.progress_changed.connect(lambda phase, done, total: progress_events.append((phase, done, total)))

    def fake_resolve(inventory, *, cache_dir, cpe_mappings, nvd_api_key, on_progress):
        on_progress("nvd", 1, 2)
        on_progress("nvd", 2, 2)
        return None

    with patch.object(pipeline, "resolve_vuln_matches", side_effect=fake_resolve):
        with qtbot.waitSignal(worker.succeeded, timeout=10000):
            worker.start()

    assert progress_events == [("nvd", 1, 2), ("nvd", 2, 2)]
