import shutil
from pathlib import Path

from compare_dialog import CompareDialog

FIXTURE = Path(__file__).parent.parent.parent / "stage2-resolve" / "tests" / "fixtures" / "sample.inventory.json"


def _write_vuln_matches(dir_, vuln_id):
    import json
    vm = {
        "generatedUtc": "2026-08-13T00:00:00Z", "package": {},
        "components": [{
            "relativePath": "a.jar", "identity": {"product": "a", "version": "1.0"},
            "confidence": "exact-purl",
            "vulnerabilities": [{"id": vuln_id, "summary": "x", "severity": [], "severityLevel": "HIGH", "source": "osv"}],
        }],
    }
    (dir_ / "sample.vuln-matches.json").write_text(json.dumps(vm), encoding="utf-8")


def test_compare_missing_fields_shows_error(qtbot):
    dialog = CompareDialog()
    qtbot.addWidget(dialog)

    dialog._run_compare()

    assert "required" in dialog.error_label.text()


def test_compare_bad_directory_shows_diff_error_message(qtbot, tmp_path):
    dialog = CompareDialog()
    qtbot.addWidget(dialog)
    dialog.old_dir_edit.setText(str(tmp_path / "nope"))
    dialog.new_dir_edit.setText(str(tmp_path))

    dialog._run_compare()

    assert "not a directory" in dialog.error_label.text()


def test_compare_success_populates_tables(qtbot, tmp_path):
    old_dir = tmp_path / "old"
    new_dir = tmp_path / "new"
    old_dir.mkdir()
    new_dir.mkdir()
    shutil.copy(FIXTURE, old_dir / "sample.inventory.json")
    shutil.copy(FIXTURE, new_dir / "sample.inventory.json")
    _write_vuln_matches(old_dir, "GHSA-old-only")
    _write_vuln_matches(new_dir, "GHSA-new-only")

    dialog = CompareDialog()
    qtbot.addWidget(dialog)
    dialog.old_dir_edit.setText(str(old_dir))
    dialog.new_dir_edit.setText(str(new_dir))

    dialog._run_compare()

    assert dialog.new_model.rowCount() == 1
    assert dialog.resolved_model.rowCount() == 1
    assert dialog.new_model.row_data(0)["id"] == "GHSA-new-only"
    assert "→" in dialog.summary_label.text()
