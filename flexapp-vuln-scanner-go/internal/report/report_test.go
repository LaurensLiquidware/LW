package report

import (
	"strings"
	"testing"

	"flexapp-vuln-scanner/internal/coverage"
	"flexapp-vuln-scanner/internal/inventory"
	"flexapp-vuln-scanner/internal/resolve"
)

// The test cases here mirror
// ../../../flexapp-vuln-scanner/stage2-resolve/tests/test_reporting.py
// exactly, for output parity with the Python implementation.

const fixturePath = "../inventory/testdata/sample.inventory.json"

func s(v string) *string { return &v }

func comp(relativePath, product, version, purl, cpe, confidence string, vulns ...resolve.Vulnerability) resolve.Component {
	var identity *inventory.Identity
	if product != "" || version != "" {
		identity = &inventory.Identity{}
		if product != "" {
			identity.Product = s(product)
		}
		if version != "" {
			identity.Version = s(version)
		}
	}
	return resolve.Component{
		RelativePath:    relativePath,
		Identity:        identity,
		Purl:            purl,
		CPE:             cpe,
		Confidence:      confidence,
		Vulnerabilities: vulns,
	}
}

func vuln(id, summary, severityLevel, source string) resolve.Vulnerability {
	return resolve.Vulnerability{ID: id, Summary: s(summary), SeverityLevel: s(severityLevel), Source: source}
}

func TestVulnerabilityURL_CVEGoesToNVD(t *testing.T) {
	if got := VulnerabilityURL("CVE-2023-51791"); got != "https://nvd.nist.gov/vuln/detail/CVE-2023-51791" {
		t.Errorf("got %q", got)
	}
}

func TestVulnerabilityURL_GHSAGoesToGithubAdvisories(t *testing.T) {
	if got := VulnerabilityURL("GHSA-aaaa-bbbb-cccc"); got != "https://github.com/advisories/GHSA-aaaa-bbbb-cccc" {
		t.Errorf("got %q", got)
	}
}

func TestVulnerabilityURL_OtherOSVIDsGoToOSVDev(t *testing.T) {
	if got := VulnerabilityURL("PYSEC-2021-1"); got != "https://osv.dev/vulnerability/PYSEC-2021-1" {
		t.Errorf("got %q", got)
	}
}

func TestVulnerabilityURL_EmptyForMissingID(t *testing.T) {
	if got := VulnerabilityURL(""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestBuildFindingRows_IncludesURL(t *testing.T) {
	vm := &resolve.Result{Components: []resolve.Component{
		comp("a.jar", "a", "1.0", "", "", "exact-purl", vuln("CVE-2023-0001", "x", "HIGH", "nvd")),
	}}
	rows := BuildFindingRows(vm)
	if rows[0].URL != "https://nvd.nist.gov/vuln/detail/CVE-2023-0001" {
		t.Errorf("URL = %q", rows[0].URL)
	}
}

func TestBuildFindingRows_CollectsAllAffectedFilesForSharedVulnerability(t *testing.T) {
	v := vuln("CVE-2026-0001", "x", "CRITICAL", "nvd")
	vm := &resolve.Result{Components: []resolve.Component{
		comp(`Program Files\App\outer-app.jar`, "OuterApp", "9.9.9", "pkg:maven/a/outer-app@9.9.9", "", "exact-purl", v),
		comp(`Program Files\App\plugins\outer-app-legacy.jar`, "OuterApp", "9.9.9", "pkg:maven/a/outer-app@9.9.9", "", "exact-purl", v),
		comp(`Data\cache\outer-app.jar.bak`, "OuterApp", "9.9.9", "pkg:maven/a/outer-app@9.9.9", "", "exact-purl", v),
	}}
	rows := BuildFindingRows(vm)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	want := []string{`Data\cache\outer-app.jar.bak`, `Program Files\App\outer-app.jar`, `Program Files\App\plugins\outer-app-legacy.jar`}
	if !equalStrings(rows[0].RelativePaths, want) {
		t.Errorf("RelativePaths = %v, want %v", rows[0].RelativePaths, want)
	}
}

func TestBuildFindingRows_DistinctVulnerabilitiesEachGetTheirOwnFiles(t *testing.T) {
	vm := &resolve.Result{Components: []resolve.Component{
		comp("a.jar", "a", "1.0", "pkg:maven/a/a@1.0", "", "exact-purl",
			vuln("CVE-2026-0001", "x", "CRITICAL", "nvd"),
			vuln("CVE-2026-0002", "y", "HIGH", "nvd"),
		),
	}}
	rows := BuildFindingRows(vm)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if !equalStrings(r.RelativePaths, []string{"a.jar"}) {
			t.Errorf("RelativePaths = %v", r.RelativePaths)
		}
	}
}

func TestBuildFindingRows_NoRelativePathGivesEmptyList(t *testing.T) {
	vm := &resolve.Result{Components: []resolve.Component{
		comp("", "a", "1.0", "pkg:maven/a/a@1.0", "", "exact-purl", vuln("CVE-2023-0001", "x", "HIGH", "nvd")),
	}}
	rows := BuildFindingRows(vm)
	if len(rows[0].RelativePaths) != 0 {
		t.Errorf("RelativePaths = %v, want empty", rows[0].RelativePaths)
	}
}

func TestRenderFindings_ShowsAffectedFilesColumn(t *testing.T) {
	v := vuln("CVE-2026-0001", "x", "CRITICAL", "nvd")
	vm := &resolve.Result{Components: []resolve.Component{
		comp(`a\outer-app.jar`, "OuterApp", "9.9.9", "pkg:maven/a/outer-app@9.9.9", "", "exact-purl", v),
		comp(`b\outer-app-legacy.jar`, "OuterApp", "9.9.9", "pkg:maven/a/outer-app@9.9.9", "", "exact-purl", v),
	}}
	out := RenderFindings(vm, "TestApp")
	if !strings.Contains(out, "Affected Files") {
		t.Error("missing Affected Files column")
	}
	if !strings.Contains(out, "`a\\outer-app.jar`<br>`b\\outer-app-legacy.jar`") {
		t.Error("missing joined affected-files cell")
	}
}

func TestRenderFindings_LinksTheID(t *testing.T) {
	vm := &resolve.Result{Components: []resolve.Component{
		comp("a.jar", "a", "1.0", "", "", "exact-purl", vuln("CVE-2023-0001", "x", "HIGH", "nvd")),
	}}
	out := RenderFindings(vm, "TestApp")
	if !strings.Contains(out, "[CVE-2023-0001](https://nvd.nist.gov/vuln/detail/CVE-2023-0001)") {
		t.Error("missing linked id")
	}
}

func TestRenderCoverageReport_ContainsRequiredSections(t *testing.T) {
	inv, err := inventory.Load(fixturePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cov := coverage.Compute(inv)
	out := RenderCoverageReport(cov, "TestApp")

	for _, want := range []string{
		"TestApp",
		"Resolution coverage: 66.7%",
		"Total files scanned: 4",
		"Files excluded (noise filtering): 1",
		"Candidate components (excluded: false): 3",
		"Components resolved: 2",
		"Components unresolved: 1",
		"os-system-path | 1",
		"jar-pom-properties | 1",
		"unresolved.bin",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in report", want)
		}
	}
}

func TestRenderCoverageReport_ZeroCandidatesSaysNA(t *testing.T) {
	cov := coverage.Coverage{
		TotalFilesScanned:   1,
		ExcludedCount:       1,
		ExcludedByReason:    map[string]int{"font-file": 1},
		CandidateComponents: 0,
		CoveragePercent:     nil,
	}
	out := RenderCoverageReport(cov, "TestApp")
	if !strings.Contains(out, "N/A") {
		t.Error("missing N/A")
	}
}

func TestRenderFindings_NoDataSaysSoPlainly(t *testing.T) {
	out := RenderFindings(nil, "TestApp")
	if !strings.Contains(out, "No vulnerability-matching data was supplied") {
		t.Error("missing no-data notice")
	}
	if !strings.Contains(out, "not the same thing as") {
		t.Error("missing disambiguation")
	}
}

func TestRenderFindings_NoMatchesFound(t *testing.T) {
	vm := &resolve.Result{Components: []resolve.Component{
		comp("a.jar", "a", "1.0", "", "", "exact-purl"),
	}}
	out := RenderFindings(vm, "TestApp")
	if !strings.Contains(out, "No vulnerability matches found.") {
		t.Error("missing no-matches notice")
	}
}

func TestRenderFindings_SeparatesConfirmedFromHeuristic(t *testing.T) {
	vm := &resolve.Result{Components: []resolve.Component{
		comp("a.jar", "a", "1.0", "", "", "exact-purl", vuln("GHSA-aaaa", "Bad thing", "HIGH", "osv")),
		comp("b.exe", "b", "2.0", "", "", "heuristic", vuln("CVE-2023-9999", "Maybe bad", "LOW", "nvd")),
	}}
	out := RenderFindings(vm, "TestApp")
	parts := strings.SplitN(out, "## Low-confidence", 2)
	confirmedSection, heuristicSection := parts[0], parts[1]

	if !strings.Contains(confirmedSection, "GHSA-aaaa") {
		t.Error("confirmed section missing GHSA-aaaa")
	}
	if strings.Contains(confirmedSection, "CVE-2023-9999") {
		t.Error("confirmed section should not contain CVE-2023-9999")
	}
	if !strings.Contains(heuristicSection, "CVE-2023-9999") {
		t.Error("heuristic section missing CVE-2023-9999")
	}
	if !strings.Contains(heuristicSection, "Verify manually") {
		t.Error("heuristic section missing verify-manually notice")
	}
}

func TestRenderFindings_DedupesSameCVEAcrossFilesSharingAnIdentity(t *testing.T) {
	v := vuln("CVE-2017-10989", "Bad thing", "CRITICAL", "nvd")
	vm := &resolve.Result{Components: []resolve.Component{
		comp(`a\sqlite3.dll`, "SQLite", "3.15.2", "", "cpe:2.3:a:sqlite:sqlite:3.15.2:*:*:*:*:*:*:*", "mapped-cpe", v),
		comp(`b\sqlite3.dll`, "SQLite", "3.15.2", "", "cpe:2.3:a:sqlite:sqlite:3.15.2:*:*:*:*:*:*:*", "mapped-cpe", v),
	}}
	out := RenderFindings(vm, "TestApp")
	if got := strings.Count(out, "[CVE-2017-10989]"); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
}

func TestRenderFindings_SameCVEDifferentVersionsBothShown(t *testing.T) {
	vm := &resolve.Result{Components: []resolve.Component{
		comp(`a\sqlite3.dll`, "SQLite", "3.15.2", "", "cpe:2.3:a:sqlite:sqlite:3.15.2:*:*:*:*:*:*:*", "mapped-cpe",
			vuln("CVE-2020-13434", "x", "MEDIUM", "nvd")),
		comp(`b\sqlite3.dll`, "SQLite", "3.7.15", "", "cpe:2.3:a:sqlite:sqlite:3.7.15:*:*:*:*:*:*:*", "mapped-cpe",
			vuln("CVE-2020-13434", "x", "MEDIUM", "nvd")),
	}}
	out := RenderFindings(vm, "TestApp")
	if got := strings.Count(out, "[CVE-2020-13434]"); got != 2 {
		t.Errorf("count = %d, want 2", got)
	}
	if !strings.Contains(out, "3.15.2") || !strings.Contains(out, "3.7.15") {
		t.Error("missing one of the versions")
	}
}

func TestRenderFindingsCSV_HasHeaderAndOneRowPerFinding(t *testing.T) {
	vm := &resolve.Result{Components: []resolve.Component{
		comp("a.jar", "a", "1.0", "", "", "exact-purl", vuln("CVE-2023-0001", "x", "HIGH", "nvd")),
	}}
	csvText, err := RenderFindingsCSV(vm)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(csvText, "\n"), "\n")
	if lines[0] != "Severity,ID,URL,Component,Version,Summary,Source,Confidence,Affected Files" {
		t.Errorf("header = %q", lines[0])
	}
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2", len(lines))
	}
	for _, want := range []string{"CVE-2023-0001", "https://nvd.nist.gov/vuln/detail/CVE-2023-0001", "exact-purl", "a.jar"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("row missing %q: %q", want, lines[1])
		}
	}
}

func TestRenderFindingsCSV_JoinsMultipleAffectedFilesWithSemicolon(t *testing.T) {
	v := vuln("CVE-2026-0001", "x", "CRITICAL", "nvd")
	vm := &resolve.Result{Components: []resolve.Component{
		comp(`a\outer-app.jar`, "OuterApp", "9.9.9", "pkg:maven/a/outer-app@9.9.9", "", "exact-purl", v),
		comp(`b\plugins\outer-app-legacy.jar`, "OuterApp", "9.9.9", "pkg:maven/a/outer-app@9.9.9", "", "exact-purl", v),
	}}
	csvText, err := RenderFindingsCSV(vm)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(csvText, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2", len(lines))
	}
	if !strings.Contains(lines[1], `a\outer-app.jar; b\plugins\outer-app-legacy.jar`) {
		t.Errorf("row = %q", lines[1])
	}
}

func TestRenderFindingsCSV_EmptyWhenNoFindings(t *testing.T) {
	vm := &resolve.Result{Components: []resolve.Component{
		comp("a.jar", "a", "1.0", "", "", "exact-purl"),
	}}
	csvText, err := RenderFindingsCSV(vm)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(csvText, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("len(lines) = %d, want 1 (header only)", len(lines))
	}
}

func TestCountBySeverity_CountsEachBucket(t *testing.T) {
	rows := []FindingRow{
		{SeverityLevel: "CRITICAL"}, {SeverityLevel: "CRITICAL"},
		{SeverityLevel: "HIGH"},
		{SeverityLevel: "MEDIUM"}, {SeverityLevel: "Moderate"},
		{SeverityLevel: "LOW"}, {SeverityLevel: "LOW"}, {SeverityLevel: "LOW"},
	}
	got := CountBySeverity(rows)
	want := map[string]int{"CRITICAL": 2, "HIGH": 1, "MEDIUM": 2, "LOW": 3}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("counts[%s] = %d, want %d", k, got[k], v)
		}
	}
}

func TestCountBySeverity_IgnoresUnknownAndMissing(t *testing.T) {
	rows := []FindingRow{{SeverityLevel: ""}, {SeverityLevel: "NONE"}, {SeverityLevel: "WEIRD"}}
	got := CountBySeverity(rows)
	for _, k := range displaySeverities {
		if got[k] != 0 {
			t.Errorf("counts[%s] = %d, want 0", k, got[k])
		}
	}
}

func TestCountBySeverity_EmptyRows(t *testing.T) {
	got := CountBySeverity(nil)
	for _, k := range displaySeverities {
		if got[k] != 0 {
			t.Errorf("counts[%s] = %d, want 0", k, got[k])
		}
	}
}

func TestDiffFindingRows_DetectsNewAndResolved(t *testing.T) {
	oldRows := []FindingRow{
		{Product: "a", Version: "1.0", ID: "CVE-2020-0001"},
		{Product: "b", Version: "2.0", ID: "CVE-2020-0002"},
	}
	newRows := []FindingRow{
		{Product: "b", Version: "2.0", ID: "CVE-2020-0002"},
		{Product: "c", Version: "3.0", ID: "CVE-2020-0003"},
	}
	diff := DiffFindingRows(oldRows, newRows)
	if len(diff.NewFindings) != 1 || diff.NewFindings[0].ID != "CVE-2020-0003" {
		t.Errorf("NewFindings = %+v", diff.NewFindings)
	}
	if len(diff.ResolvedFindings) != 1 || diff.ResolvedFindings[0].ID != "CVE-2020-0001" {
		t.Errorf("ResolvedFindings = %+v", diff.ResolvedFindings)
	}
	if diff.UnchangedCount != 1 {
		t.Errorf("UnchangedCount = %d, want 1", diff.UnchangedCount)
	}
}

func TestDiffFindingRows_SameIDDifferentVersionCountsAsBothChanges(t *testing.T) {
	oldRows := []FindingRow{{Product: "a", Version: "1.0", ID: "CVE-2020-0001"}}
	newRows := []FindingRow{{Product: "a", Version: "2.0", ID: "CVE-2020-0001"}}
	diff := DiffFindingRows(oldRows, newRows)
	if len(diff.NewFindings) != 1 {
		t.Errorf("len(NewFindings) = %d, want 1", len(diff.NewFindings))
	}
	if len(diff.ResolvedFindings) != 1 {
		t.Errorf("len(ResolvedFindings) = %d, want 1", len(diff.ResolvedFindings))
	}
	if diff.UnchangedCount != 0 {
		t.Errorf("UnchangedCount = %d, want 0", diff.UnchangedCount)
	}
}

func TestDiffFindingRows_EmptyInputs(t *testing.T) {
	diff := DiffFindingRows(nil, nil)
	if len(diff.NewFindings) != 0 || len(diff.ResolvedFindings) != 0 || diff.UnchangedCount != 0 {
		t.Errorf("diff = %+v, want all empty/zero", diff)
	}
}

func TestRenderFindings_SortsBySeverityCriticalFirst(t *testing.T) {
	vm := &resolve.Result{Components: []resolve.Component{
		comp("a.jar", "a", "1.0", "", "", "exact-purl", vuln("LOW-1", "", "LOW", "osv")),
		comp("b.jar", "b", "1.0", "", "", "exact-purl", vuln("CRIT-1", "", "CRITICAL", "osv")),
	}}
	out := RenderFindings(vm, "TestApp")
	if strings.Index(out, "CRIT-1") > strings.Index(out, "LOW-1") {
		t.Error("CRIT-1 should appear before LOW-1")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
