package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"profileunity-msp-console/internal/version"
)

// SchedulerStatus reports the collection scheduler's liveness and last-run
// outcome. The scheduler itself lands in Phase 3; until then every server
// reports "not_implemented" rather than faking a healthy scheduler.
type SchedulerStatus struct {
	Status string `json:"status"`
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
