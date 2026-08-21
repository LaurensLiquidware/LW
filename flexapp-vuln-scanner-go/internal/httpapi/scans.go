package httpapi

import (
	"encoding/json"
	"net/http"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/pipeline"
)

// startScanRequest is the POST /api/scans request body.
type startScanRequest struct {
	PackagePath string `json:"packagePath"`
	OutputDir   string `json:"outputDir"`
	NVDAPIKey   string `json:"nvdApiKey,omitempty"`
}

// StartScanHandler starts a new scan (Stage 1 + Stage 2) on a
// background goroutine and returns the created job immediately; the
// caller polls GET /api/scans/{id} (or watches /api/scans/{id}/events)
// for progress.
func StartScanHandler(deps ScanDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req startScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if req.PackagePath == "" || req.OutputDir == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "packagePath and outputDir are required"})
			return
		}
		job := deps.StartScan(req.PackagePath, req.OutputDir, req.NVDAPIKey)
		writeJSON(w, http.StatusAccepted, job.Snapshot())
	}
}

// refreshScanRequest is the POST /api/scans/refresh request body.
type refreshScanRequest struct {
	InventoryPath string `json:"inventoryPath"`
	OutputDir     string `json:"outputDir"`
	NVDAPIKey     string `json:"nvdApiKey,omitempty"`
}

// RefreshScanHandler re-runs Stage 2 (OSV/NVD matching) against an
// already-produced inventory JSON, without re-mounting the package.
func RefreshScanHandler(deps ScanDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if req.InventoryPath == "" || req.OutputDir == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inventoryPath and outputDir are required"})
			return
		}
		job := deps.StartRefresh(req.InventoryPath, req.OutputDir, req.NVDAPIKey)
		writeJSON(w, http.StatusAccepted, job.Snapshot())
	}
}

// ListScansHandler lists every scan job started this process's
// lifetime, newest first -- the dashboard's data source.
func ListScansHandler(registry *JobRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobs := registry.ListAll()
		snapshots := make([]Snapshot, len(jobs))
		for i, j := range jobs {
			snapshots[i] = j.Snapshot()
		}
		writeJSON(w, http.StatusOK, snapshots)
	}
}

// GetScanHandler returns one job's current snapshot, for polling.
func GetScanHandler(registry *JobRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		job, ok := registry.Get(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such scan job"})
			return
		}
		writeJSON(w, http.StatusOK, job.Snapshot())
	}
}

// scanFileKind maps a URL path segment to the Result.Files field it
// serves, so a request can only ever reach a path this job's own
// pipeline run actually produced -- never an arbitrary filesystem path.
func scanFileKind(files pipeline.ResultFiles, kind string) (path string, ok bool) {
	switch kind {
	case "sbom":
		return files.SBOM, files.SBOM != ""
	case "coverage":
		return files.CoverageReport, files.CoverageReport != ""
	case "findings":
		return files.Findings, files.Findings != ""
	case "pdf":
		return files.PDF, files.PDF != ""
	case "csv":
		return files.FindingsCSV, files.FindingsCSV != ""
	default:
		return "", false
	}
}

// DownloadScanFileHandler serves one of a completed job's report
// artifacts (sbom/coverage/findings/pdf/csv). Only reachable file paths
// are ones this job's own WriteReports call actually wrote -- never an
// arbitrary caller-supplied filesystem path.
func DownloadScanFileHandler(registry *JobRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		kind := r.PathValue("kind")
		job, ok := registry.Get(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such scan job"})
			return
		}
		snapshot := job.Snapshot()
		if snapshot.Result == nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "scan has not produced a result yet"})
			return
		}
		path, ok := scanFileKind(snapshot.Result.Files, kind)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such report file for this scan"})
			return
		}
		http.ServeFile(w, r, path)
	}
}

// openScanRequest is the POST /api/scans/open request body.
type openScanRequest struct {
	InventoryPath string `json:"inventoryPath"`
}

// OpenScanHandler loads an already-completed scan's results view from
// its inventory JSON, reusing a sibling vuln-matches.json if present --
// no network calls, synchronous.
func OpenScanHandler(mappings *cpemap.Mappings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req openScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if req.InventoryPath == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inventoryPath is required"})
			return
		}
		result, err := pipeline.LoadExistingResult(req.InventoryPath, mappings)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// compareScanRequest is the POST /api/scans/compare request body.
type compareScanRequest struct {
	OldDir string `json:"oldDir"`
	NewDir string `json:"newDir"`
}

// CompareScanHandler compares two single-package scan output
// directories and reports new/resolved/unchanged findings.
func CompareScanHandler(mappings *cpemap.Mappings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req compareScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if req.OldDir == "" || req.NewDir == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "oldDir and newDir are required"})
			return
		}
		diff, err := pipeline.LoadDiff(req.OldDir, req.NewDir, mappings)
		if err != nil {
			if _, ok := err.(*pipeline.DiffError); ok {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, diff)
	}
}
