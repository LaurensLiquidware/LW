package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/pipeline"
	"flexapp-vuln-scanner/internal/scanstore"
)

// ScanJob is an in-memory scan job: an adapter between pipeline's
// ProgressSink interface and this HTTP API's polling/SSE model.
//
// This is a local, single-user tool, so an in-memory registry that
// resets on restart is the right amount of infrastructure here, not a
// corner cut -- mirrors ../../../flexapp-vuln-scanner/webui/jobs.py's
// ScanJob/JobRegistry exactly. Cross-restart dashboard history is a
// separate, lightweight concern handled by internal/scanstore.
type ScanJob struct {
	ID          string
	PackagePath string
	OutputDir   string
	CreatedAt   string

	// cancel stops the job's running goroutine (kills the Stage 1
	// subprocess, aborts an in-flight Stage 2 HTTP call). Set once at
	// creation, before the goroutine starts, so it never races with
	// Cancel() being called from an HTTP handler.
	cancel context.CancelFunc

	mu            sync.Mutex
	status        string // queued, stage1, stage2, done, error, canceled
	log           []string
	err           string
	result        *pipeline.Result
	progressPhase string
	progressDone  int
	progressTotal int
}

// Cancel requests that a running job stop as soon as possible: the
// Stage 1 subprocess is killed, or an in-flight Stage 2 HTTP call is
// aborted and the OSV/NVD matching loop stops between items. A no-op
// once the job has already reached done/error/canceled.
func (j *ScanJob) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
}

func (j *ScanJob) setCanceled() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status = "canceled"
	j.log = append(j.log, "Canceled by user.")
}

// SetStatus implements pipeline.ProgressSink.
func (j *ScanJob) SetStatus(status string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status = status
}

// AppendLog implements pipeline.ProgressSink. Splits on newlines so a
// multi-line chunk of Stage 1 output becomes multiple log lines, same
// as the Python webui's ScanJob.append_log.
func (j *ScanJob) AppendLog(line string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	lines := splitLines(line)
	j.log = append(j.log, lines...)
}

// SetProgress implements pipeline.ProgressSink.
func (j *ScanJob) SetProgress(phase string, done, total int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.progressPhase = phase
	j.progressDone = done
	j.progressTotal = total
}

func (j *ScanJob) setResult(result *pipeline.Result) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.result = result
}

func (j *ScanJob) setError(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status = "error"
	j.err = err.Error()
	j.log = append(j.log, "ERROR: "+err.Error())
}

// Snapshot is a point-in-time, JSON-serializable view of a ScanJob, or
// (when Live is false) a lightweight historical row reconstructed from
// scanstore -- from a scan this process did not itself run, surviving
// a server restart. A historical row has no log or full Result; the
// Results screen re-reads the real files via InventoryPath instead.
type Snapshot struct {
	ID            string           `json:"id"`
	Live          bool             `json:"live"`
	PackagePath   string           `json:"packagePath"`
	OutputDir     string           `json:"outputDir"`
	Status        string           `json:"status"`
	Log           []string         `json:"log"`
	Error         string           `json:"error,omitempty"`
	CreatedAt     string           `json:"createdAt"`
	ProgressPhase string           `json:"progressPhase,omitempty"`
	ProgressDone  int              `json:"progressDone"`
	ProgressTotal int              `json:"progressTotal"`
	Result        *pipeline.Result `json:"result,omitempty"`

	// Summary fields, always populated when known -- from Result for a
	// live job, from the persisted scanstore.Entry for a historical
	// one. Lets the dashboard render one row shape regardless of source.
	PackageName     string         `json:"packageName,omitempty"`
	CoveragePercent *float64       `json:"coveragePercent,omitempty"`
	SeverityCounts  map[string]int `json:"severityCounts,omitempty"`
	InventoryPath   string         `json:"inventoryPath,omitempty"`
}

// Snapshot returns a consistent, JSON-serializable view of the job's
// current state.
func (j *ScanJob) Snapshot() Snapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	logCopy := make([]string, len(j.log))
	copy(logCopy, j.log)
	snap := Snapshot{
		ID: j.ID, Live: true, PackagePath: j.PackagePath, OutputDir: j.OutputDir,
		Status: j.status, Log: logCopy, Error: j.err, CreatedAt: j.CreatedAt,
		ProgressPhase: j.progressPhase, ProgressDone: j.progressDone, ProgressTotal: j.progressTotal,
		Result: j.result,
	}
	if j.result != nil {
		snap.PackageName = j.result.PackageName
		snap.CoveragePercent = j.result.Coverage.CoveragePercent
		snap.SeverityCounts = j.result.SeverityCounts
		snap.InventoryPath = j.result.InventoryPath
	}
	return snap
}

// snapshotFromEntry converts a persisted scanstore.Entry into the same
// Snapshot shape a live ScanJob produces, minus the log/full result.
func snapshotFromEntry(e scanstore.Entry) Snapshot {
	return Snapshot{
		ID: e.ID, Live: false, PackagePath: e.PackagePath, OutputDir: e.OutputDir,
		Status: e.Status, Error: e.Error, CreatedAt: e.CreatedAt,
		PackageName: e.PackageName, CoveragePercent: e.CoveragePercent,
		SeverityCounts: e.SeverityCounts, InventoryPath: e.InventoryPath,
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// JobRegistry holds every scan job started this process's lifetime.
type JobRegistry struct {
	mu   sync.Mutex
	jobs map[string]*ScanJob
}

// NewJobRegistry creates an empty JobRegistry.
func NewJobRegistry() *JobRegistry {
	return &JobRegistry{jobs: map[string]*ScanJob{}}
}

func (r *JobRegistry) create(id, packagePath, outputDir string) *ScanJob {
	job := &ScanJob{
		ID:          id,
		PackagePath: packagePath,
		OutputDir:   outputDir,
		status:      "queued",
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	r.mu.Lock()
	r.jobs[job.ID] = job
	r.mu.Unlock()
	return job
}

// Get looks up a job by id.
func (r *JobRegistry) Get(id string) (*ScanJob, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	return job, ok
}

// ListAll returns every job, newest first.
func (r *JobRegistry) ListAll() []*ScanJob {
	r.mu.Lock()
	jobs := make([]*ScanJob, 0, len(r.jobs))
	for _, j := range r.jobs {
		jobs = append(jobs, j)
	}
	r.mu.Unlock()
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].CreatedAt > jobs[k].CreatedAt })
	return jobs
}

// Has reports whether id belongs to a job in this registry.
func (r *JobRegistry) Has(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.jobs[id]
	return ok
}

func newJobID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ScanDeps are the dependencies StartScan/StartRefresh need.
type ScanDeps struct {
	Registry       *JobRegistry
	Store          *scanstore.Store // nil disables cross-restart persistence
	Mappings       *cpemap.Mappings
	StageOneScript string
	CacheDir       string
}

// ListScans returns every scan job started this process's lifetime,
// plus any scanstore-persisted entries from a previous process run
// that this process hasn't itself touched -- newest first.
func (d ScanDeps) ListScans() ([]Snapshot, error) {
	live := d.Registry.ListAll()
	snapshots := make([]Snapshot, len(live))
	for i, j := range live {
		snapshots[i] = j.Snapshot()
	}

	if d.Store != nil {
		entries, err := d.Store.All()
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !d.Registry.Has(e.ID) {
				snapshots = append(snapshots, snapshotFromEntry(e))
			}
		}
	}

	sort.SliceStable(snapshots, func(i, k int) bool { return snapshots[i].CreatedAt > snapshots[k].CreatedAt })
	return snapshots, nil
}

func (d ScanDeps) persistAdd(id, packagePath, outputDir, kind string) {
	if d.Store == nil {
		return
	}
	_, _ = d.Store.Add(id, packagePath, outputDir, kind)
}

func (d ScanDeps) persistUpdate(id string, mutate func(*scanstore.Entry)) {
	if d.Store == nil {
		return
	}
	_ = d.Store.Update(id, mutate)
}

// StartScan runs Stage 1 (mount + inventory) then Stage 2 (OSV/NVD
// matching + reports) on a background goroutine, returning immediately
// with the created job.
func (d ScanDeps) StartScan(packagePath, outputDir, nvdAPIKey string) *ScanJob {
	id := newJobID()
	job := d.Registry.create(id, packagePath, outputDir)
	ctx, cancel := context.WithCancel(context.Background())
	job.cancel = cancel
	d.persistAdd(id, packagePath, outputDir, "scan")
	go d.runJob(ctx, job, nvdAPIKey)
	return job
}

func (d ScanDeps) runJob(ctx context.Context, job *ScanJob, nvdAPIKey string) {
	inventoryPath, err := pipeline.RunStage1(ctx, job, d.StageOneScript, job.PackagePath, job.OutputDir)
	if err != nil {
		d.finishWithError(job, err)
		return
	}
	d.persistUpdate(job.ID, func(e *scanstore.Entry) { e.Status = "stage2"; e.InventoryPath = inventoryPath })

	result, err := pipeline.RunStage2(ctx, job, inventoryPath, job.OutputDir, d.CacheDir, nvdAPIKey, d.Mappings)
	if err != nil {
		d.finishWithError(job, err)
		return
	}
	d.finishWithResult(job, result)
}

// StartRefresh re-runs just the OSV/NVD matching + report step against
// an inventory JSON a scan already produced -- no Stage 1 re-mount
// needed. NVD/OSV data changes daily, so this is the way to pick up
// newly-published CVEs against a package without re-scanning it from
// scratch.
func (d ScanDeps) StartRefresh(inventoryPath, outputDir, nvdAPIKey string) *ScanJob {
	id := newJobID()
	job := d.Registry.create(id, fmt.Sprintf("(refresh) %s", inventoryPath), outputDir)
	ctx, cancel := context.WithCancel(context.Background())
	job.cancel = cancel
	d.persistAdd(id, job.PackagePath, outputDir, "refresh")
	go d.runRefreshJob(ctx, job, inventoryPath, nvdAPIKey)
	return job
}

func (d ScanDeps) runRefreshJob(ctx context.Context, job *ScanJob, inventoryPath, nvdAPIKey string) {
	job.AppendLog(fmt.Sprintf("Refreshing vulnerability matches for %s (Stage 1 not re-run)", inventoryPath))
	result, err := pipeline.RunStage2(ctx, job, inventoryPath, job.OutputDir, d.CacheDir, nvdAPIKey, d.Mappings)
	if err != nil {
		d.finishWithError(job, err)
		return
	}
	d.finishWithResult(job, result)
}

// finishWithError records a job's terminal failure, distinguishing a
// user-requested cancellation (context.Canceled) from a real error.
func (d ScanDeps) finishWithError(job *ScanJob, err error) {
	if errors.Is(err, context.Canceled) {
		job.setCanceled()
		d.persistUpdate(job.ID, func(e *scanstore.Entry) { e.Status = "canceled" })
		return
	}
	job.setError(err)
	d.persistUpdate(job.ID, func(e *scanstore.Entry) { e.Status = "error"; e.Error = err.Error() })
}

func (d ScanDeps) finishWithResult(job *ScanJob, result *pipeline.Result) {
	job.setResult(result)
	job.SetStatus("done")
	d.persistUpdate(job.ID, func(e *scanstore.Entry) {
		e.Status = "done"
		e.PackageName = result.PackageName
		e.CoveragePercent = result.Coverage.CoveragePercent
		e.SeverityCounts = result.SeverityCounts
		e.InventoryPath = result.InventoryPath
	})
}
