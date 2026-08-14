"""Server-side filesystem browser backing the "Browse..." links next to
each path input on the dashboard. This process already runs arbitrary
local PowerShell/Python against whatever path you type in - browsing the
filesystem to pick one is the same local trust boundary, not a new one
(see README.md's "Security" note). Deliberately not exposed to anything
but 127.0.0.1.
"""

from __future__ import annotations

import string
import sys
from dataclasses import dataclass
from pathlib import Path

PACKAGE_EXTENSIONS = {".vhdx", ".exe", ".flexapp"}


@dataclass
class Entry:
    name: str
    path: str
    is_dir: bool


def list_drives() -> list[str]:
    if sys.platform != "win32":
        return ["/"]
    drives = []
    for letter in string.ascii_uppercase:
        candidate = f"{letter}:\\"
        if Path(candidate).exists():
            drives.append(candidate)
    return drives


def list_directory(path: Path, *, file_extensions: set[str] | None) -> tuple[list[Entry], list[Entry]]:
    """Returns (subdirectories, files) as sorted Entry lists. `file_extensions`
    restricts which files are shown (case-insensitive); None means don't
    list files at all (directory-only pickers don't need them cluttering
    the view).
    """
    dirs: list[Entry] = []
    files: list[Entry] = []

    try:
        children = sorted(path.iterdir(), key=lambda p: p.name.lower())
    except (PermissionError, OSError):
        return dirs, files

    for child in children:
        try:
            is_dir = child.is_dir()
        except OSError:
            continue  # broken symlink / inaccessible - skip rather than crash the listing

        if is_dir:
            dirs.append(Entry(name=child.name, path=str(child), is_dir=True))
        elif file_extensions is not None and child.suffix.lower() in file_extensions:
            files.append(Entry(name=child.name, path=str(child), is_dir=False))

    return dirs, files
