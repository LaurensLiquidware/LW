// Package report renders coverage-report.md and findings.md (plus CSV
// and diff) from computed scan data.
//
// coverage-report.md needs only a Coverage value (no network, no
// vuln-matches needed). findings.md needs a resolve.Result; if none is
// supplied, it says so plainly rather than rendering an empty-looking
// report that could be mistaken for "no vulnerabilities found."
package report

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"

	"flexapp-vuln-scanner/internal/coverage"
	"flexapp-vuln-scanner/internal/normalize"
	"flexapp-vuln-scanner/internal/resolve"
)

var severityRank = map[string]int{
	"CRITICAL": 0,
	"HIGH":     1,
	"MODERATE": 2, // OSV/GHSA sometimes uses "Moderate" where NVD uses "Medium"
	"MEDIUM":   2,
	"LOW":      3,
	"NONE":     4,
}

func severityRankOf(level string) int {
	if r, ok := severityRank[strings.ToUpper(level)]; ok {
		return r
	}
	return 5
}

// VulnerabilityURL returns the canonical public reference page for a
// vulnerability id, or "" if the id doesn't match a recognized scheme.
func VulnerabilityURL(vulnID string) string {
	switch {
	case vulnID == "":
		return ""
	case strings.HasPrefix(vulnID, "CVE-"):
		return "https://nvd.nist.gov/vuln/detail/" + vulnID
	case strings.HasPrefix(vulnID, "GHSA-"):
		return "https://github.com/advisories/" + vulnID
	default:
		return "https://osv.dev/vulnerability/" + vulnID
	}
}

// RenderCoverageReport renders coverage-report.md.
func RenderCoverageReport(cov coverage.Coverage, packageName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Coverage Report — %s\n\n", packageName)

	pctStr := "N/A (no candidate components found)"
	if cov.CoveragePercent != nil {
		pctStr = fmt.Sprintf("%.1f%%", *cov.CoveragePercent)
	}
	fmt.Fprintf(&b, "**Resolution coverage: %s**\n\n", pctStr)
	fmt.Fprintf(&b, "- Total files scanned: %d\n", cov.TotalFilesScanned)
	fmt.Fprintf(&b, "- Files excluded (noise filtering): %d\n", cov.ExcludedCount)
	fmt.Fprintf(&b, "- Candidate components (excluded: false): %d\n", cov.CandidateComponents)
	fmt.Fprintf(&b, "- Components resolved: %d\n", cov.ResolvedComponents)
	fmt.Fprintf(&b, "- Components unresolved: %d\n\n", cov.UnresolvedComponents)

	b.WriteString("## Excluded files, by reason\n\n")
	if len(cov.ExcludedByReason) > 0 {
		b.WriteString("| Reason | Count |\n|---|---|\n")
		for _, reason := range sortedStringKeys(cov.ExcludedByReason) {
			fmt.Fprintf(&b, "| %s | %d |\n", reason, cov.ExcludedByReason[reason])
		}
	} else {
		b.WriteString("None excluded.\n")
	}
	b.WriteString("\n")

	b.WriteString("## Resolved components, by method\n\n")
	if len(cov.ResolvedByMethod) > 0 {
		b.WriteString("| Method | Count |\n|---|---|\n")
		for _, method := range sortedStringKeys(cov.ResolvedByMethod) {
			fmt.Fprintf(&b, "| %s | %d |\n", method, cov.ResolvedByMethod[method])
		}
	} else {
		b.WriteString("None resolved.\n")
	}
	b.WriteString("\n")

	b.WriteString("## Unresolved components\n\n")
	if len(cov.UnresolvedFiles) > 0 {
		b.WriteString("| Path | Component type | Read error |\n|---|---|---|\n")
		for _, f := range cov.UnresolvedFiles {
			readError := ""
			if f.ReadError != nil {
				readError = *f.ReadError
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", f.RelativePath, f.ComponentType, readError)
		}
	} else {
		b.WriteString("None — every candidate component resolved.\n")
	}
	b.WriteString("\n")

	return b.String()
}

// FindingRow is one flattened (component, vulnerability) row.
type FindingRow struct {
	SeverityLevel string
	ID            string
	URL           string
	Summary       string
	Product       string
	Version       string
	RelativePaths []string
	Confidence    string
	Source        string
}

// BuildFindingRows flattens a vuln-matches result into one row per
// distinct (component, vulnerability), severity-sorted. Keyed by
// (purl-or-cpe, vulnerability id): the same physical component (e.g. the
// same bundled sqlite3.dll copied to more than one path) can appear as
// more than one candidate, each carrying an identical vulnerability
// list -- every relativePath sharing that dedup key is collected into
// the row's RelativePaths (sorted, deduplicated) rather than keeping
// only the first one seen.
func BuildFindingRows(vulnMatches *resolve.Result) []FindingRow {
	if vulnMatches == nil {
		return nil
	}

	type key struct {
		identity string
		vulnID   string
	}
	rowsByKey := map[key]*FindingRow{}
	var order []key

	for _, component := range vulnMatches.Components {
		var product, version string
		if component.Identity != nil {
			if component.Identity.Product != nil {
				product = *component.Identity.Product
			}
			if component.Identity.Version != nil {
				version = *component.Identity.Version
			}
		}
		confidence := component.Confidence
		relativePath := component.RelativePath

		dedupIdentity := component.Purl
		if dedupIdentity == "" {
			dedupIdentity = component.CPE
		}
		if dedupIdentity == "" {
			dedupIdentity = relativePath
		}

		for _, vuln := range component.Vulnerabilities {
			k := key{identity: dedupIdentity, vulnID: vuln.ID}
			if row, ok := rowsByKey[k]; ok {
				if relativePath != "" && !containsString(row.RelativePaths, relativePath) {
					row.RelativePaths = append(row.RelativePaths, relativePath)
				}
				continue
			}

			rowProduct := product
			if rowProduct == "" {
				rowProduct = relativePath
			}
			severityLevel := ""
			if vuln.SeverityLevel != nil {
				severityLevel = *vuln.SeverityLevel
			}
			summary := ""
			if vuln.Summary != nil {
				summary = *vuln.Summary
			}
			var relativePaths []string
			if relativePath != "" {
				relativePaths = []string{relativePath}
			}

			row := &FindingRow{
				SeverityLevel: severityLevel,
				ID:            vuln.ID,
				URL:           VulnerabilityURL(vuln.ID),
				Summary:       summary,
				Product:       rowProduct,
				Version:       version,
				RelativePaths: relativePaths,
				Confidence:    confidence,
				Source:        vuln.Source,
			}
			rowsByKey[k] = row
			order = append(order, k)
		}
	}

	rows := make([]FindingRow, len(order))
	for i, k := range order {
		row := rowsByKey[k]
		sort.Strings(row.RelativePaths)
		rows[i] = *row
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ri, rj := severityRankOf(rows[i].SeverityLevel), severityRankOf(rows[j].SeverityLevel)
		if ri != rj {
			return ri < rj
		}
		return rows[i].ID < rows[j].ID
	})
	return rows
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

var displaySeverities = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"}

// CountBySeverity counts BuildFindingRows' output into the 4 severity
// buckets shown in a UI's summary counts. "Moderate" (OSV/GHSA's
// spelling) folds into "Medium" (NVD's spelling).
func CountBySeverity(rows []FindingRow) map[string]int {
	counts := map[string]int{}
	for _, level := range displaySeverities {
		counts[level] = 0
	}
	for _, r := range rows {
		level := strings.ToUpper(r.SeverityLevel)
		if level == "MODERATE" {
			level = "MEDIUM"
		}
		if _, ok := counts[level]; ok {
			counts[level]++
		}
	}
	return counts
}

var csvHeader = []string{"Severity", "ID", "URL", "Component", "Version", "Summary", "Source", "Confidence", "Affected Files"}

// RenderFindingsCSV renders a CSV of every finding row -- both confirmed
// and heuristic, in one table with a Confidence column. Only meaningful
// when there IS vuln-matches data -- an absent file (rather than an
// empty CSV) disambiguates "not run" from "zero vulnerabilities found."
func RenderFindingsCSV(vulnMatches *resolve.Result) (string, error) {
	rows := BuildFindingRows(vulnMatches)
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(csvHeader); err != nil {
		return "", err
	}
	for _, r := range rows {
		record := []string{
			r.SeverityLevel, r.ID, r.URL, r.Product, r.Version, r.Summary, r.Source, r.Confidence,
			// "; " (not ",") -- a Windows path never contains a
			// semicolon but commonly needs a comma-safe separator.
			strings.Join(r.RelativePaths, "; "),
		}
		if err := w.Write(record); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Diff is the result of comparing two scans' finding rows.
type Diff struct {
	NewFindings      []FindingRow
	ResolvedFindings []FindingRow
	UnchangedCount   int
}

// DiffFindingRows compares two scans' already-flattened finding rows
// and reports what changed. Matched by (product, version, vulnerability
// id) -- not the internal purl/cpe dedup key BuildFindingRows uses
// internally, since two scans of the "same" package can resolve a
// component via a different purl/cpe confidence path while still being
// the same real-world component+CVE pairing a human would want treated
// as unchanged.
func DiffFindingRows(oldRows, newRows []FindingRow) Diff {
	type key struct {
		product, version, id string
	}
	keyOf := func(r FindingRow) key { return key{r.Product, r.Version, r.ID} }

	oldKeys := map[key]struct{}{}
	for _, r := range oldRows {
		oldKeys[keyOf(r)] = struct{}{}
	}
	newKeys := map[key]struct{}{}
	for _, r := range newRows {
		newKeys[keyOf(r)] = struct{}{}
	}

	var newFindings, resolvedFindings []FindingRow
	unchanged := 0
	for _, r := range newRows {
		if _, ok := oldKeys[keyOf(r)]; ok {
			unchanged++
		} else {
			newFindings = append(newFindings, r)
		}
	}
	for _, r := range oldRows {
		if _, ok := newKeys[keyOf(r)]; !ok {
			resolvedFindings = append(resolvedFindings, r)
		}
	}

	return Diff{NewFindings: newFindings, ResolvedFindings: resolvedFindings, UnchangedCount: unchanged}
}

func isConfirmed(confidence string) bool {
	return confidence == normalize.ConfidenceExactPurl || confidence == normalize.ConfidenceMappedCPE
}

// SplitByConfidence splits rows into confirmed (exact-purl / mapped-cpe)
// and heuristic buckets.
func SplitByConfidence(rows []FindingRow) (confirmed, heuristic []FindingRow) {
	for _, r := range rows {
		if isConfirmed(r.Confidence) {
			confirmed = append(confirmed, r)
		} else if r.Confidence == normalize.ConfidenceHeuristic {
			heuristic = append(heuristic, r)
		}
	}
	return confirmed, heuristic
}

// RenderFindings renders findings.md. vulnMatches nil means the resolve
// step wasn't run -- rendered as an explicit "no data" notice, never
// conflated with "zero vulnerabilities found."
func RenderFindings(vulnMatches *resolve.Result, packageName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Findings — %s\n\n", packageName)

	if vulnMatches == nil {
		b.WriteString("No vulnerability-matching data was supplied for this report " +
			"(the resolve step wasn't run, or its output wasn't passed in). " +
			"This report has no findings to show - that is not the same " +
			"thing as \"no vulnerabilities found.\"\n\n")
		return b.String()
	}

	rows := BuildFindingRows(vulnMatches)
	if len(rows) == 0 {
		b.WriteString("No vulnerability matches found.\n\n")
		return b.String()
	}
	confirmed, heuristic := SplitByConfidence(rows)

	renderTable := func(entries []FindingRow) string {
		var t strings.Builder
		t.WriteString("| Severity | ID | Component | Version | Affected Files | Summary | Source | Confidence |\n")
		t.WriteString("|---|---|---|---|---|---|---|---|\n")
		for _, r := range entries {
			severity := r.SeverityLevel
			if severity == "" {
				severity = "UNKNOWN"
			}
			summary := r.Summary
			if len(summary) > 100 {
				summary = summary[:100] + "…"
			}
			idCell := r.ID
			if r.URL != "" {
				idCell = fmt.Sprintf("[%s](%s)", r.ID, r.URL)
			}
			filesCell := "—"
			if len(r.RelativePaths) > 0 {
				quoted := make([]string, len(r.RelativePaths))
				for i, p := range r.RelativePaths {
					quoted[i] = "`" + p + "`"
				}
				filesCell = strings.Join(quoted, "<br>")
			}
			fmt.Fprintf(&t, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
				severity, idCell, r.Product, r.Version, filesCell, summary, r.Source, r.Confidence)
		}
		return t.String()
	}

	b.WriteString("## Confirmed matches (exact-purl / mapped-cpe)\n\n")
	if len(confirmed) > 0 {
		b.WriteString(renderTable(confirmed))
	} else {
		b.WriteString("None.\n")
	}
	b.WriteString("\n")

	b.WriteString("## Low-confidence matches (heuristic)\n\n")
	b.WriteString("These matches used automatic vendor/product normalization, not a " +
		"verified purl or a curated CPE mapping. **Verify manually before " +
		"treating any of these as a confirmed finding.**\n\n")
	if len(heuristic) > 0 {
		b.WriteString(renderTable(heuristic))
	} else {
		b.WriteString("None.\n")
	}
	b.WriteString("\n")

	return b.String()
}

func sortedStringKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
