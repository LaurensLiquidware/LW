package reportpdf

import (
	"bytes"
	"strings"
	"testing"

	"profileunity-msp-console/internal/dashboard"
	"profileunity-msp-console/internal/tenant"
)

func TestCoverageLabel(t *testing.T) {
	cases := []struct {
		status dashboard.CoverageStatus
		want   string
	}{
		{dashboard.CoverageComplete, "Complete — every day this month was collected successfully"},
		{dashboard.CoveragePartial, "Partial — some days this month were not collected successfully"},
		{dashboard.CoverageNone, "None — no successful collection this month; figures below are not meaningful"},
	}
	for _, c := range cases {
		if got := coverageLabel(c.status); got != c.want {
			t.Errorf("coverageLabel(%q) = %q, want %q", c.status, got, c.want)
		}
	}
}

func TestFmtInt(t *testing.T) {
	if got := fmtInt(nil); got != "unknown" {
		t.Errorf("fmtInt(nil) = %q, want %q", got, "unknown")
	}
	v := 500
	if got := fmtInt(&v); got != "500" {
		t.Errorf("fmtInt(&500) = %q, want %q", got, "500")
	}
}

func TestFmtAvg(t *testing.T) {
	if got := fmtAvg(nil); got != "unknown" {
		t.Errorf("fmtAvg(nil) = %q, want %q", got, "unknown")
	}
	v := 12.34
	if got := fmtAvg(&v); got != "12.3" {
		t.Errorf("fmtAvg(&12.34) = %q, want %q", got, "12.3")
	}
}

func TestFmtProduct(t *testing.T) {
	if got := fmtProduct(""); got != "unknown" {
		t.Errorf("fmtProduct(\"\") = %q, want %q", got, "unknown")
	}
	if got := fmtProduct("ProU+FlexApp"); got != "ProU+FlexApp" {
		t.Errorf("fmtProduct(\"ProU+FlexApp\") = %q, want %q", got, "ProU+FlexApp")
	}
}

func TestSafeFilenamePart(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Acme Corp", "Acme-Corp"},
		{"../../etc/passwd", "..-..-etc-passwd"},
		{"", "tenant"},
		{"!!!", "tenant"},
		{"already-safe_name.txt", "already-safe_name.txt"},
	}
	for _, c := range cases {
		if got := SafeFilenamePart(c.in); got != c.want {
			t.Errorf("SafeFilenamePart(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func fullTenantReport() dashboard.TenantMonthlyReport {
	maxUsers := 500
	entitled := 420
	peak := 480
	avg := 410.5
	return dashboard.TenantMonthlyReport{
		Tenant:                   tenant.Tenant{DisplayName: "Acme Corp"},
		Year:                     2026,
		Month:                    7,
		DaysInMonth:              31,
		DaysCollected:            30,
		DaysFailed:               1,
		Coverage:                 dashboard.CoveragePartial,
		PeakUsed:                 &peak,
		PeakUsedDate:             "2026-07-15",
		AverageUsed:              &avg,
		EntitledAtMonthEnd:       &entitled,
		MaximumUsersAtMonthEnd:   &maxUsers,
		LicenseProductAtMonthEnd: "ProU+FlexApp",
		EntitlementChanges: []dashboard.EntitlementChange{
			{Date: "2026-07-10", FromTotal: 400, ToTotal: 500},
		},
	}
}

func emptyTenantReport() dashboard.TenantMonthlyReport {
	return dashboard.TenantMonthlyReport{
		Tenant:   tenant.Tenant{DisplayName: "No Data Tenant"},
		Year:     2026,
		Month:    7,
		Coverage: dashboard.CoverageNone,
	}
}

func TestRenderTenantReportPDF_ProducesValidOutput(t *testing.T) {
	pdf := RenderTenantReportPDF(fullTenantReport(), false)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() = true, want false; details: %v", pdf.Error())
	}
	if buf.Len() == 0 {
		t.Fatal("rendered PDF is empty")
	}
	if !strings.HasPrefix(buf.String(), "%PDF") {
		t.Fatalf("rendered output does not start with %%PDF magic bytes: %q", buf.String()[:min(20, buf.Len())])
	}
}

func TestRenderTenantReportPDF_NoDataDoesNotPanic(t *testing.T) {
	pdf := RenderTenantReportPDF(emptyTenantReport(), false)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() = true, want false; details: %v", pdf.Error())
	}
	if buf.Len() == 0 {
		t.Fatal("rendered PDF is empty")
	}
}

func TestRenderPortfolioReportPDF_ProducesValidOutput(t *testing.T) {
	totalMax := 1200
	totalEntitled := 1000
	peak := 1100
	avg := 990.5
	r := dashboard.PortfolioMonthlyReport{
		Year:                        2026,
		Month:                       7,
		TenantsRegistered:           3,
		PeakTotalUsed:               &peak,
		PeakTotalUsedDate:           "2026-07-15",
		AverageTotalUsed:            &avg,
		TotalEntitledAtMonthEnd:     &totalEntitled,
		TotalMaximumUsersAtMonthEnd: &totalMax,
		TenantReports:               []dashboard.TenantMonthlyReport{fullTenantReport(), emptyTenantReport()},
	}

	pdf := RenderPortfolioReportPDF(r, false)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() = true, want false; details: %v", pdf.Error())
	}
	if buf.Len() == 0 {
		t.Fatal("rendered PDF is empty")
	}
	if !strings.HasPrefix(buf.String(), "%PDF") {
		t.Fatalf("rendered output does not start with %%PDF magic bytes: %q", buf.String()[:min(20, buf.Len())])
	}
}

func TestReportTitle(t *testing.T) {
	if got := reportTitle("Monthly Report — Acme", false); got != "Monthly Report — Acme" {
		t.Errorf("reportTitle(..., false) = %q, want no watermark appended", got)
	}
	want := "Monthly Report — Acme — " + demoWatermark
	if got := reportTitle("Monthly Report — Acme", true); got != want {
		t.Errorf("reportTitle(..., true) = %q, want %q", got, want)
	}
}

func TestFooterText(t *testing.T) {
	if got := footerText(3, false); got != "ProfileUnity MSP Licensing Console — Page 3 of {nb}" {
		t.Errorf("footerText(3, false) = %q, want the normal product-name footer", got)
	}
	want := demoWatermark + " — Page 3 of {nb}"
	if got := footerText(3, true); got != want {
		t.Errorf("footerText(3, true) = %q, want %q", got, want)
	}
}

// TestRenderTenantReportPDF_DemoModeStillProducesValidOutput is a smoke
// test only -- reportTitle/footerText above cover the actual watermark
// content; rendered PDF bytes for an embedded UTF-8 TrueType font aren't
// literal ASCII in the content stream, so they can't be substring-matched
// here without shelling out to an external tool (deliberately avoided for
// this package, see report_pdf_test.go's other Render* tests).
func TestRenderTenantReportPDF_DemoModeStillProducesValidOutput(t *testing.T) {
	pdf := RenderTenantReportPDF(fullTenantReport(), true)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() = true, want false; details: %v", pdf.Error())
	}
	if buf.Len() == 0 {
		t.Fatal("rendered PDF is empty")
	}
}
