package report

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"

	"flexapp-vuln-scanner/internal/coverage"
	"flexapp-vuln-scanner/internal/resolve"
)

// liquidwareLogoPNG is the white/on-brand-blue wordmark variant (rendered
// from web/frontend/src/assets/images/logo-primary-light.svg -- the same
// asset the Angular app's own header bar uses over the same blue), so
// the PDF report and the web UI show the identical mark rather than two
// different logo treatments. fpdf can't embed SVG directly, hence the
// pre-rendered PNG.
//
//go:embed assets/liquidware-logo-white.png
var liquidwareLogoPNG []byte

const liquidwareLogoAspect = 700.0 / 222.0 // width/height of the embedded PNG

// Brand tokens (Liquidware style guide, colors_and_type.css) -- see the
// project brief's "use the tokens as given" rule; not invented here.
var (
	brandPrimary  = [3]int{0x00, 0x61, 0xa0} // --p-primary-600, this project's header/accent blue
	brandTextDark = [3]int{0x27, 0x27, 0x2a} // --p-surface-800, body/heading text
)

const brandBannerHeightMM = 22.0

// severityColor maps a severity level to an RGB triple, matching the
// Liquidware brand tokens used elsewhere in this project: CVE severity
// maps to the brand's "poor"/"fair" reds and ambers only -- never
// "good" green, since no CVE severity is an honest "good".
func severityColor(level string) (r, g, b int) {
	switch strings.ToUpper(level) {
	case "CRITICAL":
		return 0x7f, 0x1d, 0x1d
	case "HIGH":
		return 0xdc, 0x26, 0x26
	case "MEDIUM", "MODERATE":
		return 0xca, 0x8a, 0x04
	case "LOW":
		return 0x71, 0x71, 0x7a
	default:
		return 0x37, 0x41, 0x51
	}
}

// PackageMeta is the subset of Stage 1 package metadata the PDF report
// header shows.
type PackageMeta struct {
	VersionMajorMinorBuildRevision string
	ScanFinishedUTC                string
}

// renderBrandBanner draws the running Liquidware-blue header banner with
// the wordmark, repeated on every page via SetHeaderFunc -- the same
// treatment the Angular app's own top nav bar uses, so a printed/PDF
// report and the live UI read as the same product rather than a
// generic, unbranded document bolted on the side.
func renderBrandBanner(pdf *fpdf.Fpdf) {
	pageW, _ := pdf.GetPageSize()
	pdf.SetFillColor(brandPrimary[0], brandPrimary[1], brandPrimary[2])
	pdf.Rect(0, 0, pageW, brandBannerHeightMM, "F")

	const imgName = "liquidware-logo"
	opts := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
	if pdf.GetImageInfo(imgName) == nil {
		pdf.RegisterImageOptionsReader(imgName, opts, bytes.NewReader(liquidwareLogoPNG))
	}
	logoH := 9.0
	logoW := logoH * liquidwareLogoAspect
	pdf.ImageOptions(imgName, 15, (brandBannerHeightMM-logoH)/2, logoW, logoH, false, opts, 0, "")
}

// RenderPDF writes a combined coverage + findings PDF report to outPath.
// This is a presentation-layer alternative for a human reader -- the
// Markdown/JSON outputs remain the source of truth.
func RenderPDF(outPath, packageName string, meta PackageMeta, cov coverage.Coverage, vulnMatches *resolve.Result) error {
	pdf := fpdf.New("P", "mm", "Letter", "")
	pdf.SetMargins(15, brandBannerHeightMM+8, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.SetHeaderFunc(func() { renderBrandBanner(pdf) })
	pdf.AddPage()

	pdf.SetTextColor(brandPrimary[0], brandPrimary[1], brandPrimary[2])
	pdf.SetFont("Helvetica", "B", 20)
	pdf.MultiCell(0, 9, "FlexApp Vulnerability and Security Scan Report", "", "L", false)
	pdf.SetTextColor(brandTextDark[0], brandTextDark[1], brandTextDark[2])
	pdf.SetFont("Helvetica", "B", 14)
	pdf.MultiCell(0, 7, packageName, "", "L", false)

	var metaBits []string
	if meta.VersionMajorMinorBuildRevision != "" {
		metaBits = append(metaBits, "Package version: "+meta.VersionMajorMinorBuildRevision)
	}
	if meta.ScanFinishedUTC != "" {
		metaBits = append(metaBits, "Scan completed: "+meta.ScanFinishedUTC)
	}
	if len(metaBits) > 0 {
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 5, strings.Join(metaBits, "   |   "), "", "L", false)
	}
	pdf.Ln(3)

	renderCoverageSection(pdf, cov)

	pdf.AddPage()
	renderFindingsSection(pdf, vulnMatches)

	return pdf.OutputFileAndClose(outPath)
}

func h2(pdf *fpdf.Fpdf, text string) {
	pdf.Ln(2)
	pdf.SetTextColor(brandPrimary[0], brandPrimary[1], brandPrimary[2])
	pdf.SetFont("Helvetica", "B", 15)
	pdf.MultiCell(0, 8, text, "", "L", false)
	pdf.SetTextColor(brandTextDark[0], brandTextDark[1], brandTextDark[2])
	pdf.SetFont("Helvetica", "", 10)
}

func h3(pdf *fpdf.Fpdf, text string) {
	pdf.Ln(1)
	pdf.SetTextColor(brandPrimary[0], brandPrimary[1], brandPrimary[2])
	pdf.SetFont("Helvetica", "B", 12)
	pdf.MultiCell(0, 6, text, "", "L", false)
	pdf.SetTextColor(brandTextDark[0], brandTextDark[1], brandTextDark[2])
	pdf.SetFont("Helvetica", "", 10)
}

func kvTable(pdf *fpdf.Fpdf, header []string, rows [][2]string, headerRow bool) {
	col1, col2 := 110.0, 40.0
	if headerRow {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetFillColor(brandTextDark[0], brandTextDark[1], brandTextDark[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.CellFormat(col1, 7, header[0], "1", 0, "L", true, 0, "")
		pdf.CellFormat(col2, 7, header[1], "1", 1, "L", true, 0, "")
		pdf.SetTextColor(brandTextDark[0], brandTextDark[1], brandTextDark[2])
		pdf.SetFont("Helvetica", "", 10)
	}
	for _, r := range rows {
		pdf.CellFormat(col1, 7, r[0], "1", 0, "L", false, 0, "")
		pdf.CellFormat(col2, 7, r[1], "1", 1, "L", false, 0, "")
	}
}

func renderCoverageSection(pdf *fpdf.Fpdf, cov coverage.Coverage) {
	h2(pdf, "Resolution Coverage")

	pctStr := "N/A (no candidate components found)"
	if cov.CoveragePercent != nil {
		pctStr = fmt.Sprintf("%.1f%%", *cov.CoveragePercent)
	}
	pdf.SetFont("Helvetica", "B", 11)
	pdf.MultiCell(0, 6, "Resolution coverage: "+pctStr, "", "L", false)
	pdf.SetFont("Helvetica", "", 10)
	pdf.Ln(1)

	kvTable(pdf, nil, [][2]string{
		{"Total files scanned", fmt.Sprint(cov.TotalFilesScanned)},
		{"Files excluded (noise filtering)", fmt.Sprint(cov.ExcludedCount)},
		{"Candidate components", fmt.Sprint(cov.CandidateComponents)},
		{"Components resolved", fmt.Sprint(cov.ResolvedComponents)},
		{"Components unresolved", fmt.Sprint(cov.UnresolvedComponents)},
	}, false)

	if len(cov.ResolvedByMethod) > 0 {
		h3(pdf, "Resolved components, by method")
		var rows [][2]string
		for _, method := range sortedStringKeys(cov.ResolvedByMethod) {
			rows = append(rows, [2]string{method, fmt.Sprint(cov.ResolvedByMethod[method])})
		}
		kvTable(pdf, []string{"Method", "Count"}, rows, true)
	}

	if len(cov.ExcludedByReason) > 0 {
		h3(pdf, "Excluded files, by reason")
		var rows [][2]string
		for _, reason := range sortedStringKeys(cov.ExcludedByReason) {
			rows = append(rows, [2]string{reason, fmt.Sprint(cov.ExcludedByReason[reason])})
		}
		kvTable(pdf, []string{"Reason", "Count"}, rows, true)
	}

	if len(cov.UnresolvedFiles) > 0 {
		h3(pdf, fmt.Sprintf("Unresolved components (%d)", len(cov.UnresolvedFiles)))
		pdf.SetFont("Helvetica", "", 9)
		for _, f := range cov.UnresolvedFiles {
			pdf.MultiCell(0, 5, fmt.Sprintf("%s  (%s)", f.RelativePath, f.ComponentType), "", "L", false)
		}
		pdf.SetFont("Helvetica", "", 10)
	}
}

func renderFindingsSection(pdf *fpdf.Fpdf, vulnMatches *resolve.Result) {
	h2(pdf, "Vulnerability Findings")

	if vulnMatches == nil {
		pdf.MultiCell(0, 6, "No vulnerability-matching data was supplied for this report "+
			"(the resolve step wasn't run, or its output wasn't passed in). This is not "+
			"the same thing as \"no vulnerabilities found.\"", "", "L", false)
		return
	}

	rows := BuildFindingRows(vulnMatches)
	confirmed, heuristic := SplitByConfidence(rows)

	h3(pdf, fmt.Sprintf("Confirmed matches — exact-purl / mapped-cpe (%d)", len(confirmed)))
	renderFindingsBlocks(pdf, confirmed)

	pdf.Ln(3)
	h3(pdf, fmt.Sprintf("Low-confidence matches — heuristic (%d)", len(heuristic)))
	pdf.SetFont("Helvetica", "", 9)
	pdf.MultiCell(0, 5, "These matches used automatic vendor/product normalization, not a "+
		"verified purl or a curated CPE mapping. Verify manually before treating any of "+
		"these as a confirmed finding.", "", "L", false)
	pdf.SetFont("Helvetica", "", 10)
	renderFindingsBlocks(pdf, heuristic)
}

// renderFindingsBlocks renders each finding as a compact block (rather
// than a strict fixed-height grid table, which fpdf has no native
// variable-row-height support for) -- same information as the
// Markdown/CSV renderers' table, just laid out as stacked entries.
func renderFindingsBlocks(pdf *fpdf.Fpdf, rows []FindingRow) {
	if len(rows) == 0 {
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 6, "None.", "", "L", false)
		return
	}

	for _, r := range rows {
		severity := r.SeverityLevel
		if severity == "" {
			severity = "UNKNOWN"
		}
		sr, sg, sb := severityColor(severity)

		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetTextColor(sr, sg, sb)
		pdf.CellFormat(25, 6, severity, "", 0, "L", false, 0, "")
		pdf.SetTextColor(brandTextDark[0], brandTextDark[1], brandTextDark[2])

		title := fmt.Sprintf("%s — %s v%s", r.ID, r.Product, r.Version)
		if r.URL != "" {
			pdf.WriteLinkString(6, title, r.URL)
			pdf.Ln(6)
		} else {
			pdf.MultiCell(0, 6, title, "", "L", false)
		}

		pdf.SetFont("Helvetica", "", 9)
		if r.Summary != "" {
			pdf.MultiCell(0, 5, r.Summary, "", "L", false)
		}
		filesLine := "Affected files: —"
		if len(r.RelativePaths) > 0 {
			filesLine = "Affected files: " + strings.Join(r.RelativePaths, "; ")
		}
		pdf.MultiCell(0, 5, filesLine, "", "L", false)
		pdf.MultiCell(0, 5, fmt.Sprintf("Source: %s   Confidence: %s", r.Source, r.Confidence), "", "L", false)
		pdf.Ln(2)
	}
}
