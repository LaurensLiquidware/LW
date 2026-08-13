from pathlib import Path

import pytest
from jsonschema import ValidationError

from flexapp_vuln.inventory import iter_non_excluded_files, load_inventory

FIXTURE = Path(__file__).parent / "fixtures" / "sample.inventory.json"


def test_load_valid_inventory():
    data = load_inventory(FIXTURE)
    assert data["schemaVersion"] == "1.0"
    assert len(data["files"]) == 4


def test_iter_non_excluded_files_skips_excluded():
    data = load_inventory(FIXTURE)
    non_excluded = list(iter_non_excluded_files(data))
    assert len(non_excluded) == 3
    assert all(not f["excluded"] for f in non_excluded)


def test_load_invalid_inventory_raises(tmp_path):
    bad = tmp_path / "bad.json"
    bad.write_text('{"schemaVersion": "1.0"}')  # missing required "files"/"package"
    with pytest.raises(ValidationError):
        load_inventory(bad)
