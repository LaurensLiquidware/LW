package httpapi

import (
	"net/http"
	"time"

	"profileunity-msp-console/internal/dashboard"
)

// AlertDeps bundles what AlertsHandler needs.
type AlertDeps struct {
	Repos    dashboard.Repos
	Location *time.Location
}

type alertDTO struct {
	Tenant  tenantStatusDTO `json:"tenant"`
	Reasons []string        `json:"reasons"`
}

// AlertsHandler reports every tenant currently needing operator
// attention (project brief §7.6: over its license limit, support
// expired/expiring soon, or its data can't currently be trusted). This
// is in-app only, not emailed — see README for the scope decision — so
// the frontend polls this on a normal page load rather than the server
// pushing anything.
func AlertsHandler(deps AlertDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statuses, err := dashboard.BuildAll(r.Context(), deps.Repos, time.Now().UTC(), deps.Location)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		alerts := dashboard.DetectAlerts(statuses)
		dtos := make([]alertDTO, 0, len(alerts))
		for _, a := range alerts {
			reasons := make([]string, 0, len(a.Reasons))
			for _, r := range a.Reasons {
				reasons = append(reasons, string(r))
			}
			dtos = append(dtos, alertDTO{Tenant: toTenantStatusDTO(a.Tenant), Reasons: reasons})
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}
