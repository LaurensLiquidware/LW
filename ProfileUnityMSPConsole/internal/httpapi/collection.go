package httpapi

import (
	"net/http"

	"profileunity-msp-console/internal/scheduler"
)

// CollectionDeps bundles what CollectNowHandler needs. Status reuses the
// same schedulerStatus callback NewRouter takes for the health endpoint,
// so a manual run and the health endpoint agree on what "success" means
// without duplicating scheduler.outcomeFor's logic here.
type CollectionDeps struct {
	Scheduler *scheduler.Scheduler
	Status    func() SchedulerStatus

	// DemoMode is true when running against a demo.db sidecar file. See
	// DisallowInDemoMode, which uses this to block CollectNowHandler
	// (wired in router.go) so demo tenants' fictional hostnames are never
	// polled.
	DemoMode bool
}

// CollectNowHandler triggers an immediate collection pass across every
// enabled tenant (project brief §7.2: manual "Collect Now") — the same
// code path the scheduler's own ticker uses. It blocks until the run
// finishes and returns the resulting status, since the caller (a
// "Collect Now" button) needs a result to refresh its view against.
func CollectNowHandler(deps CollectionDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := deps.Scheduler.CollectNow(r.Context()); err != nil {
			http.Error(w, "collection failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, deps.Status())
	}
}
