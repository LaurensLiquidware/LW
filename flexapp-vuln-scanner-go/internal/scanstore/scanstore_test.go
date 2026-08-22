package scanstore

import (
	"os"
	"path/filepath"
	"testing"
)

// The test cases here mirror
// ../../../flexapp-vuln-scanner/desktop/tests/test_recent_scans_store.py,
// adapted for Store.Add taking an externally-supplied id (matching the
// caller's already-generated ScanJob.ID) rather than generating its own.

func TestAdd_CreatesEntryWithQueuedStatus(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "recent-scans.json"))

	entry, err := store.Add("id1", `C:\pkg\App.vhdx`, `C:\out`, "scan")
	if err != nil {
		t.Fatal(err)
	}
	if entry.PackagePath != `C:\pkg\App.vhdx` {
		t.Errorf("PackagePath = %q", entry.PackagePath)
	}
	if entry.Status != "queued" {
		t.Errorf("Status = %q, want queued", entry.Status)
	}
	if entry.Kind != "scan" {
		t.Errorf("Kind = %q, want scan", entry.Kind)
	}

	all, err := store.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != entry.ID || all[0].PackagePath != entry.PackagePath {
		t.Errorf("All() = %+v, want [%+v]", all, entry)
	}
}

func TestEntriesPersistAcrossStoreInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent-scans.json")
	store1 := New(path)
	if _, err := store1.Add("id1", "a.vhdx", "/out/a", "scan"); err != nil {
		t.Fatal(err)
	}

	store2 := New(path)
	all, err := store2.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].PackagePath != "a.vhdx" {
		t.Errorf("All() = %+v", all)
	}
}

func TestNewestEntryIsFirst(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "recent-scans.json"))
	first, err := store.Add("id1", "a.vhdx", "/out/a", "scan")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Add("id2", "b.vhdx", "/out/b", "scan")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := store.All()
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].ID != second.ID || entries[1].ID != first.ID {
		t.Errorf("entries = %+v", entries)
	}
}

func TestUpdate_ModifiesMatchingEntryOnly(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "recent-scans.json"))
	entryA, err := store.Add("id-a", "a.vhdx", "/out/a", "scan")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add("id-b", "b.vhdx", "/out/b", "scan"); err != nil {
		t.Fatal(err)
	}

	if err := store.Update(entryA.ID, func(e *Entry) {
		e.Status = "done"
		e.PackageName = "A"
	}); err != nil {
		t.Fatal(err)
	}

	all, err := store.All()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Entry{}
	for _, e := range all {
		byID[e.ID] = e
	}
	if byID[entryA.ID].Status != "done" || byID[entryA.ID].PackageName != "A" {
		t.Errorf("entry A = %+v", byID[entryA.ID])
	}
	if byID["id-b"].Status != "queued" {
		t.Errorf("entry B should be untouched, got %+v", byID["id-b"])
	}
}

func TestRemove_DeletesEntry(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "recent-scans.json"))
	entry, err := store.Add("id1", "a.vhdx", "/out/a", "scan")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Remove(entry.ID); err != nil {
		t.Fatal(err)
	}

	all, err := store.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("All() = %+v, want empty", all)
	}
}

func TestMissingFileReturnsEmptyList(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "does-not-exist.json"))
	all, err := store.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("All() = %+v, want empty", all)
	}
}

func TestCorruptFileReturnsEmptyListRatherThanCrashing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent-scans.json")
	if err := os.WriteFile(path, []byte("not valid json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := New(path)
	all, err := store.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("All() = %+v, want empty", all)
	}
}
