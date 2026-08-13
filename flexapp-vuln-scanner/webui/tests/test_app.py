import html
import re
import shutil
import urllib.parse
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
    assert b"Run a New Scan" in resp.data


def test_footer_shows_version_on_every_page():
    # Sparks Tool Project Review Checklist §6: version must be visible
    # without reading source, on the tool's normal interface.
    import flexapp_vuln
    resp = client().get("/")
    assert f"v{flexapp_vuln.__version__}".encode() in resp.data


def test_license_route_serves_spark_license_pdf():
    resp = client().get("/license")
    assert resp.status_code == 200
    assert resp.data.startswith(b"%PDF-")


def test_sbom_route_serves_tool_bom_when_present():
    resp = client().get("/sbom")
    # bom.cdx.json ships at the repo root as of this checklist pass - if
    # it's ever missing (e.g. a stripped-down checkout), 404 is correct
    # rather than a crash.
    assert resp.status_code in (200, 404)
    if resp.status_code == 200:
        import json
        data = json.loads(resp.data)
        assert data["bomFormat"] == "CycloneDX"
        assert data["specVersion"] == "1.6"


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


def test_result_page_links_cve_id_to_nvd(tmp_path):
    import json as json_module

    shutil.copy(FIXTURE, tmp_path / "sample.inventory.json")
    vuln_matches = {
        "generatedUtc": "2026-08-13T00:00:00Z",
        "package": {},
        "components": [
            {
                "relativePath": "Program Files\\App\\lib\\libcrypto-1_1.dll",
                "identity": {"product": "OpenSSL", "version": "1.1.1w"},
                "purl": None,
                "cpe": "cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*",
                "confidence": "mapped-cpe",
                "vulnerabilities": [
                    {"id": "CVE-2023-0001", "summary": "x", "severity": [], "severityLevel": "HIGH", "source": "nvd"}
                ],
            },
        ],
    }
    (tmp_path / "sample.vuln-matches.json").write_text(json_module.dumps(vuln_matches), encoding="utf-8")

    resp = client().post("/open", data={"dir_path": str(tmp_path)}, follow_redirects=True)

    assert resp.status_code == 200
    assert b'href="https://nvd.nist.gov/vuln/detail/CVE-2023-0001"' in resp.data


def _hrefs(page_html: str) -> list[str]:
    # Jinja HTML-escapes "&" to "&amp;" inside href="..." attributes -
    # unescape before treating these as real URLs to parse query params from.
    return [html.unescape(h) for h in re.findall(r'href="([^"]+)"', page_html)]


def _query(url: str) -> dict[str, str]:
    parsed = urllib.parse.urlparse(url)
    return {k: v[0] for k, v in urllib.parse.parse_qs(parsed.query).items()}


def test_browse_navigation_links_carry_other_field_values(tmp_path):
    # Regression test: browsing for output_dir after already picking a
    # package_path must not clobber it - every link on the browse page
    # (drives, up, subfolder nav) needs to carry the other fields' current
    # values through, not just the one being edited.
    (tmp_path / "sub").mkdir()
    package_value = "C:\\some\\package.vhdx"

    resp = client().get("/browse", query_string={
        "target": "output_dir", "path": str(tmp_path), "package_path": package_value,
    })
    html = resp.data.decode()

    browse_hrefs = [h for h in _hrefs(html) if h.startswith("/browse")]
    assert browse_hrefs, "expected at least one /browse navigation link (e.g. into 'sub')"
    for href in browse_hrefs:
        assert _query(href).get("package_path") == package_value


def test_browse_select_folder_link_preserves_other_fields(tmp_path):
    package_value = "C:\\some\\package.vhdx"

    resp = client().get("/browse", query_string={
        "target": "output_dir", "path": str(tmp_path), "package_path": package_value,
    })
    html = resp.data.decode()

    select_hrefs = [h for h in _hrefs(html) if h.startswith("/?")]
    assert select_hrefs, "expected a 'select this folder' link back to /"
    query = _query(select_hrefs[0])
    assert query.get("package_path") == package_value
    assert query.get("output_dir") == str(tmp_path)


def test_browse_unknown_target_400():
    resp = client().get("/browse", query_string={"target": "nonsense"})
    assert resp.status_code == 400


def test_browse_no_path_shows_drives():
    resp = client().get("/browse", query_string={"target": "output_dir"})
    assert resp.status_code == 200
    assert b"Drives" in resp.data


def test_browse_lists_subdirectories_and_offers_select_for_dir_mode(tmp_path):
    (tmp_path / "child").mkdir()

    resp = client().get("/browse", query_string={"target": "output_dir", "path": str(tmp_path)})

    assert resp.status_code == 200
    html = resp.data.decode()
    assert "child" in html
    assert "Select This Folder" in html


def test_browse_file_mode_only_lists_package_extensions(tmp_path):
    (tmp_path / "package.vhdx").write_text("x")
    (tmp_path / "notes.txt").write_text("x")

    resp = client().get("/browse", query_string={"target": "package_path", "path": str(tmp_path)})

    html = resp.data.decode()
    assert "package.vhdx" in html
    assert "notes.txt" not in html
    # File-picker mode has no "select this folder" affordance.
    assert "Select This Folder" not in html


def test_browse_nonexistent_path_returns_400(tmp_path):
    resp = client().get("/browse", query_string={"target": "output_dir", "path": str(tmp_path / "nope")})
    assert resp.status_code == 400


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
