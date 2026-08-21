package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/pipeline"
)

// ScanJob is an in-memory scan job: an adapter between pipeline's
// ProgressSink interface and this HTTP API's polling/SSE model.
//
// This is a local, single-user tool, so an in-memory registry that
// resets on restart is the right amount of infrastructure here, not a
// corner cut -- mirrors ../../../flexapp-vuln-scanner/webui/jobs.py's
// ScanJob/JobRegistry exactly.
type ScanJob struct {
	ID          string
	PackagePath string
	OutputDir   string
	CreatedAt   string

	mu            sync.Mutex
	status        string // queued, stage1, stage2, done, error
	log           []string
	err           string
	result        *pipeline.Result
	progressPhase string
	progressDone  int
	progressTotal int
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

// Snapshot is a point-in-time, JSON-serializable view of a ScanJob.
type Snapshot struct {
	ID            string           `json:"id"`
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
}

// Snapshot returns a consistent, JSON-serializable view of the job's
// current state.
func (j *ScanJob) Snapshot() Snapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	logCopy := make([]string, len(j.log))
	copy(logCopy, j.log)
	return Snapshot{
		ID: j.ID, PackagePath: j.PackagePath, OutputDir: j.OutputDir,
		Status: j.status, Log: logCopy, Error: j.err, CreatedAt: j.CreatedAt,
		ProgressPhase: j.progressPhase, ProgressDone: j.progressDone, ProgressTotal: j.progressTotal,
		Result: j.result,
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

func (r *JobRegistry) create(packagePath, outputDir string) *ScanJob {
	job := &ScanJob{
		ID:          newJobID(),
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

func newJobID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ScanDeps are the dependencies StartScan/StartRefresh need.
type ScanDeps struct {
	Registry       *JobRegistry
	Mappings       *cpemap.Mappings
	StageOneScript string
	CacheDir       string
}

// StartScan runs Stage 1 (mount + inventory) then Stage 2 (OSV/NVD
// matching + reports) on a background goroutine, returning immediately
// with the created job.
func (d ScanDeps) StartScan(packagePath, outputDir, nvdAPIKey string) *ScanJob {
	job := d.Registry.create(packagePath, outputDir)
	go d.runJob(job, nvdAPIKey)
	return job
}

func (d ScanDeps) runJob(job *ScanJob, nvdAPIKey string) {
	inventoryPath, err := pipeline.RunStage1(job, d.StageOneScript, job.PackagePath, job.OutputDir)
	if err != nil {
		job.setError(err)
		return
	}
	result, err := pipeline.RunStage2(job, inventoryPath, job.OutputDir, d.CacheDir, nvdAPIKey, d.Mappings)
	if err != nil {
		job.setError(err)
		return
	}
	job.setResult(result)
	job.SetStatus("done")
}

// StartRefresh re-runs just the OSV/NVD matching + report step against
// an inventory JSON a scan already produced -- no Stage 1 re-mount
// needed. NVD/OSV data changes daily, so this is the way to pick up
// newly-published CVEs against a package without re-scanning it from
// scratch.
func (d ScanDeps) StartRefresh(inventoryPath, outputDir, nvdAPIKey string) *ScanJob {
	job := d.Registry.create(fmt.Sprintf("(refresh) %s", inventoryPath), outputDir)
	go d.runRefreshJob(job, inventoryPath, nvdAPIKey)
	return job
}

func (d ScanDeps) runRefreshJob(job *ScanJob, inventoryPath, nvdAPIKey string) {
	job.AppendLog(fmt.Sprintf("Refreshing vulnerability matches for %s (Stage 1 not re-run)", inventoryPath))
	result, err := pipeline.RunStage2(job, inventoryPath, job.OutputDir, d.CacheDir, nvdAPIKey, d.Mappings)
	if err != nil {
		job.setError(err)
		return
	}
	job.setResult(result)
	job.SetStatus("done")
}
