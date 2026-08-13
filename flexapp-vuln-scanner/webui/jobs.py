"""Background scan-job orchestration for the web UI's "run a scan" flow:
Stage 1 (PowerShell subprocess, mounts the package and writes an inventory
JSON) then Stage 2 (in-process flexapp_vuln calls - same functions the CLI
uses, so results can never drift from what `resolve`/`report` would produce).

Runs synchronously in a background thread per job, with an in-memory job
registry - this is a local, single-user tool (see webui/README.md), so no
persistence/database is the right amount of infrastructure here, not a
corner cut.
"""

from __future__ import annotations

import json
import re
import shutil
import subprocess
import threading
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import paths  # noqa: F401 - sys.path setup, must run before flexapp_vuln imports

from flexapp_vuln.cli import UnreachableService, package_display_name, resolve_vuln_matches
from flexapp_vuln.coverage import compute_coverage
from flexapp_vuln.cpe_mappings import CpeMappings
from flexapp_vuln.inventory import load_inventory
from flexapp_vuln.pdf_report import render_pdf_report
from flexapp_vuln.reporting import build_finding_rows, render_coverage_report, render_findings
from flexapp_vuln.sbom import build_sbom

STAGE1_SCRIPT = paths.STAGE1_DIR / "Invoke-FlexAppInventory.ps1"
DEFAULT_CACHE_DIR = paths.STAGE2_DIR / "cache"

_WROTE_INVENTORY_RE = re.compile(r"^Wrote (.+\.inventory\.json)", re.MULTILINE)


@dataclass
class ScanJob:
    id: str
    package_path: str
    output_dir: str
    status: str = "queued"  # queued, stage1, stage2, done, error
    log: list[str] = field(default_factory=list)
    error: str | None = None
    created_at: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    result: dict[str, Any] | None = None

    def append_log(self, line: str) -> None:
        for sub in line.splitlines() or [""]:
            self.log.append(sub)


class JobRegistry:
    def __init__(self) -> None:
        self._jobs: dict[str, ScanJob] = {}
        self._lock = threading.Lock()

    def create(self, package_path: str, output_dir: str) -> ScanJob:
        job = ScanJob(id=uuid.uuid4().hex[:12], package_path=package_path, output_dir=output_dir)
        with self._lock:
            self._jobs[job.id] = job
        return job

    def get(self, job_id: str) -> ScanJob | None:
        with self._lock:
            return self._jobs.get(job_id)

    def list_all(self) -> list[ScanJob]:
        with self._lock:
            return sorted(self._jobs.values(), key=lambda j: j.created_at, reverse=True)


REGISTRY = JobRegistry()


def start_scan(package_path: str, output_dir: str, *, nvd_api_key: str | None = None) -> ScanJob:
    job = REGISTRY.create(package_path, output_dir)
    thread = threading.Thread(target=_run_job, args=(job, nvd_api_key), daemon=True)
    thread.start()
    return job


def _run_job(job: ScanJob, nvd_api_key: str | None) -> None:
    try:
        inventory_path = _run_stage1(job)
        _run_stage2(job, inventory_path, nvd_api_key)
        job.status = "done"
    except Exception as exc:  # noqa: BLE001 - surfaced to the UI, never swallowed
        job.status = "error"
        job.error = str(exc)
        job.append_log(f"ERROR: {exc}")


def _run_stage1(job: ScanJob) -> Path:
    job.status = "stage1"
    if not STAGE1_SCRIPT.exists():
        raise RuntimeError(f"Stage 1 script not found at {STAGE1_SCRIPT}")

    pwsh = shutil.which("pwsh")
    if not pwsh:
        raise RuntimeError(
            "pwsh (PowerShell 7) not found on PATH - Stage 1 needs it to mount "
            "the package and run the inventory scan."
        )

    cmd = [
        pwsh, "-NoLogo", "-NoProfile", "-File", str(STAGE1_SCRIPT),
        "-Path", job.package_path, "-OutputDir", job.output_dir,
    ]
    job.append_log(f"$ {' '.join(cmd)}")

    output_lines: list[str] = []
    proc = subprocess.Popen(
        cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, bufsize=1,
    )
    assert proc.stdout is not None
    for line in proc.stdout:
        job.append_log(line)
        output_lines.append(line)
    return_code = proc.wait()
    if return_code != 0:
        raise RuntimeError(f"Stage 1 exited with code {return_code} - see log above")

    match = _WROTE_INVENTORY_RE.search("\n".join(output_lines))
    if not match:
        raise RuntimeError(
            "Stage 1 finished but no '<package>.inventory.json' path was found "
            "in its output - can't proceed to Stage 2"
        )
    return Path(match.group(1).strip())


def _run_stage2(job: ScanJob, inventory_path: Path, nvd_api_key: str | None) -> None:
    job.status = "stage2"
    out_dir = Path(job.output_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    job.append_log(f"Loading inventory: {inventory_path}")
    inventory = load_inventory(inventory_path)
    cpe_mappings = CpeMappings.load()

    job.append_log("Querying OSV.dev + NVD for vulnerability matches...")
    try:
        vuln_matches = resolve_vuln_matches(
            inventory, cache_dir=DEFAULT_CACHE_DIR, cpe_mappings=cpe_mappings, nvd_api_key=nvd_api_key,
        )
    except UnreachableService as exc:
        raise RuntimeError(f"could not reach {exc.host}: {exc.original}") from exc

    out_base = inventory_path.stem.removesuffix(".inventory")
    vuln_matches_path = out_dir / f"{out_base}.vuln-matches.json"
    vuln_matches_path.write_text(json.dumps(vuln_matches, indent=2), encoding="utf-8")
    job.append_log(f"Wrote {vuln_matches_path}")

    write_reports(job, inventory, inventory_path, vuln_matches, out_dir, out_base)


def write_reports(
    job: ScanJob | None,
    inventory: dict[str, Any],
    inventory_path: Path,
    vuln_matches: dict[str, Any] | None,
    out_dir: Path,
    out_base: str,
) -> dict[str, Any]:
    """Writes sbom/coverage/findings/PDF and returns the result summary the
    results page renders. Shared between a fresh scan job and "open an
    existing output directory" (job is None there - no log to append to).
    """
    cpe_mappings = CpeMappings.load()
    package_name = package_display_name(inventory)
    coverage = compute_coverage(inventory)
    sbom = build_sbom(inventory, cpe_mappings=cpe_mappings)
    coverage_md = render_coverage_report(coverage, package_name)
    findings_md = render_findings(vuln_matches, package_name)

    sbom_path = out_dir / f"{out_base}.sbom.cdx.json"
    coverage_path = out_dir / f"{out_base}.coverage-report.md"
    findings_path = out_dir / f"{out_base}.findings.md"
    pdf_path = out_dir / f"{out_base}.report.pdf"

    sbom_path.write_text(json.dumps(sbom, indent=2), encoding="utf-8")
    coverage_path.write_text(coverage_md, encoding="utf-8")
    findings_path.write_text(findings_md, encoding="utf-8")

    package = inventory.get("package", {})
    package_meta = {**package, **(package.get("flexAppXml") or {})}
    render_pdf_report(
        pdf_path, package_name=package_name, package_meta=package_meta,
        coverage=coverage, vuln_matches=vuln_matches,
    )

    if job is not None:
        for path in (sbom_path, coverage_path, findings_path, pdf_path):
            job.append_log(f"Wrote {path}")

    all_rows = build_finding_rows(vuln_matches) if vuln_matches is not None else []
    confirmed_rows = [r for r in all_rows if r["confidence"] in ("exact-purl", "mapped-cpe")]
    heuristic_rows = [r for r in all_rows if r["confidence"] == "heuristic"]

    result = {
        "package_name": package_name,
        "coverage": coverage,
        "confirmed_rows": confirmed_rows,
        "heuristic_rows": heuristic_rows,
        "has_vuln_matches": vuln_matches is not None,
        "inventory_path": str(inventory_path),
        "output_dir": str(out_dir),
        "files": {
            "sbom": str(sbom_path),
            "coverage_report": str(coverage_path),
            "findings": str(findings_path),
            "pdf": str(pdf_path),
        },
    }
    if job is not None:
        job.result = result
    return result


def load_existing_result(inventory_path: Path) -> dict[str, Any]:
    """Rebuilds a results view from an already-completed scan's inventory
    JSON, reusing a sibling <base>.vuln-matches.json if one exists - same
    idea as `report --vuln-matches`, just without needing to re-run
    `resolve` (no network calls).
    """
    inventory = load_inventory(inventory_path)
    out_base = inventory_path.stem.removesuffix(".inventory")
    out_dir = inventory_path.parent

    vuln_matches_path = out_dir / f"{out_base}.vuln-matches.json"
    vuln_matches = None
    if vuln_matches_path.exists():
        vuln_matches = json.loads(vuln_matches_path.read_text(encoding="utf-8"))

    return write_reports(None, inventory, inventory_path, vuln_matches, out_dir, out_base)
