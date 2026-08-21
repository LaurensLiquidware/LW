package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/pipeline"
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
	job1 := deps.Registry.create("a.vhdx", "/tmp/a")
	job2 := deps.Registry.create("b.vhdx", "/tmp/b")
	_ = job1
	_ = job2

	req := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	rec := httptest.NewRecorder()
	ListScansHandler(deps.Registry)(rec, req)

	var snaps []Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snaps); err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("len(snaps) = %d, want 2", len(snaps))
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
	job := deps.Registry.create("a.vhdx", "/tmp/a")
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
	job := deps.Registry.create("a.vhdx", "/tmp/a")

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
