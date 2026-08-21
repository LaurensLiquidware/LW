from recent_scans_store import RecentScansStore


def test_add_creates_entry_with_queued_status(tmp_path):
    store = RecentScansStore(tmp_path / "recent-scans.json")

    entry = store.add("C:\\pkg\\App.vhdx", "C:\\out", kind="scan")

    assert entry["package_path"] == "C:\\pkg\\App.vhdx"
    assert entry["status"] == "queued"
    assert entry["kind"] == "scan"
    assert store.all() == [entry]


def test_entries_persist_across_store_instances(tmp_path):
    path = tmp_path / "recent-scans.json"
    store1 = RecentScansStore(path)
    store1.add("a.vhdx", "/out/a")

    store2 = RecentScansStore(path)
    assert len(store2.all()) == 1
    assert store2.all()[0]["package_path"] == "a.vhdx"


def test_newest_entry_is_first(tmp_path):
    store = RecentScansStore(tmp_path / "recent-scans.json")
    first = store.add("a.vhdx", "/out/a")
    second = store.add("b.vhdx", "/out/b")

    entries = store.all()
    assert entries[0]["id"] == second["id"]
    assert entries[1]["id"] == first["id"]


def test_update_modifies_matching_entry_only(tmp_path):
    store = RecentScansStore(tmp_path / "recent-scans.json")
    entry_a = store.add("a.vhdx", "/out/a")
    store.add("b.vhdx", "/out/b")

    store.update(entry_a["id"], status="done", package_name="A")

    entries = {e["id"]: e for e in store.all()}
    assert entries[entry_a["id"]]["status"] == "done"
    assert entries[entry_a["id"]]["package_name"] == "A"


def test_remove_deletes_entry(tmp_path):
    store = RecentScansStore(tmp_path / "recent-scans.json")
    entry = store.add("a.vhdx", "/out/a")

    store.remove(entry["id"])

    assert store.all() == []


def test_missing_file_returns_empty_list(tmp_path):
    store = RecentScansStore(tmp_path / "does-not-exist.json")
    assert store.all() == []


def test_corrupt_file_returns_empty_list_rather_than_crashing(tmp_path):
    path = tmp_path / "recent-scans.json"
    path.write_text("not valid json{{{", encoding="utf-8")
    store = RecentScansStore(path)
    assert store.all() == []
