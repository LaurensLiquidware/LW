package httpapi

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-pdf/fpdf"

	"profileunity-msp-console/internal/dashboard"
)

// coverageLabel renders a CoverageStatus in the plain-English wording a
// report reader needs, not the machine-readable enum value — project
// brief §7.5 requires reports to say explicitly how much of the month's
// data can be trusted.
func coverageLabel(c dashboard.CoverageStatus) string {
	switch c {
	case dashboard.CoverageComplete:
		return "Complete — every day this month was collected successfully"
	case dashboard.CoveragePartial:
		return "Partial — some days this month were not collected successfully"
	default:
		return "None — no successful collection this month; figures below are not meaningful"
	}
}

func fmtInt(v *int) string {
	if v == nil {
		return "unknown"
	}
	return fmt.Sprintf("%d", *v)
}

func fmtAvg(v *float64) string {
	if v == nil {
		return "unknown"
	}
	return fmt.Sprintf("%.1f", *v)
}

func newReportPDF(title string) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(18, 18, 18)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "ProfileUnity MSP Licensing Console", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 7, title, "", 1, "L", false, 0, "")
	pdf.Ln(4)
	return pdf
}

func writeSectionHeading(pdf *fpdf.Fpdf, text string) {
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, text, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
}

func writeStatLine(pdf *fpdf.Fpdf, label, value string) {
	pdf.CellFormat(60, 6, label, "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 6, value, "", 1, "L", false, 0, "")
}

func writeTenantReportBody(pdf *fpdf.Fpdf, r dashboard.TenantMonthlyReport) {
	writeSectionHeading(pdf, fmt.Sprintf("%s — %04d-%02d", r.Tenant.DisplayName, r.Year, r.Month))
	writeStatLine(pdf, "Data coverage:", coverageLabel(r.Coverage))
	writeStatLine(pdf, "Days collected / in month:", fmt.Sprintf("%d / %d (%d failed, %d never attempted)", r.DaysCollected, r.DaysInMonth, r.DaysFailed, r.DaysNeverAttempted))
	peak := fmtInt(r.PeakUsed)
	if r.PeakUsed != nil {
		peak = fmt.Sprintf("%s (on %s)", peak, r.PeakUsedDate)
	}
	writeStatLine(pdf, "Peak used licenses:", peak)
	writeStatLine(pdf, "Average used licenses:", fmtAvg(r.AverageUsed))
	writeStatLine(pdf, "Entitled at month end:", fmtInt(r.EntitledAtMonthEnd))
	pdf.Ln(2)

	if len(r.EntitlementChanges) == 0 {
		pdf.SetFont("Helvetica", "I", 10)
		pdf.CellFormat(0, 6, "No entitlement changes this month.", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
	} else {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(0, 6, "Entitlement changes:", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		for _, c := range r.EntitlementChanges {
			pdf.CellFormat(0, 6, fmt.Sprintf("  %s: %d -> %d", c.Date, c.FromTotal, c.ToTotal), "", 1, "L", false, 0, "")
		}
	}
	pdf.Ln(4)
}

// renderTenantReportPDF renders a single tenant's monthly report.
func renderTenantReportPDF(r dashboard.TenantMonthlyReport) *fpdf.Fpdf {
	pdf := newReportPDF(fmt.Sprintf("Monthly Report — %s", r.Tenant.DisplayName))
	writeTenantReportBody(pdf, r)
	return pdf
}

// renderPortfolioReportPDF renders the MSP-wide summary followed by each
// tenant's own detail section, so a single download covers everything an
// operator needs for the month.
func renderPortfolioReportPDF(r dashboard.PortfolioMonthlyReport) *fpdf.Fpdf {
	pdf := newReportPDF(fmt.Sprintf("Monthly Portfolio Report — %04d-%02d", r.Year, r.Month))

	writeSectionHeading(pdf, "Portfolio summary")
	writeStatLine(pdf, "Tenants registered:", fmt.Sprintf("%d", r.TenantsRegistered))
	peak := fmtInt(r.PeakTotalUsed)
	if r.PeakTotalUsed != nil {
		peak = fmt.Sprintf("%s (on %s)", peak, r.PeakTotalUsedDate)
	}
	writeStatLine(pdf, "Peak total used licenses:", peak)
	writeStatLine(pdf, "Average total used licenses:", fmtAvg(r.AverageTotalUsed))
	writeStatLine(pdf, "Total entitled at month end:", fmtInt(r.TotalEntitledAtMonthEnd))
	pdf.Ln(6)

	writeSectionHeading(pdf, "Per-tenant detail")
	for _, tr := range r.TenantReports {
		writeTenantReportBody(pdf, tr)
	}
	return pdf
}

var unsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// safeFilenamePart strips anything but a conservative filename-safe
// character set from a user-supplied display name before it goes into a
// Content-Disposition header — that header value is otherwise attacker-
// controlled input (a tenant's display name) reflected straight into an
// HTTP response.
func safeFilenamePart(s string) string {
	cleaned := unsafeFilenameChars.ReplaceAllString(s, "-")
	cleaned = strings.Trim(cleaned, "-")
	if cleaned == "" {
		return "tenant"
	}
	return cleaned
}
