package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"profileunity-msp-console/internal/version"
)

// SchedulerStatus reports the collection scheduler's liveness and last-run
// outcome (project brief §9: "health endpoint reporting scheduler
// liveness and last-run outcome"). Status is "not_implemented" only
// before the scheduler starts; once it does, it is "ok", "partial",
// "failed", or "run_error" as internal/scheduler.Status reports.
type SchedulerStatus struct {
	Status       string `json:"status"`
	Running      bool   `json:"running"`
	LastRunAtUTC string `json:"lastRunAtUtc,omitempty"`
	LastRunError string `json:"lastRunError,omitempty"`
	TenantCount  int    `json:"tenantCount,omitempty"`
	SuccessCount int    `json:"successCount,omitempty"`
}

type healthResponse struct {
	Status    string          `json:"status"`
	Version   string          `json:"version"`
	TimeUTC   string          `json:"timeUtc"`
	Scheduler SchedulerStatus `json:"scheduler"`
}

// HealthHandler returns the health endpoint handler. schedulerStatus is a
// function rather than a fixed value so later phases can report live
// scheduler state without changing this handler's signature.
func HealthHandler(schedulerStatus func() SchedulerStatus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := healthResponse{
			Status:    "ok",
			Version:   version.Version,
			TimeUTC:   time.Now().UTC().Format(time.RFC3339),
			Scheduler: schedulerStatus(),
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(resp)
	}
}
