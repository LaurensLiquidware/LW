"""Repo-relative path setup, imported first by anything in desktop/ that
needs flexapp_vuln - Stage 2's package lives under stage2-resolve/, a
sibling directory, not installed on the path by default. Same pattern
as webui/paths.py; kept separate rather than shared, since importing
across the webui/desktop boundary for something this small isn't worth
the coupling.
"""

from __future__ import annotations

import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
STAGE2_DIR = REPO_ROOT / "stage2-resolve"

if str(STAGE2_DIR) not in sys.path:
    sys.path.insert(0, str(STAGE2_DIR))
