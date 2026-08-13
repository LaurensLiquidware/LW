"""Local web UI for the FlexApp vulnerability scanner pipeline.

Runs Stage 1 (PowerShell, package mount + inventory) and Stage 2
(flexapp_vuln, in-process) from a browser instead of two separate manual
commands, and lets you browse any past scan's coverage/findings/SBOM/PDF
output. Local, single-user tool - see README.md before exposing this to
anything but 127.0.0.1.
"""

from __future__ import annotations

import uuid
from pathlib import Path

from flask import Flask, abort, jsonify, redirect, render_template, request, send_file, url_for

import jobs
import paths  # noqa: F401 - sys.path setup, must run before flexapp_vuln imports

app = Flask(__name__)

# Ephemeral store for "open an existing output directory" results, keyed by a
# random id so download links never carry a raw filesystem path from the
# client (see README.md's "Security" note) - same shape as jobs.REGISTRY but
# for scans this process didn't run itself.
_OPENED: dict[str, dict] = {}

_DOWNLOAD_KINDS = {
    "sbom": "sbom.cdx.json",
    "coverage_report": "coverage-report.md",
    "findings": "findings.md",
    "pdf": "report.pdf",
}


@app.route("/")
def index():
    return render_template("index.html", jobs=jobs.REGISTRY.list_all())


@app.route("/scan", methods=["POST"])
def new_scan():
    package_path = request.form.get("package_path", "").strip()
    output_dir = request.form.get("output_dir", "").strip()
    nvd_api_key = request.form.get("nvd_api_key", "").strip() or None

    if not package_path or not output_dir:
        return render_template(
            "index.html", jobs=jobs.REGISTRY.list_all(),
            form_error="Both a package path and an output directory are required.",
        ), 400

    job = jobs.start_scan(package_path, output_dir, nvd_api_key=nvd_api_key)
    return redirect(url_for("scan_status", job_id=job.id))


@app.route("/scan/<job_id>")
def scan_status(job_id: str):
    job = jobs.REGISTRY.get(job_id)
    if job is None:
        abort(404)
    return render_template("scan.html", job=job)


@app.route("/scan/<job_id>/poll")
def scan_poll(job_id: str):
    job = jobs.REGISTRY.get(job_id)
    if job is None:
        abort(404)
    return jsonify({
        "status": job.status,
        "log": job.log,
        "error": job.error,
        "done": job.status in ("done", "error"),
    })


@app.route("/scan/<job_id>/results")
def scan_results(job_id: str):
    job = jobs.REGISTRY.get(job_id)
    if job is None:
        abort(404)
    if job.result is None:
        return redirect(url_for("scan_status", job_id=job_id))
    return render_template("result.html", result=job.result, download_route="download_job", download_id=job_id)


@app.route("/download/job/<id>/<kind>")
def download_job(id: str, kind: str):
    job = jobs.REGISTRY.get(id)
    if job is None or job.result is None or kind not in _DOWNLOAD_KINDS:
        abort(404)
    path = Path(job.result["files"][kind])
    if not path.is_file():
        abort(404)
    return send_file(path, as_attachment=True)


@app.route("/open", methods=["POST"])
def open_directory():
    dir_path = Path(request.form.get("dir_path", "").strip())
    if not dir_path.is_dir():
        return render_template(
            "index.html", jobs=jobs.REGISTRY.list_all(),
            open_error=f"'{dir_path}' is not a directory.",
        ), 400

    inventory_files = sorted(dir_path.glob("*.inventory.json"))
    if not inventory_files:
        return render_template(
            "index.html", jobs=jobs.REGISTRY.list_all(),
            open_error=f"No *.inventory.json file found directly under '{dir_path}'.",
        ), 400

    opened_ids = []
    for inventory_path in inventory_files:
        result = jobs.load_existing_result(inventory_path)
        open_id = uuid.uuid4().hex[:12]
        _OPENED[open_id] = result
        opened_ids.append(open_id)

    if len(opened_ids) == 1:
        return redirect(url_for("open_results", open_id=opened_ids[0]))
    return render_template(
        "index.html", jobs=jobs.REGISTRY.list_all(),
        opened_choices=[(oid, _OPENED[oid]["package_name"]) for oid in opened_ids],
    )


@app.route("/open/<open_id>/results")
def open_results(open_id: str):
    result = _OPENED.get(open_id)
    if result is None:
        abort(404)
    return render_template("result.html", result=result, download_route="download_open", download_id=open_id)


@app.route("/download/open/<id>/<kind>")
def download_open(id: str, kind: str):
    result = _OPENED.get(id)
    if result is None or kind not in _DOWNLOAD_KINDS:
        abort(404)
    path = Path(result["files"][kind])
    if not path.is_file():
        abort(404)
    return send_file(path, as_attachment=True)


if __name__ == "__main__":
    # 127.0.0.1 only, deliberately - see README.md's "Security" note before
    # changing this to 0.0.0.0. This process can run arbitrary local
    # PowerShell/Python and read arbitrary paths you type into it; it is not
    # meant to be reachable from anywhere but the machine it runs on.
    app.run(host="127.0.0.1", port=5000, debug=False)
