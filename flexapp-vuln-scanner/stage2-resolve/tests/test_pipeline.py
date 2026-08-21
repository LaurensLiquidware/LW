import json
import shutil
from dataclasses import dataclass, field
from pathlib import Path
from unittest.mock import patch

import flexapp_vuln.pipeline as pipeline

FIXTURE = Path(__file__).parent / "fixtures" / "sample.inventory.json"


@dataclass
class FakeSink:
    """Minimal stand-in for pipeline.ProgressSink - both webui's ScanJob
    and the desktop app's Qt-signal adapter satisfy the same shape."""
    status: str = "queued"
    log: list = field(default_factory=list)
    progress_phase: str | None = None
    progress_done: int = 0
    progress_total: int = 0

    def append_log(self, line: str) -> None:
        self.log.append(line)

    def set_progress(self, phase: str, done: int, total: int) -> None:
        self.progress_phase = phase
        self.progress_done = done
        self.progress_total = total


def test_run_stage1_missing_script_raises():
    sink = FakeSink()
    with patch.object(pipeline, "STAGE1_SCRIPT", Path("/nonexistent/Invoke-FlexAppInventory.ps1")):
        try:
            pipeline.run_stage1(sink, "whatever.vhdx", "/tmp/out")
            assert False, "expected RuntimeError"
        except RuntimeError as exc:
            assert "not found" in str(exc)
    assert sink.status == "stage1"


def test_run_stage2_wires_on_progress_and_returns_result(tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)
    sink = FakeSink()

    def fake_resolve(inventory, *, cache_dir, cpe_mappings, nvd_api_key, on_progress):
        on_progress("nvd", 1, 3)
        on_progress("nvd", 3, 3)
        return {"generatedUtc": "2026-08-13T00:00:00Z", "package": {}, "components": []}

    with patch.object(pipeline, "resolve_vuln_matches", side_effect=fake_resolve):
        result = pipeline.run_stage2(sink, inventory_path, str(tmp_path), None)

    assert sink.progress_phase == "nvd"
    assert sink.progress_done == 3
    assert sink.progress_total == 3
    assert result["has_vuln_matches"] is True


def test_run_stage2_surfaces_unreachable_service_as_runtime_error(tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)
    sink = FakeSink()

    with patch.object(pipeline, "resolve_vuln_matches", side_effect=RuntimeError("boom")):
        try:
            pipeline.run_stage2(sink, inventory_path, str(tmp_path), None)
            assert False, "expected RuntimeError"
        except RuntimeError as exc:
            assert "boom" in str(exc)


def test_load_existing_result_no_vuln_matches(tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)

    result = pipeline.load_existing_result(inventory_path)

    assert result["has_vuln_matches"] is False
    assert result["confirmed_rows"] == []
    for kind, path_str in result["files"].items():
        assert Path(path_str).is_file(), f"{kind} not written"


def test_load_existing_result_with_vuln_matches(tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)
    vuln_matches = {
        "generatedUtc": "2026-08-13T00:00:00Z", "package": {},
        "components": [{
            "relativePath": "a.jar", "identity": {"product": "a", "version": "1.0"},
            "purl": "pkg:maven/a/a@1.0", "confidence": "exact-purl",
            "vulnerabilities": [{"id": "GHSA-aaaa", "summary": "Bad", "severity": [], "severityLevel": "HIGH", "source": "osv"}],
        }],
    }
    (tmp_path / "sample.vuln-matches.json").write_text(json.dumps(vuln_matches), encoding="utf-8")

    result = pipeline.load_existing_result(inventory_path)

    assert result["has_vuln_matches"] is True
    assert result["confirmed_rows"][0]["id"] == "GHSA-aaaa"
    assert result["confirmed_rows"][0]["relativePaths"] == ["a.jar"]


def test_load_diff_raises_diff_error_for_non_directory(tmp_path):
    try:
        pipeline.load_diff(tmp_path / "nope", tmp_path)
        assert False, "expected DiffError"
    except pipeline.DiffError as exc:
        assert "not a directory" in str(exc)


def test_load_diff_reports_new_and_resolved(tmp_path):
    old_dir = tmp_path / "old"
    new_dir = tmp_path / "new"
    old_dir.mkdir()
    new_dir.mkdir()
    shutil.copy(FIXTURE, old_dir / "sample.inventory.json")
    shutil.copy(FIXTURE, new_dir / "sample.inventory.json")

    def write(dir_, vuln_id):
        vm = {
            "generatedUtc": "2026-08-13T00:00:00Z", "package": {},
            "components": [{
                "relativePath": "a.jar", "identity": {"product": "a", "version": "1.0"},
                "confidence": "exact-purl",
                "vulnerabilities": [{"id": vuln_id, "summary": "x", "severity": [], "severityLevel": "HIGH", "source": "osv"}],
            }],
        }
        (dir_ / "sample.vuln-matches.json").write_text(json.dumps(vm), encoding="utf-8")

    write(old_dir, "GHSA-old-only")
    write(new_dir, "GHSA-new-only")

    diff = pipeline.load_diff(old_dir, new_dir)

    assert [r["id"] for r in diff["new_findings"]] == ["GHSA-new-only"]
    assert [r["id"] for r in diff["resolved_findings"]] == ["GHSA-old-only"]
