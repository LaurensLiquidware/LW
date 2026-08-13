import json
import shutil
import time
from pathlib import Path
from unittest.mock import patch

import jobs

FIXTURE = Path(__file__).parent.parent.parent / "stage2-resolve" / "tests" / "fixtures" / "sample.inventory.json"


def test_run_stage1_missing_script_raises():
    fake_job = jobs.ScanJob(id="x", package_path="whatever.vhdx", output_dir="/tmp/out")
    with patch.object(jobs, "STAGE1_SCRIPT", Path("/nonexistent/Invoke-FlexAppInventory.ps1")):
        try:
            jobs._run_stage1(fake_job)
            assert False, "expected RuntimeError"
        except RuntimeError as exc:
            assert "not found" in str(exc)
    assert fake_job.status == "stage1"


def test_run_refresh_job_skips_stage1_and_writes_reports(tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)
    fake_job = jobs.ScanJob(id="x", package_path="(refresh) unused", output_dir=str(tmp_path))

    with patch.object(jobs, "resolve_vuln_matches", return_value={"generatedUtc": "2026-08-13T00:00:00Z", "package": {}, "components": []}) as mock_resolve:
        jobs._run_refresh_job(fake_job, inventory_path, None)

    mock_resolve.assert_called_once()
    assert fake_job.status == "done"
    assert fake_job.error is None
    assert fake_job.result is not None
    assert fake_job.result["has_vuln_matches"] is True
    assert any("Stage 1 not re-run" in line for line in fake_job.log)


def test_run_refresh_job_surfaces_errors(tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)
    fake_job = jobs.ScanJob(id="x", package_path="(refresh) unused", output_dir=str(tmp_path))

    with patch.object(jobs, "resolve_vuln_matches", side_effect=RuntimeError("boom")):
        jobs._run_refresh_job(fake_job, inventory_path, None)

    assert fake_job.status == "error"
    assert fake_job.error == "boom"


def test_start_refresh_runs_in_background_and_completes(tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)

    with patch.object(jobs, "resolve_vuln_matches", return_value=None):
        job = jobs.start_refresh(str(inventory_path), str(tmp_path))

        # Poll briefly for the background thread to finish rather than assuming timing.
        deadline = time.time() + 5
        while job.status not in ("done", "error") and time.time() < deadline:
            time.sleep(0.05)

    assert job.status == "done"
    assert job.result is not None


def test_load_existing_result_no_vuln_matches(tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)

    result = jobs.load_existing_result(inventory_path)

    assert result["has_vuln_matches"] is False
    assert result["confirmed_rows"] == []
    assert result["heuristic_rows"] == []
    assert result["coverage"]["candidateComponents"] == 3
    for kind, path_str in result["files"].items():
        assert Path(path_str).is_file(), f"{kind} not written"


def test_load_existing_result_splits_confirmed_and_heuristic(tmp_path):
    inventory_path = tmp_path / "sample.inventory.json"
    shutil.copy(FIXTURE, inventory_path)

    vuln_matches = {
        "generatedUtc": "2026-08-13T00:00:00Z",
        "package": {},
        "components": [
            {
                "relativePath": "a.jar",
                "identity": {"product": "a", "version": "1.0"},
                "purl": "pkg:maven/a/a@1.0",
                "cpe": None,
                "confidence": "exact-purl",
                "vulnerabilities": [
                    {"id": "GHSA-aaaa", "summary": "Bad", "severity": [], "severityLevel": "HIGH", "source": "osv"}
                ],
            },
            {
                "relativePath": "b.exe",
                "identity": {"product": "b", "version": "2.0"},
                "purl": None,
                "cpe": "cpe:2.3:a:b:b:2.0:*:*:*:*:*:*:*",
                "confidence": "heuristic",
                "vulnerabilities": [
                    {"id": "CVE-2020-0001", "summary": "Maybe", "severity": [], "severityLevel": "LOW", "source": "nvd"}
                ],
            },
        ],
    }
    vuln_matches_path = tmp_path / "sample.vuln-matches.json"
    vuln_matches_path.write_text(json.dumps(vuln_matches), encoding="utf-8")

    result = jobs.load_existing_result(inventory_path)

    assert result["has_vuln_matches"] is True
    assert len(result["confirmed_rows"]) == 1
    assert result["confirmed_rows"][0]["id"] == "GHSA-aaaa"
    assert len(result["heuristic_rows"]) == 1
    assert result["heuristic_rows"][0]["id"] == "CVE-2020-0001"

    pdf_path = Path(result["files"]["pdf"])
    assert pdf_path.read_bytes().startswith(b"%PDF-")
