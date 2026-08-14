"""Repo-relative path setup, imported first by anything in webui/ that needs
flexapp_vuln - Stage 2's package lives under stage2-resolve/, a sibling
directory, not installed on the path by default.
"""

from __future__ import annotations

import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
STAGE1_DIR = REPO_ROOT / "stage1-extract"
STAGE2_DIR = REPO_ROOT / "stage2-resolve"

if str(STAGE2_DIR) not in sys.path:
    sys.path.insert(0, str(STAGE2_DIR))
