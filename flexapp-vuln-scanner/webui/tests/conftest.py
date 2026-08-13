import sys
from pathlib import Path

# webui/ itself (parent of this tests/ dir) needs to be on sys.path so
# `import app`, `import jobs`, `import paths` resolve as top-level modules,
# the same way they do when app.py is run directly with `python app.py`.
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
