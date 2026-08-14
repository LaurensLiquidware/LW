package reportpdf

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	_ "embed"

	"github.com/go-pdf/fpdf"

	"profileunity-msp-console/internal/dashboard"
)

// A UTF-8 TrueType font, not one of fpdf's built-in core fonts (Helvetica
// et al.): those only support single-byte Latin-1/WinAnsi text, and
// writing raw UTF-8 tenant/report text through them corrupts anything
// outside that range into mojibake rather than merely dropping it —
// found during the project brief §11 Unicode/i18n compliance pass.
// DejaVu Sans (Bitstream Vera-derived, permissively licensed — see
// fonts/DEJAVU-LICENSE.txt) covers Latin Extended, Cyrillic, Greek, and
// Vietnamese correctly. CJK, Arabic, and Hebrew are not covered — a
// CJK-capable font is tens of megabytes and out of scope for a
// self-contained single binary; this is a known, documented limitation.
//
//go:embed fonts/DejaVuSans.ttf
var dejaVuSansRegular []byte

//go:embed fonts/DejaVuSans-Bold.ttf
var dejaVuSansBold []byte

const reportFontFamily = "DejaVuSans"

// Liquidware wordmark, white-on-transparent for use on the brand-blue
// header band below -- fpdf embeds raster images only, so this is a PNG
// rendered from the same assets/images/logo-primary-light.svg the web UI
// uses, not a separate design. See docs/design-system-reference for the
// brand colors this file's header/section styling matches (the console
// header's --header-bg / --p-primary-600).
//
//go:embed images/liquidware-logo-white.png
var liquidwareLogoWhite []byte

const (
	logoImageName             = "liquidware-logo-white"
	logoAspectWidthOverHeight = 890.0 / 217.0

	// #0061A0 -- matches the web console header's --header-bg exactly,
	// so a printed report and the app it came from read as one product.
	brandR, brandG, brandB = 0, 97, 160

	brandHeaderHeight = 24.0
	brandLogoHeight   = 10.0
)

// newBrandedHeaderFunc returns an fpdf header callback that paints the
// brand-blue band, embeds the Liquidware logo, and writes the report's
// title/subtitle in white -- run via SetHeaderFunc, so it repeats
// identically on every page of a multi-page portfolio report, not just
// the first.
func newBrandedHeaderFunc(pdf *fpdf.Fpdf, title string) func() {
	return func() {
		pageWidth, _ := pdf.GetPageSize()

		pdf.SetFillColor(brandR, brandG, brandB)
		pdf.Rect(0, 0, pageWidth, brandHeaderHeight, "F")

		logoWidth := brandLogoHeight * logoAspectWidthOverHeight
		pdf.ImageOptions(logoImageName, 18, (brandHeaderHeight-brandLogoHeight)/2, logoWidth, brandLogoHeight, false, fpdf.ImageOptions{ImageType: "png"}, 0, "")

		textX := 18 + logoWidth + 6
		textWidth := pageWidth - textX - 18
		pdf.SetTextColor(255, 255, 255)
		pdf.SetXY(textX, 5.5)
		pdf.SetFont(reportFontFamily, "B", 13)
		pdf.CellFormat(textWidth, 6, "ProfileUnity MSP Licensing Console", "", 0, "L", false, 0, "")
		pdf.SetXY(textX, 13)
		pdf.SetFont(reportFontFamily, "", 10)
		pdf.CellFormat(textWidth, 6, title, "", 0, "L", false, 0, "")

		// AddPage restores the font/color active before it was called,
		// but NOT the cursor position -- without this, body content
		// would start writing from wherever the title text above left
		// it (inside the band) rather than below it, on every page.
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(18, brandHeaderHeight+8)
	}
}

// newBrandedFooterFunc returns an fpdf footer callback: a thin brand-blue
// rule and a muted page-number line, repeated on every page.
func newBrandedFooterFunc(pdf *fpdf.Fpdf) func() {
	return func() {
		pageWidth, pageHeight := pdf.GetPageSize()
		y := pageHeight - 15
		pdf.SetDrawColor(brandR, brandG, brandB)
		pdf.SetLineWidth(0.4)
		pdf.Line(18, y, pageWidth-18, y)

		pdf.SetTextColor(130, 130, 130)
		pdf.SetFont(reportFontFamily, "", 8)
		pdf.SetXY(18, y+2)
		pdf.CellFormat(pageWidth-36, 5, fmt.Sprintf("ProfileUnity MSP Licensing Console — Page %d of {nb}", pdf.PageNo()), "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
}

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
	pdf.AddUTF8FontFromBytes(reportFontFamily, "", dejaVuSansRegular)
	pdf.AddUTF8FontFromBytes(reportFontFamily, "B", dejaVuSansBold)
	pdf.RegisterImageOptionsReader(logoImageName, fpdf.ImageOptions{ImageType: "png"}, bytes.NewReader(liquidwareLogoWhite))

	pdf.SetMargins(18, brandHeaderHeight+8, 18)
	pdf.SetAutoPageBreak(true, 22)
	pdf.AliasNbPages("{nb}")
	pdf.SetHeaderFunc(newBrandedHeaderFunc(pdf, title))
	pdf.SetFooterFunc(newBrandedFooterFunc(pdf))
	pdf.AddPage()
	return pdf
}

func writeSectionHeading(pdf *fpdf.Fpdf, text string) {
	pdf.SetFont(reportFontFamily, "B", 12)
	pdf.SetTextColor(brandR, brandG, brandB)
	pdf.CellFormat(0, 8, text, "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont(reportFontFamily, "", 10)
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
	writeStatLine(pdf, "Maximum users at month end:", fmtInt(r.MaximumUsersAtMonthEnd))
	pdf.Ln(2)

	if len(r.EntitlementChanges) == 0 {
		pdf.CellFormat(0, 6, "No entitlement changes this month.", "", 1, "L", false, 0, "")
	} else {
		pdf.SetFont(reportFontFamily, "B", 10)
		pdf.CellFormat(0, 6, "Entitlement changes:", "", 1, "L", false, 0, "")
		pdf.SetFont(reportFontFamily, "", 10)
		for _, c := range r.EntitlementChanges {
			pdf.CellFormat(0, 6, fmt.Sprintf("  %s: %d -> %d", c.Date, c.FromTotal, c.ToTotal), "", 1, "L", false, 0, "")
		}
	}
	pdf.Ln(4)
}

// RenderTenantReportPDF renders a single tenant's monthly report.
func RenderTenantReportPDF(r dashboard.TenantMonthlyReport) *fpdf.Fpdf {
	pdf := newReportPDF(fmt.Sprintf("Monthly Report — %s", r.Tenant.DisplayName))
	writeTenantReportBody(pdf, r)
	return pdf
}

// RenderPortfolioReportPDF renders the MSP-wide summary followed by each
// tenant's own detail section, so a single download (or emailed
// attachment) covers everything an operator needs for the month.
func RenderPortfolioReportPDF(r dashboard.PortfolioMonthlyReport) *fpdf.Fpdf {
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
	writeStatLine(pdf, "Total maximum users at month end:", fmtInt(r.TotalMaximumUsersAtMonthEnd))
	pdf.Ln(6)

	writeSectionHeading(pdf, "Per-tenant detail")
	for _, tr := range r.TenantReports {
		writeTenantReportBody(pdf, tr)
	}
	return pdf
}

var unsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// SafeFilenamePart strips anything but a conservative filename-safe
// character set from a user-supplied display name before it goes into a
// Content-Disposition header — that header value is otherwise attacker-
// controlled input (a tenant's display name) reflected straight into an
// HTTP response.
func SafeFilenamePart(s string) string {
	cleaned := unsafeFilenameChars.ReplaceAllString(s, "-")
	cleaned = strings.Trim(cleaned, "-")
	if cleaned == "" {
		return "tenant"
	}
	return cleaned
}
