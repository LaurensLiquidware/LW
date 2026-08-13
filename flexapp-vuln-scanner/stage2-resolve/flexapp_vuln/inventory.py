"""Loads and validates a Stage 1 inventory JSON against the shared schema."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import jsonschema

_DEFAULT_SCHEMA_PATH = Path(__file__).resolve().parents[2] / "schemas" / "inventory.schema.json"


def load_schema(schema_path: Path | str | None = None) -> dict[str, Any]:
    path = Path(schema_path) if schema_path else _DEFAULT_SCHEMA_PATH
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def load_inventory(inventory_path: Path | str, schema_path: Path | str | None = None) -> dict[str, Any]:
    """Loads a Stage 1 inventory JSON file and validates it against the schema.

    Raises jsonschema.ValidationError if the file doesn't match the contract -
    Stage 2 should never silently proceed against a malformed Stage 1 output.
    """
    path = Path(inventory_path)
    with path.open("r", encoding="utf-8") as f:
        data = json.load(f)

    schema = load_schema(schema_path)
    jsonschema.validate(instance=data, schema=schema)
    return data


def iter_non_excluded_files(inventory: dict[str, Any]):
    """Yields file records that were not excluded in Stage 1.

    This is the candidate-component set per PLAN.md's coverage definition:
    denominator = every file with excluded: false.
    """
    for record in inventory.get("files", []):
        if not record.get("excluded", False):
            yield record
