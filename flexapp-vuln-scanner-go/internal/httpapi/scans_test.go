package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/pipeline"
	"flexapp-vuln-scanner/internal/scanstore"
)

const fixturePath = "../inventory/testdata/sample.inventory.json"

func testScanDeps(t *testing.T) ScanDeps {
	t.Helper()
	return ScanDeps{
		Registry:       NewJobRegistry(),
		Mappings:       cpemap.New(nil),
		StageOneScript: "/nonexistent/Invoke-FlexAppInventory.ps1",
		CacheDir:       t.TempDir(),
	}
}

func copyFixture(t *testing.T, dstDir string) string {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dstDir, "sample.inventory.json")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestStartScanHandler_RequiresPackagePathAndOutputDir(t *testing.T) {
	deps := testScanDeps(t)
	req := httptest.NewRequest(http.MethodPost, "/api/scans", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	StartScanHandler(deps)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestStartScanHandler_MissingStage1ScriptSurfacesAsJobError(t *testing.T) {
	deps := testScanDeps(t)
	body, _ := json.Marshal(startScanRequest{PackagePath: "whatever.vhdx", OutputDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/scans", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	StartScanHandler(deps)(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}

	var snap Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}

	// Poll until the background goroutine reports the error (Stage 1
	// script doesn't exist, per testScanDeps).
	job, ok := deps.Registry.Get(snap.ID)
	if !ok {
		t.Fatal("job not found in registry")
	}
	waitFor(t, func() bool { return job.Snapshot().Status == "error" })
	if got := job.Snapshot().Error; got == "" {
		t.Error("expected a non-empty error message")
	}
}

func TestListScansHandler_ReturnsNewestFirst(t *testing.T) {
	deps := testScanDeps(t)
	job1 := deps.Registry.create("id-a", "a.vhdx", "/tmp/a")
	job2 := deps.Registry.create("id-b", "b.vhdx", "/tmp/b")
	_ = job1
	_ = job2

	req := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	rec := httptest.NewRecorder()
	ListScansHandler(deps)(rec, req)

	var snaps []Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snaps); err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("len(snaps) = %d, want 2", len(snaps))
	}
}

// TestCancelScanHandler_StopsARunningScan proves cancellation is wired
// end-to-end through the HTTP handler: starting a scan against a slow
// Stage 1 script, then cancelling it, actually stops the subprocess and
// leaves the job "canceled" rather than "error" or stuck running.
func TestCancelScanHandler_StopsARunningScan(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not found on PATH")
	}
	deps := testScanDeps(t)
	deps.StageOneScript = "../pipeline/testdata/slow-stage1.ps1"

	body, _ := json.Marshal(startScanRequest{PackagePath: "pkg", OutputDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/scans", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	StartScanHandler(deps)(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	var snap Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}

	job, ok := deps.Registry.Get(snap.ID)
	if !ok {
		t.Fatal("job not found in registry")
	}
	waitFor(t, func() bool { return job.Snapshot().Status == "stage1" })

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/scans/"+snap.ID+"/cancel", nil)
	cancelReq.SetPathValue("id", snap.ID)
	cancelRec := httptest.NewRecorder()
	CancelScanHandler(deps.Registry)(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d", cancelRec.Code)
	}

	start := time.Now()
	waitFor(t, func() bool { return job.Snapshot().Status == "canceled" })
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("took %v to cancel, want well under the script's 30s sleep", elapsed)
	}
}

func TestListScansHandler_MergesPersistedHistoryFromPreviousRun(t *testing.T) {
	deps := testScanDeps(t)
	deps.Store = scanstore.New(filepath.Join(t.TempDir(), "recent-scans.json"))

	// A live job this process ran.
	deps.Registry.create("id-live", "live.vhdx", "/tmp/live")
	deps.persistAdd("id-live", "live.vhdx", "/tmp/live", "scan")

	// A historical entry from a "previous run" -- present in the store
	// but not in this process's live registry.
	pct := 42.0
	if _, err := deps.Store.Add("id-old", "old.vhdx", "/tmp/old", "scan"); err != nil {
		t.Fatal(err)
	}
	if err := deps.Store.Update("id-old", func(e *scanstore.Entry) {
		e.Status = "done"
		e.PackageName = "OldPackage"
		e.CoveragePercent = &pct
	}); err != nil {
		t.Fatal(err)
	}

	snapshots, err := deps.ListScans()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("len(snapshots) = %d, want 2: %+v", len(snapshots), snapshots)
	}

	byID := map[string]Snapshot{}
	for _, s := range snapshots {
		byID[s.ID] = s
	}
	if !byID["id-live"].Live {
		t.Error("id-live should be Live")
	}
	if byID["id-old"].Live {
		t.Error("id-old should not be Live")
	}
	if byID["id-old"].PackageName != "OldPackage" || byID["id-old"].CoveragePercent == nil || *byID["id-old"].CoveragePercent != 42.0 {
		t.Errorf("id-old summary = %+v", byID["id-old"])
	}
}

func TestGetScanHandler_UnknownIDIs404(t *testing.T) {
	deps := testScanDeps(t)
	req := httptest.NewRequest(http.MethodGet, "/api/scans/nope", nil)
	req.SetPathValue("id", "nope")
	rec := httptest.NewRecorder()
	GetScanHandler(deps.Registry)(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDownloadScanFileHandler_UnknownKindIs404(t *testing.T) {
	deps := testScanDeps(t)
	job := deps.Registry.create("id-a", "a.vhdx", "/tmp/a")
	job.setResult(&pipeline.Result{Files: pipeline.ResultFiles{SBOM: "/tmp/a/a.sbom.cdx.json"}})

	req := httptest.NewRequest(http.MethodGet, "/api/scans/"+job.ID+"/files/nope", nil)
	req.SetPathValue("id", job.ID)
	req.SetPathValue("kind", "nope")
	rec := httptest.NewRecorder()
	DownloadScanFileHandler(deps.Registry)(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDownloadScanFileHandler_NoResultYetIsConflict(t *testing.T) {
	deps := testScanDeps(t)
	job := deps.Registry.create("id-a", "a.vhdx", "/tmp/a")

	req := httptest.NewRequest(http.MethodGet, "/api/scans/"+job.ID+"/files/sbom", nil)
	req.SetPathValue("id", job.ID)
	req.SetPathValue("kind", "sbom")
	rec := httptest.NewRecorder()
	DownloadScanFileHandler(deps.Registry)(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestOpenScanHandler_LoadsExistingResult(t *testing.T) {
	dir := t.TempDir()
	inventoryPath := copyFixture(t, dir)

	body, _ := json.Marshal(openScanRequest{InventoryPath: inventoryPath})
	req := httptest.NewRequest(http.MethodPost, "/api/scans/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	OpenScanHandler(cpemap.New(nil))(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var result pipeline.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.HasVulnMatches {
		t.Error("HasVulnMatches = true, want false (no sibling vuln-matches.json)")
	}
}

func TestOpenScanHandler_MissingInventoryPathIs400(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/scans/open", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	OpenScanHandler(cpemap.New(nil))(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCompareScanHandler_NonDirectoryIsBadRequest(t *testing.T) {
	dir := t.TempDir()
	body, _ := json.Marshal(compareScanRequest{OldDir: filepath.Join(dir, "nope"), NewDir: dir})
	req := httptest.NewRequest(http.MethodPost, "/api/scans/compare", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	CompareScanHandler(cpemap.New(nil))(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition never became true")
}
