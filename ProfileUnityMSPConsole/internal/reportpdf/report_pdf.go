package reportpdf

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
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

// Branding is the MSP operator's own company name/logo (set from the
// Settings screen, see internal/settings.Settings.CompanyName/
// CompanyLogoImage), drawn alongside -- never replacing -- Liquidware's
// own header branding. The zero value means no MSP branding is
// configured, and newBrandedHeaderFunc renders exactly as it always has.
type Branding struct {
	CompanyName string
	LogoImage   []byte
	// LogoImageType is "png" or "jpg", matching fpdf.ImageOptions.ImageType.
	LogoImageType string
}

// mspLogoImageName is a fixed fpdf image name for the MSP's logo. Safe to
// reuse across renders/pages: fpdf's image registry is per-Fpdf instance,
// and newReportPDF registers (or skips) it once per document.
const mspLogoImageName = "msp-logo"

// mspStripHeight is a second, shorter band directly below Liquidware's
// own header band, added only when MSP branding is configured -- a
// distinct row, not squeezed sideways into Liquidware's own title area,
// so there's no risk of the two overlapping regardless of how wide the
// MSP's name or logo is. This is what "alongside, not replacing" means
// in practice: Liquidware's logo/title are drawn completely unchanged,
// at their original size and position.
const mspStripHeight = 12.0

// mspLogoMaxHeight/mspLogoMaxWidth bound the MSP logo's drawn size within
// the strip -- height first (leaves visible padding above/below), then
// width (so an extremely wide/panoramic image can't dominate the strip).
const (
	mspLogoMaxHeight = 8.0
	mspLogoMaxWidth  = 60.0
)

const mspBrandFontSize = 11.0

// hasMSPBranding reports whether branding carries anything to draw.
func hasMSPBranding(branding Branding) bool {
	return branding.CompanyName != "" || len(branding.LogoImage) > 0
}

// totalHeaderHeight is brandHeaderHeight, plus mspStripHeight whenever
// MSP branding is configured.
func totalHeaderHeight(branding Branding) float64 {
	if hasMSPBranding(branding) {
		return brandHeaderHeight + mspStripHeight
	}
	return brandHeaderHeight
}

// newBrandedHeaderFunc returns an fpdf header callback that paints the
// brand-blue band, embeds the Liquidware logo, and writes the report's
// title/subtitle in white -- run via SetHeaderFunc, so it repeats
// identically on every page of a multi-page portfolio report, not just
// the first. If branding carries an MSP company name and/or logo, a
// second, shorter band is added directly below, with the MSP's own
// logo/name right-aligned in it -- Liquidware's own logo/title above are
// completely unchanged.
func newBrandedHeaderFunc(pdf *fpdf.Fpdf, title string, branding Branding) func() {
	return func() {
		pageWidth, _ := pdf.GetPageSize()
		totalHeight := totalHeaderHeight(branding)

		pdf.SetFillColor(brandR, brandG, brandB)
		pdf.Rect(0, 0, pageWidth, totalHeight, "F")

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

		if hasMSPBranding(branding) {
			drawMSPBrandingStrip(pdf, pageWidth, branding)
		}

		// AddPage restores the font/color active before it was called,
		// but NOT the cursor position -- without this, body content
		// would start writing from wherever the title text above left
		// it (inside the band) rather than below it, on every page.
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(18, totalHeight+8)
	}
}

// drawMSPBrandingStrip draws the MSP's own logo/name right-aligned in
// the strip directly below Liquidware's own header band (see
// mspStripHeight) -- a distinct row, so it can never overlap Liquidware's
// title/subtitle above it regardless of the MSP's name length or logo
// aspect ratio.
func drawMSPBrandingStrip(pdf *fpdf.Fpdf, pageWidth float64, branding Branding) {
	const margin = 18.0
	stripTop := brandHeaderHeight
	stripCenterY := stripTop + mspStripHeight/2

	logoWidth := 0.0
	logoHeight := 0.0
	if len(branding.LogoImage) > 0 {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(branding.LogoImage))
		if err == nil && cfg.Height > 0 {
			aspect := float64(cfg.Width) / float64(cfg.Height)
			logoHeight, logoWidth = mspLogoMaxHeight, mspLogoMaxHeight*aspect
			if logoWidth > mspLogoMaxWidth {
				logoWidth = mspLogoMaxWidth
				logoHeight = logoWidth / aspect
			}
			pdf.RegisterImageOptionsReader(mspLogoImageName, fpdf.ImageOptions{ImageType: branding.LogoImageType}, bytes.NewReader(branding.LogoImage))
			pdf.ImageOptions(mspLogoImageName, pageWidth-margin-logoWidth, stripCenterY-logoHeight/2, logoWidth, logoHeight, false, fpdf.ImageOptions{ImageType: branding.LogoImageType}, 0, "")
		}
	}

	if branding.CompanyName == "" {
		return
	}
	pdf.SetFont(reportFontFamily, "B", mspBrandFontSize)
	maxNameWidth := pageWidth - 2*margin - logoWidth
	if logoWidth > 0 {
		maxNameWidth -= 4
	}
	name := truncateToWidth(pdf, branding.CompanyName, maxNameWidth)
	nameWidth := pdf.GetStringWidth(name)
	nameX := pageWidth - margin - logoWidth - nameWidth
	if logoWidth > 0 {
		nameX -= 4
	}
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(nameX, stripCenterY-2.5)
	pdf.CellFormat(nameWidth, 5, name, "", 0, "L", false, 0, "")
}

// truncateToWidth shortens s, appending "..." if needed, until it fits
// within maxWidth at the font currently set on pdf -- so an operator's
// company name can never overflow past the page margin regardless of
// length.
func truncateToWidth(pdf *fpdf.Fpdf, s string, maxWidth float64) string {
	if maxWidth <= 0 || pdf.GetStringWidth(s) <= maxWidth {
		return s
	}
	const ellipsis = "..."
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := string(runes) + ellipsis
		if pdf.GetStringWidth(candidate) <= maxWidth {
			return candidate
		}
	}
	return ellipsis
}

// newBrandedFooterFunc returns an fpdf footer callback: a thin brand-blue
// rule and a muted page-number line, repeated on every page.
func newBrandedFooterFunc(pdf *fpdf.Fpdf, demoMode bool) func() {
	return func() {
		pageWidth, pageHeight := pdf.GetPageSize()
		y := pageHeight - 15
		pdf.SetDrawColor(brandR, brandG, brandB)
		pdf.SetLineWidth(0.4)
		pdf.Line(18, y, pageWidth-18, y)

		pdf.SetTextColor(130, 130, 130)
		pdf.SetFont(reportFontFamily, "", 8)
		pdf.SetXY(18, y+2)
		pdf.CellFormat(pageWidth-36, 5, footerText(pdf.PageNo(), demoMode), "", 0, "L", false, 0, "")
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

func fmtProduct(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}

// demoWatermark is appended to a report's header title and substituted
// into its footer whenever it's rendered from a demo.db sidecar database
// (see cmd/server/main.go's DemoMode plumbing) -- a demo report forwarded
// to a customer as real is a foreseeable accident this exists to prevent.
const demoWatermark = "DEMO DATA — not a real customer report"

// reportTitle appends demoWatermark to title when demoMode is set. Pure
// and unexported so it's testable directly, the same way this file tests
// coverageLabel/fmtInt/fmtAvg/fmtProduct, rather than only by parsing
// rendered PDF bytes (which, for an embedded UTF-8 TrueType font, aren't
// literal ASCII in the content stream).
func reportTitle(title string, demoMode bool) string {
	if demoMode {
		return title + " — " + demoWatermark
	}
	return title
}

// footerText builds the per-page footer line, substituting demoWatermark
// for the normal product name whenever demoMode is set. See reportTitle's
// doc comment for why this is a separate pure function.
func footerText(pageNo int, demoMode bool) string {
	if demoMode {
		return fmt.Sprintf("%s — Page %d of {nb}", demoWatermark, pageNo)
	}
	return fmt.Sprintf("ProfileUnity MSP Licensing Console — Page %d of {nb}", pageNo)
}

func newReportPDF(title string, demoMode bool, branding Branding) *fpdf.Fpdf {
	title = reportTitle(title, demoMode)
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes(reportFontFamily, "", dejaVuSansRegular)
	pdf.AddUTF8FontFromBytes(reportFontFamily, "B", dejaVuSansBold)
	pdf.RegisterImageOptionsReader(logoImageName, fpdf.ImageOptions{ImageType: "png"}, bytes.NewReader(liquidwareLogoWhite))

	pdf.SetMargins(18, totalHeaderHeight(branding)+8, 18)
	pdf.SetAutoPageBreak(true, 22)
	pdf.AliasNbPages("{nb}")
	pdf.SetHeaderFunc(newBrandedHeaderFunc(pdf, title, branding))
	pdf.SetFooterFunc(newBrandedFooterFunc(pdf, demoMode))
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
	writeStatLine(pdf, "Maximum users:", fmtInt(r.MaximumUsersAtMonthEnd))
	writeStatLine(pdf, "Product:", fmtProduct(r.LicenseProductAtMonthEnd))
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

// RenderTenantReportPDF renders a single tenant's monthly report. demoMode
// watermarks the header/footer with demoWatermark -- set it whenever r was
// built from a demo.db sidecar database. branding draws the MSP
// operator's own company name/logo alongside Liquidware's own header
// branding -- pass the zero Branding{} if none is configured.
func RenderTenantReportPDF(r dashboard.TenantMonthlyReport, demoMode bool, branding Branding) *fpdf.Fpdf {
	pdf := newReportPDF(fmt.Sprintf("Monthly Report — %s", r.Tenant.DisplayName), demoMode, branding)
	writeTenantReportBody(pdf, r)
	return pdf
}

// RenderPortfolioReportPDF renders the MSP-wide summary followed by each
// tenant's own detail section, so a single download (or emailed
// attachment) covers everything an operator needs for the month. demoMode
// watermarks the header/footer, same as RenderTenantReportPDF. branding is
// also the same -- see RenderTenantReportPDF.
func RenderPortfolioReportPDF(r dashboard.PortfolioMonthlyReport, demoMode bool, branding Branding) *fpdf.Fpdf {
	pdf := newReportPDF(fmt.Sprintf("Monthly Portfolio Report — %04d-%02d", r.Year, r.Month), demoMode, branding)

	writeSectionHeading(pdf, "Portfolio summary")
	writeStatLine(pdf, "Tenants registered:", fmt.Sprintf("%d", r.TenantsRegistered))
	peak := fmtInt(r.PeakTotalUsed)
	if r.PeakTotalUsed != nil {
		peak = fmt.Sprintf("%s (on %s)", peak, r.PeakTotalUsedDate)
	}
	writeStatLine(pdf, "Peak total used licenses:", peak)
	writeStatLine(pdf, "Average total used licenses:", fmtAvg(r.AverageTotalUsed))
	writeStatLine(pdf, "Total entitled at month end:", fmtInt(r.TotalEntitledAtMonthEnd))
	writeStatLine(pdf, "Total maximum users:", fmtInt(r.TotalMaximumUsersAtMonthEnd))
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
