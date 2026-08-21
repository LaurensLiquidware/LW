"""Web-UI-specific scan-job bookkeeping: an in-memory job registry the
`/scan/<id>/poll` route reads, backed by threading.Thread. The actual
scan orchestration (Stage 1 subprocess, Stage 2 resolve, writing reports,
loading an existing scan, diffing two scans) lives in
flexapp_vuln.pipeline, shared with the native desktop app - nothing in
this file duplicates that logic, it only adapts ScanJob to the pipeline's
duck-typed progress-sink interface (append_log/status/set_progress) and
runs it on a background thread for the web UI's polling model.

This is a local, single-user tool (see webui/README.md), so an in-memory
registry that resets on restart is the right amount of infrastructure
here, not a corner cut.
"""

from __future__ import annotations

import threading
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import paths  # noqa: F401 - sys.path setup, must run before flexapp_vuln imports

from flexapp_vuln.pipeline import (
    DEFAULT_CACHE_DIR,
    STAGE1_SCRIPT,
    DiffError,
    load_diff,
    load_existing_result,
    run_stage1,
    run_stage2,
    write_reports,
)

__all__ = [
    "ScanJob", "JobRegistry", "REGISTRY", "start_scan", "start_refresh",
    "write_reports", "load_existing_result", "load_diff", "DiffError",
    "STAGE1_SCRIPT", "DEFAULT_CACHE_DIR",
]


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
    # Set during stage2's OSV/NVD query loops - see resolve_vuln_matches's
    # on_progress. phase is "osv" or "nvd"; None before stage2 starts.
    progress_phase: str | None = None
    progress_done: int = 0
    progress_total: int = 0

    def append_log(self, line: str) -> None:
        for sub in line.splitlines() or [""]:
            self.log.append(sub)

    def set_progress(self, phase: str, done: int, total: int) -> None:
        self.progress_phase = phase
        self.progress_done = done
        self.progress_total = total


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
        inventory_path = run_stage1(job, job.package_path, job.output_dir)
        job.result = run_stage2(job, inventory_path, job.output_dir, nvd_api_key)
        job.status = "done"
    except Exception as exc:  # noqa: BLE001 - surfaced to the UI, never swallowed
        job.status = "error"
        job.error = str(exc)
        job.append_log(f"ERROR: {exc}")


def start_refresh(inventory_path: str, output_dir: str, *, nvd_api_key: str | None = None) -> ScanJob:
    """Re-runs just the OSV/NVD matching + report step against an inventory
    JSON a scan already produced - no Stage 1 VHDX re-mount needed. NVD/OSV
    data changes daily, so this is the way to pick up newly-published CVEs
    against a package without re-scanning it from scratch.
    """
    job = REGISTRY.create(f"(refresh) {inventory_path}", output_dir)
    thread = threading.Thread(target=_run_refresh_job, args=(job, Path(inventory_path), nvd_api_key), daemon=True)
    thread.start()
    return job


def _run_refresh_job(job: ScanJob, inventory_path: Path, nvd_api_key: str | None) -> None:
    try:
        job.append_log(f"Refreshing vulnerability matches for {inventory_path} (Stage 1 not re-run)")
        job.result = run_stage2(job, inventory_path, job.output_dir, nvd_api_key)
        job.status = "done"
    except Exception as exc:  # noqa: BLE001 - surfaced to the UI, never swallowed
        job.status = "error"
        job.error = str(exc)
        job.append_log(f"ERROR: {exc}")
