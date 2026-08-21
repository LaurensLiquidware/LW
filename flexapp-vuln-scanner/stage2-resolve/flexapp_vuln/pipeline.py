"""Shared scan orchestration: Stage 1 (PowerShell subprocess, mounts the
package and writes an inventory JSON) then Stage 2 (in-process
flexapp_vuln calls - same functions the CLI uses, so results can never
drift from what `resolve`/`report` would produce), plus loading an
existing scan's results and diffing two scans.

Deliberately front-end agnostic: both the Flask web UI (webui/jobs.py)
and the native desktop app (desktop/) call these same functions, so the
actual scanning behavior can't drift between the two front ends. A
front end supplies its own "sink" for progress reporting - any object
with an `append_log(line)` method, a settable `status` attribute, and a
`set_progress(phase, done, total)` method. The web UI's ScanJob and the
desktop app's Qt-signal-backed adapter both satisfy this without either
one importing the other - this module has no Flask and no Qt import.
"""

from __future__ import annotations

import json
import re
import shutil
import subprocess
from pathlib import Path
from typing import Any, Protocol

from .cli import UnreachableService, package_display_name, resolve_vuln_matches
from .coverage import compute_coverage
from .cpe_mappings import CpeMappings
from .inventory import load_inventory
from .pdf_report import render_pdf_report
from .reporting import (
    build_finding_rows,
    count_by_severity,
    diff_finding_rows,
    render_coverage_report,
    render_findings,
    render_findings_csv,
)
from .sbom import build_sbom

# flexapp_vuln/ -> stage2-resolve/ -> flexapp-vuln-scanner/ (the repo root
# both stage1-extract/ and stage2-resolve/ are siblings under).
_REPO_ROOT = Path(__file__).resolve().parent.parent.parent
STAGE1_SCRIPT = _REPO_ROOT / "stage1-extract" / "Invoke-FlexAppInventory.ps1"
DEFAULT_CACHE_DIR = _REPO_ROOT / "stage2-resolve" / "cache"

_WROTE_INVENTORY_RE = re.compile(r"^Wrote (.+\.inventory\.json)", re.MULTILINE)


class ProgressSink(Protocol):
    status: str

    def append_log(self, line: str) -> None: ...
    def set_progress(self, phase: str, done: int, total: int) -> None: ...


def run_stage1(sink: ProgressSink, package_path: str, output_dir: str) -> Path:
    sink.status = "stage1"
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
        "-Path", package_path, "-OutputDir", output_dir,
    ]
    sink.append_log(f"$ {' '.join(cmd)}")

    output_lines: list[str] = []
    proc = subprocess.Popen(
        cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, bufsize=1,
    )
    assert proc.stdout is not None
    for line in proc.stdout:
        sink.append_log(line)
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


def run_stage2(sink: ProgressSink, inventory_path: Path, output_dir: str, nvd_api_key: str | None) -> dict[str, Any]:
    sink.status = "stage2"
    out_dir = Path(output_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    sink.append_log(f"Loading inventory: {inventory_path}")
    inventory = load_inventory(inventory_path)
    cpe_mappings = CpeMappings.load()

    sink.append_log("Querying OSV.dev + NVD for vulnerability matches...")
    try:
        vuln_matches = resolve_vuln_matches(
            inventory, cache_dir=DEFAULT_CACHE_DIR, cpe_mappings=cpe_mappings, nvd_api_key=nvd_api_key,
            on_progress=sink.set_progress,
        )
    except UnreachableService as exc:
        raise RuntimeError(f"could not reach {exc.host}: {exc.original}") from exc

    out_base = inventory_path.stem.removesuffix(".inventory")
    vuln_matches_path = out_dir / f"{out_base}.vuln-matches.json"
    vuln_matches_path.write_text(json.dumps(vuln_matches, indent=2), encoding="utf-8")
    sink.append_log(f"Wrote {vuln_matches_path}")

    return write_reports(sink, inventory, inventory_path, vuln_matches, out_dir, out_base)


def write_reports(
    sink: ProgressSink | None,
    inventory: dict[str, Any],
    inventory_path: Path,
    vuln_matches: dict[str, Any] | None,
    out_dir: Path,
    out_base: str,
) -> dict[str, Any]:
    """Writes sbom/coverage/findings/PDF and returns the result summary a
    results view renders. Shared between a fresh scan and "open an
    existing output directory" (sink is None there - no log to append to).
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

    # Only written when there IS vuln-matches data - unlike findings.md,
    # a CSV has no way to spell out "no data supplied" in prose, so an
    # absent file (rather than an empty-looking one) is what disambiguates
    # that from "zero vulnerabilities found."
    findings_csv_path = None
    if vuln_matches is not None:
        findings_csv_path = out_dir / f"{out_base}.findings.csv"
        findings_csv_path.write_text(render_findings_csv(vuln_matches), encoding="utf-8")

    package = inventory.get("package", {})
    package_meta = {**package, **(package.get("flexAppXml") or {})}
    render_pdf_report(
        pdf_path, package_name=package_name, package_meta=package_meta,
        coverage=coverage, vuln_matches=vuln_matches,
    )

    if sink is not None:
        written = (sbom_path, coverage_path, findings_path, pdf_path)
        if findings_csv_path is not None:
            written += (findings_csv_path,)
        for path in written:
            sink.append_log(f"Wrote {path}")

    all_rows = build_finding_rows(vuln_matches) if vuln_matches is not None else []
    confirmed_rows = [r for r in all_rows if r["confidence"] in ("exact-purl", "mapped-cpe")]
    heuristic_rows = [r for r in all_rows if r["confidence"] == "heuristic"]

    return {
        "package_name": package_name,
        "coverage": coverage,
        "confirmed_rows": confirmed_rows,
        "heuristic_rows": heuristic_rows,
        "severity_counts": count_by_severity(all_rows),
        "has_vuln_matches": vuln_matches is not None,
        "inventory_path": str(inventory_path),
        "output_dir": str(out_dir),
        "files": {
            "sbom": str(sbom_path),
            "coverage_report": str(coverage_path),
            "findings": str(findings_path),
            "pdf": str(pdf_path),
            **({"findings_csv": str(findings_csv_path)} if findings_csv_path is not None else {}),
        },
    }


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


class DiffError(Exception):
    """A directory given to load_diff() can't be compared - not a
    directory, no inventory in it, or more than one (ambiguous which
    package to compare). Surfaced to a UI as a plain error message, not
    a traceback.
    """


def _find_single_inventory(dir_path: Path) -> Path:
    if not dir_path.is_dir():
        raise DiffError(f"'{dir_path}' is not a directory.")

    inventory_files = sorted(dir_path.glob("*.inventory.json"))
    if not inventory_files:
        raise DiffError(f"No *.inventory.json file found directly under '{dir_path}'.")
    if len(inventory_files) > 1:
        raise DiffError(
            f"'{dir_path}' contains more than one *.inventory.json - comparison needs "
            "a single-scan folder. Use \"Open an Existing Scan Output Folder\" instead "
            "for a directory holding more than one package's scan."
        )
    return inventory_files[0]


def load_diff(old_dir: Path, new_dir: Path) -> dict[str, Any]:
    """Compares two single-package scan output directories: which findings
    are new in `new_dir` that weren't in `old_dir`, which were resolved
    (present in `old_dir`, gone in `new_dir`), and how many are unchanged.
    Raises DiffError if either directory isn't a comparable single-scan
    folder.
    """
    old_inventory_path = _find_single_inventory(old_dir)
    new_inventory_path = _find_single_inventory(new_dir)

    old_result = load_existing_result(old_inventory_path)
    new_result = load_existing_result(new_inventory_path)

    old_rows = old_result["confirmed_rows"] + old_result["heuristic_rows"]
    new_rows = new_result["confirmed_rows"] + new_result["heuristic_rows"]
    finding_diff = diff_finding_rows(old_rows, new_rows)

    return {
        "old": old_result,
        "new": new_result,
        **finding_diff,
    }
