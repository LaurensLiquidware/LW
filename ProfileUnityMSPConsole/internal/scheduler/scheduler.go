// Package scheduler runs collection across every enabled tenant, on an
// in-process ticker (project brief §5: "no external cron dependency").
// Snapshots are keyed by calendar day and upserted (internal/snapshot),
// so ticking more often than daily is harmless — see internal/config's
// PUMC_COLLECTION_INTERVAL doc comment. Manual "Collect Now" (§7.2) is
// just CollectNow, the same code path the ticker uses.
package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"profileunity-msp-console/internal/collector"
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

// RunSummary is the result of one collection pass.
type RunSummary struct {
	RunID       string
	StartedAt   time.Time
	FinishedAt  time.Time
	TenantCount int
	Counts      map[snapshot.Status]int
}

// Status is a point-in-time view of the scheduler, for the health
// endpoint (project brief §9: "health endpoint reporting scheduler
// liveness and last-run outcome").
type Status struct {
	Running        bool
	LastRunAt      time.Time
	LastRunOutcome string // "ok", "partial", "failed", "run_error", or "" if never run
	LastRunSummary RunSummary
	LastRunError   string
}

// Scheduler polls every enabled tenant on Interval, capped at
// Concurrency concurrent polls, each bounded by TenantTimeout (including
// retries) so one dead tenant never stalls the run.
type Scheduler struct {
	tenants   *tenant.Repo
	snapshots *snapshot.Repo

	Interval      time.Duration
	Location      *time.Location
	Concurrency   int
	TenantTimeout time.Duration

	mu     sync.Mutex
	status Status
}

// New creates a Scheduler. Concurrency and TenantTimeout must be set to
// sane positive values by the caller (config.Load already validates its
// own equivalents).
func New(tenants *tenant.Repo, snapshots *snapshot.Repo, interval time.Duration, location *time.Location, concurrency int, tenantTimeout time.Duration) *Scheduler {
	return &Scheduler{
		tenants:       tenants,
		snapshots:     snapshots,
		Interval:      interval,
		Location:      location,
		Concurrency:   concurrency,
		TenantTimeout: tenantTimeout,
	}
}

// Run blocks, collecting once immediately and then on every tick, until
// ctx is canceled. Call it in its own goroutine.
func (s *Scheduler) Run(ctx context.Context) {
	s.CollectNow(ctx)

	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CollectNow(ctx)
		}
	}
}

// CollectNow runs one collection pass over every enabled tenant and
// returns immediately once it finishes. It is the same code path the
// ticker in Run uses, so a manual "Collect Now" (§7.2) behaves exactly
// like a scheduled run.
func (s *Scheduler) CollectNow(ctx context.Context) (RunSummary, error) {
	s.setRunning(true)
	defer s.setRunning(false)

	runID := uuid.NewString()
	now := time.Now().UTC()

	tenants, err := s.tenants.List(ctx)
	if err != nil {
		s.recordRunError(now, err)
		return RunSummary{}, err
	}

	var enabled []tenant.Tenant
	for _, t := range tenants {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}

	counts := make(map[snapshot.Status]int)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(1, s.Concurrency))

	for _, t := range enabled {
		t := t
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			status := s.collectTenant(ctx, runID, t, now)
			mu.Lock()
			counts[status]++
			mu.Unlock()
		}()
	}
	wg.Wait()

	summary := RunSummary{
		RunID:       runID,
		StartedAt:   now,
		FinishedAt:  time.Now().UTC(),
		TenantCount: len(enabled),
		Counts:      counts,
	}
	s.recordRunSummary(summary)
	return summary, nil
}

// collectTenant runs one tenant's collection and persists the result.
// Panics are recovered here so one tenant's bug can never take the whole
// scheduler down (project brief §7.2: "one dead tenant must never stall
// the run"). Persistence always uses a fresh, un-timed-out context —
// tenantCtx may already have expired by the time there's a result to
// store, and a failed poll must still be recorded.
func (s *Scheduler) collectTenant(ctx context.Context, runID string, t tenant.Tenant, now time.Time) (resultStatus snapshot.Status) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("collection run %s: tenant %s panicked: %v", runID, t.ID, r)
			s.storeErrorSnapshot(t.ID, now, "internal error during collection")
			resultStatus = snapshot.StatusError
		}
	}()

	tenantCtx, cancel := context.WithTimeout(ctx, s.TenantTimeout)
	defer cancel()

	var creds *tenant.Credentials
	if t.HasPassword {
		var err error
		creds, err = s.tenants.GetCredentials(tenantCtx, t.ID)
		if err != nil {
			log.Printf("collection run %s: tenant %s: load credentials: %v", runID, t.ID, err)
			s.storeErrorSnapshot(t.ID, now, "failed to load stored credentials")
			return snapshot.StatusError
		}
	}

	snap := collector.CollectOne(tenantCtx, t, creds, now, s.Location)
	if _, err := s.snapshots.Upsert(context.Background(), snap); err != nil {
		log.Printf("collection run %s: tenant %s: store snapshot: %v", runID, t.ID, err)
	}
	return snap.Status
}

func (s *Scheduler) storeErrorSnapshot(tenantID string, now time.Time, message string) {
	snap := snapshot.Snapshot{
		TenantID:       tenantID,
		CollectionDate: snapshot.CollectionDateFor(now, s.Location),
		CollectedAtUTC: now,
		Status:         snapshot.StatusError,
		ErrorMessage:   message,
	}
	if _, err := s.snapshots.Upsert(context.Background(), snap); err != nil {
		log.Printf("tenant %s: store error snapshot: %v", tenantID, err)
	}
}

// Status returns the current scheduler state, for the health endpoint.
func (s *Scheduler) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Scheduler) setRunning(running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Running = running
}

func (s *Scheduler) recordRunSummary(summary RunSummary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastRunAt = summary.FinishedAt
	s.status.LastRunSummary = summary
	s.status.LastRunError = ""
	s.status.LastRunOutcome = outcomeFor(summary)
}

func (s *Scheduler) recordRunError(at time.Time, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastRunAt = at
	s.status.LastRunOutcome = "run_error"
	s.status.LastRunError = err.Error()
}

func outcomeFor(summary RunSummary) string {
	if summary.TenantCount == 0 {
		return "ok"
	}
	successes := summary.Counts[snapshot.StatusSuccess]
	switch {
	case successes == summary.TenantCount:
		return "ok"
	case successes == 0:
		return "failed"
	default:
		return "partial"
	}
}
