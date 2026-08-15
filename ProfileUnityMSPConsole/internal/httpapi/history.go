package httpapi

import (
	"errors"
	"net/http"

	"profileunity-msp-console/internal/dashboard"
	"profileunity-msp-console/internal/tenant"
)

// HistoryDeps bundles what the history handlers need.
type HistoryDeps struct {
	Repos dashboard.Repos
}

type historyPointDTO struct {
	Date          string `json:"date"`
	Status        string `json:"status"`
	UsedLicenses  *int   `json:"usedLicenses"`
	TotalLicenses *int   `json:"totalLicenses"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
}

type entitlementChangeDTO struct {
	Date      string `json:"date"`
	FromTotal int    `json:"fromTotal"`
	ToTotal   int    `json:"toTotal"`
}

type tenantHistoryResponse struct {
	Tenant             tenantDTO              `json:"tenant"`
	Points             []historyPointDTO      `json:"points"`
	EntitlementChanges []entitlementChangeDTO `json:"entitlementChanges"`
}

// TenantHistoryHandler serves one tenant's full collection history
// (project brief §7.4). Points cover every collection attempt, success
// or not — the frontend renders anything but a success as a gap, never
// interpolating across it and never plotting it as zero.
func TenantHistoryHandler(deps HistoryDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		t, err := deps.Repos.Tenants.Get(r.Context(), id)
		if errors.Is(err, tenant.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		points, err := deps.Repos.Snapshots.ListByTenant(r.Context(), id)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := tenantHistoryResponse{
			Tenant:             toDTO(t),
			Points:             make([]historyPointDTO, 0, len(points)),
			EntitlementChanges: make([]entitlementChangeDTO, 0),
		}
		for _, p := range points {
			dto := historyPointDTO{Date: p.CollectionDate, Status: string(p.Status), UsedLicenses: p.UsedLicenses, TotalLicenses: p.TotalLicenses}
			if p.Status != "success" {
				dto.ErrorMessage = p.ErrorMessage
			}
			resp.Points = append(resp.Points, dto)
		}
		for _, c := range dashboard.DetectEntitlementChanges(points) {
			resp.EntitlementChanges = append(resp.EntitlementChanges, entitlementChangeDTO{Date: c.Date, FromTotal: c.FromTotal, ToTotal: c.ToTotal})
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

type portfolioPointDTO struct {
	Date              string `json:"date"`
	TotalUsed         int    `json:"totalUsed"`
	TotalEntitled     int    `json:"totalEntitled"`
	TenantsReporting  int    `json:"tenantsReporting"`
	TenantsRegistered int    `json:"tenantsRegistered"`
}

// PortfolioHistoryHandler serves the aggregate, all-tenants view from
// project brief §7.4.
func PortfolioHistoryHandler(deps HistoryDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := deps.Repos.Tenants.List(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		allSuccess, err := deps.Repos.Snapshots.ListAllSuccess(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		points := dashboard.BuildPortfolioHistory(len(tenants), allSuccess)
		dtos := make([]portfolioPointDTO, 0, len(points))
		for _, p := range points {
			dtos = append(dtos, portfolioPointDTO{
				Date: p.Date, TotalUsed: p.TotalUsed, TotalEntitled: p.TotalEntitled,
				TenantsReporting: p.TenantsReporting, TenantsRegistered: p.TenantsRegistered,
			})
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}
