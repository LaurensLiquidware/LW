# PyInstaller spec for the FlexApp Vulnerability Scanner desktop app.
#
# Build on a real Windows machine (PyInstaller builds for the OS it runs
# on - it does not cross-compile), from this directory:
#
#   pip install -r requirements.txt -r ../requirements.txt
#   pyinstaller flexapp_scanner.spec
#
# Produces dist\FlexAppVulnScanner\FlexAppVulnScanner.exe (--onedir, for
# faster startup than --onefile - see NATIVE_APP_MIGRATION.md's packaging
# note). Still needs pwsh (PowerShell 7) on the target machine's PATH for
# Stage 1 - PyInstaller bundles the Python side only.
#
# NOT YET VALIDATED against a real Windows build - this repo's dev/test
# environment is Linux (see PLAN.md's own precedent for OSV.dev/NVD:
# written against the documented behavior, confirmed only once run on a
# real Windows host). Treat this spec as a starting point, not a
# guarantee it produces a clean build on the first try.

import sys
from pathlib import Path

from PyInstaller.utils.hooks import collect_data_files

block_cipher = None
repo_root = Path(SPECPATH).parent

a = Analysis(
    ["main.py"],
    pathex=[str(repo_root / "stage2-resolve"), SPECPATH],
    binaries=[],
    # cpe-mappings.yaml, string-signatures.psd1, and friends are read by
    # flexapp_vuln at runtime via importlib.resources / relative paths,
    # not just imported - bundle the whole config/ directory alongside
    # the frozen code rather than relying on PyInstaller's import
    # analysis to find non-Python data files. Same reason for
    # rfc3987_syntax's collect_data_files() below: jsonschema's optional
    # "format" checkers pull it in transitively, and it loads a .lark
    # grammar file from disk at import time - PyInstaller's default
    # import analysis follows Python imports, not arbitrary file reads,
    # so without this the frozen exe crashes on startup with
    # FileNotFoundError (caught live building this spec).
    datas=[
        (str(repo_root / "stage2-resolve" / "config"), "config"),
        (str(repo_root / "schemas"), "schemas"),
        *collect_data_files("rfc3987_syntax"),
    ],
    hiddenimports=[],
    hookspath=[],
    runtime_hooks=[],
    excludes=["flask", "werkzeug", "jinja2"],  # the web UI's deps, not needed here
    cipher=block_cipher,
    noarchive=False,
)

pyz = PYZ(a.pure, a.zipped_data, cipher=block_cipher)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="FlexAppVulnScanner",
    debug=False,
    strip=False,
    upx=False,
    console=False,  # a native window, not a console app
    icon=None,  # TODO: add an .ico once the app has real branding assets
)

coll = COLLECT(
    exe,
    a.binaries,
    a.zipfiles,
    a.datas,
    strip=False,
    upx=False,
    name="FlexAppVulnScanner",
)
