package httpapi

import (
	"net/http"
	"time"

	"profileunity-msp-console/internal/dashboard"
)

// DashboardDeps bundles what DashboardHandler needs.
type DashboardDeps struct {
	Repos    dashboard.Repos
	Location *time.Location
}

type tenantStatusDTO struct {
	Tenant tenantDTO `json:"tenant"`

	DataStatus   string `json:"dataStatus"`
	UsageStatus  string `json:"usageStatus"`
	ExpiryStatus string `json:"expiryStatus"`

	UtilizationPercent *float64 `json:"utilizationPercent"`
	ExpiryRunwayDays   *int     `json:"expiryRunwayDays"`

	LicenseMode      string `json:"licenseMode,omitempty"`
	LicenseProduct   string `json:"licenseProduct,omitempty"`
	ConsoleVersion   string `json:"consoleVersion,omitempty"`
	TotalLicenses    *int   `json:"totalLicenses,omitempty"`
	UsedLicenses     *int   `json:"usedLicenses,omitempty"`
	LastSuccessAtUTC string `json:"lastSuccessAtUtc,omitempty"`
	LastAttemptAtUTC string `json:"lastAttemptAtUtc,omitempty"`
	LastError        string `json:"lastError,omitempty"`
}

func toTenantStatusDTO(ts dashboard.TenantStatus) tenantStatusDTO {
	dto := tenantStatusDTO{
		Tenant:             toDTO(ts.Tenant),
		DataStatus:         string(ts.Data),
		UsageStatus:        string(ts.Usage),
		ExpiryStatus:       string(ts.Expiry),
		UtilizationPercent: ts.UtilizationPercent,
		ExpiryRunwayDays:   ts.ExpiryRunwayDays,
	}
	if ts.LatestSuccess != nil {
		s := ts.LatestSuccess
		dto.LicenseMode = s.LicenseMode
		dto.LicenseProduct = s.LicenseProduct
		dto.ConsoleVersion = s.ConsoleVersion
		dto.TotalLicenses = s.TotalLicenses
		dto.UsedLicenses = s.UsedLicenses
		dto.LastSuccessAtUTC = s.CollectedAtUTC.UTC().Format(time.RFC3339)
	}
	if ts.Latest != nil {
		dto.LastAttemptAtUTC = ts.Latest.CollectedAtUTC.UTC().Format(time.RFC3339)
		if ts.Latest.Status != "success" {
			dto.LastError = ts.Latest.ErrorMessage
		}
	}
	return dto
}

// DashboardHandler reports every tenant's at-a-glance state (project
// brief §7.3): usage, expiry, and data-trust statuses, computed by the
// same pure functions a future PDF/spreadsheet export will reuse.
func DashboardHandler(deps DashboardDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statuses, err := dashboard.BuildAll(r.Context(), deps.Repos, time.Now().UTC(), deps.Location)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		dtos := make([]tenantStatusDTO, 0, len(statuses))
		for _, ts := range statuses {
			dtos = append(dtos, toTenantStatusDTO(ts))
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}
