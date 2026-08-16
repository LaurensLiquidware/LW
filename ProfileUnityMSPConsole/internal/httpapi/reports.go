package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"profileunity-msp-console/internal/dashboard"
	"profileunity-msp-console/internal/reportpdf"
	"profileunity-msp-console/internal/tenant"
)

// ReportDeps bundles what the report handlers need.
type ReportDeps struct {
	Repos dashboard.Repos

	// DemoMode watermarks PDF exports (see reportpdf.RenderTenantReportPDF/
	// RenderPortfolioReportPDF) whenever running against a demo.db
	// sidecar database.
	DemoMode bool
}

// monthRange parses "year"/"month" query parameters (both required,
// month 1-12) and returns the first/last day of that calendar month as
// "YYYY-MM-DD" strings plus the day count — the inputs BuildTenantMonthlyReport
// and BuildPortfolioMonthlyReport need.
func monthRange(r *http.Request) (year, month, days int, from, to string, err error) {
	year, err = strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil {
		return 0, 0, 0, "", "", fmt.Errorf("invalid or missing year")
	}
	month, err = strconv.Atoi(r.URL.Query().Get("month"))
	if err != nil || month < 1 || month > 12 {
		return 0, 0, 0, "", "", fmt.Errorf("invalid or missing month (must be 1-12)")
	}

	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)
	return year, month, last.Day(), first.Format("2006-01-02"), last.Format("2006-01-02"), nil
}

type entitlementChangeReportDTO struct {
	Date      string `json:"date"`
	FromTotal int    `json:"fromTotal"`
	ToTotal   int    `json:"toTotal"`
}

type tenantMonthlyReportDTO struct {
	Tenant tenantDTO `json:"tenant"`
	Year   int       `json:"year"`
	Month  int       `json:"month"`

	DaysInMonth        int    `json:"daysInMonth"`
	DaysCollected      int    `json:"daysCollected"`
	DaysFailed         int    `json:"daysFailed"`
	DaysNeverAttempted int    `json:"daysNeverAttempted"`
	Coverage           string `json:"coverage"`

	PeakUsed     *int     `json:"peakUsed"`
	PeakUsedDate string   `json:"peakUsedDate,omitempty"`
	AverageUsed  *float64 `json:"averageUsed"`

	EntitledAtMonthEnd       *int                         `json:"entitledAtMonthEnd"`
	MaximumUsersAtMonthEnd   *int                         `json:"maximumUsersAtMonthEnd"`
	LicenseProductAtMonthEnd string                       `json:"licenseProductAtMonthEnd,omitempty"`
	EntitlementChanges       []entitlementChangeReportDTO `json:"entitlementChanges"`
}

func toTenantReportDTO(r dashboard.TenantMonthlyReport) tenantMonthlyReportDTO {
	changes := make([]entitlementChangeReportDTO, 0, len(r.EntitlementChanges))
	for _, c := range r.EntitlementChanges {
		changes = append(changes, entitlementChangeReportDTO{Date: c.Date, FromTotal: c.FromTotal, ToTotal: c.ToTotal})
	}
	return tenantMonthlyReportDTO{
		Tenant:                   toDTO(r.Tenant),
		Year:                     r.Year,
		Month:                    r.Month,
		DaysInMonth:              r.DaysInMonth,
		DaysCollected:            r.DaysCollected,
		DaysFailed:               r.DaysFailed,
		DaysNeverAttempted:       r.DaysNeverAttempted,
		Coverage:                 string(r.Coverage),
		PeakUsed:                 r.PeakUsed,
		PeakUsedDate:             r.PeakUsedDate,
		AverageUsed:              r.AverageUsed,
		EntitledAtMonthEnd:       r.EntitledAtMonthEnd,
		MaximumUsersAtMonthEnd:   r.MaximumUsersAtMonthEnd,
		LicenseProductAtMonthEnd: r.LicenseProductAtMonthEnd,
		EntitlementChanges:       changes,
	}
}

// buildTenantReport loads everything BuildTenantMonthlyReport needs for
// one tenant + month. Returns tenant.ErrNotFound unchanged so callers can
// map it to 404.
func buildTenantReport(r *http.Request, deps ReportDeps, tenantID string) (dashboard.TenantMonthlyReport, error) {
	year, month, days, from, to, err := monthRange(r)
	if err != nil {
		return dashboard.TenantMonthlyReport{}, err
	}
	t, err := deps.Repos.Tenants.Get(r.Context(), tenantID)
	if err != nil {
		return dashboard.TenantMonthlyReport{}, err
	}
	points, err := deps.Repos.Snapshots.ListByTenantInRange(r.Context(), tenantID, from, to)
	if err != nil {
		return dashboard.TenantMonthlyReport{}, err
	}
	return dashboard.BuildTenantMonthlyReport(t, year, month, days, points), nil
}

// TenantMonthlyReportHandler serves one tenant's monthly usage/
// entitlement/coverage report as JSON (project brief §7.5).
func TenantMonthlyReportHandler(deps ReportDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := buildTenantReport(r, deps, r.PathValue("id"))
		if errors.Is(err, tenant.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, toTenantReportDTO(report))
	}
}

// TenantMonthlyReportPDFHandler serves the same report rendered as a PDF
// download.
func TenantMonthlyReportPDFHandler(deps ReportDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := buildTenantReport(r, deps, r.PathValue("id"))
		if errors.Is(err, tenant.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		pdf := reportpdf.RenderTenantReportPDF(report, deps.DemoMode)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%04d-%02d.pdf"`, reportpdf.SafeFilenamePart(report.Tenant.DisplayName), report.Year, report.Month))
		if err := pdf.Output(w); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}
}

type portfolioMonthlyReportDTO struct {
	Year              int `json:"year"`
	Month             int `json:"month"`
	TenantsRegistered int `json:"tenantsRegistered"`

	PeakTotalUsed     *int     `json:"peakTotalUsed"`
	PeakTotalUsedDate string   `json:"peakTotalUsedDate,omitempty"`
	AverageTotalUsed  *float64 `json:"averageTotalUsed"`

	TotalEntitledAtMonthEnd     *int                     `json:"totalEntitledAtMonthEnd"`
	TotalMaximumUsersAtMonthEnd *int                     `json:"totalMaximumUsersAtMonthEnd"`
	TenantReports               []tenantMonthlyReportDTO `json:"tenantReports"`
}

func toPortfolioReportDTO(r dashboard.PortfolioMonthlyReport) portfolioMonthlyReportDTO {
	tenantReports := make([]tenantMonthlyReportDTO, 0, len(r.TenantReports))
	for _, tr := range r.TenantReports {
		tenantReports = append(tenantReports, toTenantReportDTO(tr))
	}
	return portfolioMonthlyReportDTO{
		Year:                        r.Year,
		Month:                       r.Month,
		TenantsRegistered:           r.TenantsRegistered,
		PeakTotalUsed:               r.PeakTotalUsed,
		PeakTotalUsedDate:           r.PeakTotalUsedDate,
		AverageTotalUsed:            r.AverageTotalUsed,
		TotalEntitledAtMonthEnd:     r.TotalEntitledAtMonthEnd,
		TotalMaximumUsersAtMonthEnd: r.TotalMaximumUsersAtMonthEnd,
		TenantReports:               tenantReports,
	}
}

// buildPortfolioReport loads everything BuildPortfolioMonthlyReport needs
// across every tenant for one month.
func buildPortfolioReport(r *http.Request, deps ReportDeps) (dashboard.PortfolioMonthlyReport, error) {
	year, month, days, from, to, err := monthRange(r)
	if err != nil {
		return dashboard.PortfolioMonthlyReport{}, err
	}
	return dashboard.LoadPortfolioMonthlyReport(r.Context(), deps.Repos, year, month, days, from, to)
}

// PortfolioMonthlyReportHandler serves the MSP-wide monthly report as
// JSON.
func PortfolioMonthlyReportHandler(deps ReportDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := buildPortfolioReport(r, deps)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, toPortfolioReportDTO(report))
	}
}

// PortfolioMonthlyReportPDFHandler serves the same report as a PDF
// download.
func PortfolioMonthlyReportPDFHandler(deps ReportDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := buildPortfolioReport(r, deps)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		pdf := reportpdf.RenderPortfolioReportPDF(report, deps.DemoMode)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="portfolio-%04d-%02d.pdf"`, report.Year, report.Month))
		if err := pdf.Output(w); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}
}
