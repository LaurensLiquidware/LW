import shutil
from pathlib import Path

import app as flask_app_module
import jobs

FIXTURE = Path(__file__).parent.parent.parent / "stage2-resolve" / "tests" / "fixtures" / "sample.inventory.json"


def client():
    flask_app_module.app.testing = True
    return flask_app_module.app.test_client()


def test_index_loads():
    resp = client().get("/")
    assert resp.status_code == 200
    assert b"Run a new scan" in resp.data


def test_new_scan_missing_fields_returns_400():
    resp = client().post("/scan", data={"package_path": "", "output_dir": ""})
    assert resp.status_code == 400
    assert b"required" in resp.data


def test_scan_status_unknown_job_404():
    resp = client().get("/scan/does-not-exist")
    assert resp.status_code == 404


def test_scan_poll_unknown_job_404():
    resp = client().get("/scan/does-not-exist/poll")
    assert resp.status_code == 404


def test_open_directory_missing_dir_returns_400(tmp_path):
    resp = client().post("/open", data={"dir_path": str(tmp_path / "nope")})
    assert resp.status_code == 400
    assert b"not a directory" in resp.data


def test_open_directory_no_inventory_returns_400(tmp_path):
    resp = client().post("/open", data={"dir_path": str(tmp_path)})
    assert resp.status_code == 400
    assert b"No *.inventory.json" in resp.data


def test_open_directory_single_inventory_redirects_to_results(tmp_path):
    shutil.copy(FIXTURE, tmp_path / "sample.inventory.json")

    resp = client().post("/open", data={"dir_path": str(tmp_path)}, follow_redirects=True)

    assert resp.status_code == 200
    assert b"Resolution Coverage" in resp.data
    assert b"Vulnerability Findings" in resp.data
    # No vuln-matches.json alongside the fixture - must say so, not look empty.
    assert b"not the same thing as" in resp.data


def test_download_unknown_job_and_kind_404(tmp_path):
    resp = client().get("/download/job/does-not-exist/pdf")
    assert resp.status_code == 404

    shutil.copy(FIXTURE, tmp_path / "sample.inventory.json")
    result = jobs.load_existing_result(tmp_path / "sample.inventory.json")
    job = jobs.REGISTRY.create("fake.vhdx", str(tmp_path))
    job.result = result
    job.status = "done"

    resp = client().get(f"/download/job/{job.id}/pdf")
    assert resp.status_code == 200
    assert resp.data.startswith(b"%PDF-")

    resp = client().get(f"/download/job/{job.id}/not-a-real-kind")
    assert resp.status_code == 404
