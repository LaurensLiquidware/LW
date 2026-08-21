// Package pipeline is the shared scan orchestration: Stage 1
// (PowerShell subprocess, mounts the package and writes an inventory
// JSON) then Stage 2 (in-process resolve/report calls), plus loading an
// existing scan's results and diffing two scans.
//
// Ported from ../../../flexapp-vuln-scanner/stage2-resolve/flexapp_vuln/pipeline.go
// (Python). Front-end agnostic: the HTTP API layer supplies its own
// ProgressSink for progress reporting.
package pipeline

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"flexapp-vuln-scanner/internal/coverage"
	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/inventory"
	"flexapp-vuln-scanner/internal/report"
	"flexapp-vuln-scanner/internal/resolve"
	"flexapp-vuln-scanner/internal/sbom"
)

// ProgressSink receives progress updates during a scan. The HTTP API
// layer's SSE-backed job type implements this.
type ProgressSink interface {
	SetStatus(status string)
	AppendLog(line string)
	SetProgress(phase string, done, total int)
}

var wroteInventoryRe = regexp.MustCompile(`(?m)^Wrote (.+\.inventory\.json)`)

// RunStage1 shells out to the Stage 1 PowerShell script, streaming its
// output to sink, and returns the inventory JSON path it wrote.
func RunStage1(sink ProgressSink, stage1Script, packagePath, outputDir string) (string, error) {
	sink.SetStatus("stage1")

	if _, err := os.Stat(stage1Script); err != nil {
		return "", fmt.Errorf("Stage 1 script not found at %s", stage1Script)
	}

	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		return "", fmt.Errorf("pwsh (PowerShell 7) not found on PATH - Stage 1 needs it to mount the package and run the inventory scan")
	}

	cmd := exec.Command(pwsh, "-NoLogo", "-NoProfile", "-File", stage1Script, "-Path", packagePath, "-OutputDir", outputDir)
	sink.AppendLog("$ " + strings.Join(cmd.Args, " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var outputLines []string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		sink.AppendLog(line)
		outputLines = append(outputLines, line)
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("Stage 1 exited with code %d - see log above", exitErr.ExitCode())
		}
		return "", err
	}

	match := wroteInventoryRe.FindStringSubmatch(strings.Join(outputLines, "\n"))
	if match == nil {
		return "", fmt.Errorf("Stage 1 finished but no '<package>.inventory.json' path was found in its output - can't proceed to Stage 2")
	}
	return strings.TrimSpace(match[1]), nil
}

// RunStage2 loads the inventory, resolves vulnerability matches, and
// writes every report artifact, returning the result summary a results
// view renders.
func RunStage2(sink ProgressSink, inventoryPath, outputDir, cacheDir, nvdAPIKey string, mappings *cpemap.Mappings) (*Result, error) {
	sink.SetStatus("stage2")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}

	sink.AppendLog("Loading inventory: " + inventoryPath)
	inv, err := inventory.Load(inventoryPath)
	if err != nil {
		return nil, err
	}

	sink.AppendLog("Querying OSV.dev + NVD for vulnerability matches...")
	vulnMatches, err := resolve.Resolve(inv, cacheDir, mappings, nvdAPIKey, sink.SetProgress)
	if err != nil {
		return nil, err
	}

	outBase := stemWithoutInventorySuffix(inventoryPath)
	vulnMatchesPath := filepath.Join(outputDir, outBase+".vuln-matches.json")
	if err := writeJSON(vulnMatchesPath, vulnMatches); err != nil {
		return nil, err
	}
	sink.AppendLog("Wrote " + vulnMatchesPath)

	return WriteReports(sink, inv, inventoryPath, vulnMatches, outputDir, outBase, mappings)
}

// Result is the result summary a results view renders.
type Result struct {
	PackageName    string              `json:"packageName"`
	Coverage       coverage.Coverage   `json:"coverage"`
	ConfirmedRows  []report.FindingRow `json:"confirmedRows"`
	HeuristicRows  []report.FindingRow `json:"heuristicRows"`
	SeverityCounts map[string]int      `json:"severityCounts"`
	HasVulnMatches bool                `json:"hasVulnMatches"`
	InventoryPath  string              `json:"inventoryPath"`
	OutputDir      string              `json:"outputDir"`
	Files          ResultFiles         `json:"files"`
}

// ResultFiles is the set of report artifact paths WriteReports wrote.
type ResultFiles struct {
	SBOM           string `json:"sbom"`
	CoverageReport string `json:"coverageReport"`
	Findings       string `json:"findings"`
	PDF            string `json:"pdf"`
	FindingsCSV    string `json:"findingsCsv,omitempty"` // "" if vulnMatches was nil
}

// WriteReports writes sbom/coverage/findings/PDF and returns the result
// summary a results view renders. Shared between a fresh scan and
// "open an existing output directory" (sink may be nil there -- no log
// to append to).
func WriteReports(sink ProgressSink, inv *inventory.Inventory, inventoryPath string, vulnMatches *resolve.Result, outDir, outBase string, mappings *cpemap.Mappings) (*Result, error) {
	packageName := inv.DisplayName()
	cov := coverage.Compute(inv)
	sbomDoc := sbom.Build(inv, mappings)
	coverageMD := report.RenderCoverageReport(cov, packageName)
	findingsMD := report.RenderFindings(vulnMatches, packageName)

	sbomPath := filepath.Join(outDir, outBase+".sbom.cdx.json")
	coveragePath := filepath.Join(outDir, outBase+".coverage-report.md")
	findingsPath := filepath.Join(outDir, outBase+".findings.md")
	pdfPath := filepath.Join(outDir, outBase+".report.pdf")

	if err := writeJSON(sbomPath, sbomDoc); err != nil {
		return nil, err
	}
	if err := os.WriteFile(coveragePath, []byte(coverageMD), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(findingsPath, []byte(findingsMD), 0o644); err != nil {
		return nil, err
	}

	// Only written when there IS vuln-matches data -- unlike
	// findings.md, a CSV has no way to spell out "no data supplied" in
	// prose, so an absent file (rather than an empty-looking one) is
	// what disambiguates that from "zero vulnerabilities found."
	var findingsCSVPath string
	if vulnMatches != nil {
		csvText, err := report.RenderFindingsCSV(vulnMatches)
		if err != nil {
			return nil, err
		}
		findingsCSVPath = filepath.Join(outDir, outBase+".findings.csv")
		if err := os.WriteFile(findingsCSVPath, []byte(csvText), 0o644); err != nil {
			return nil, err
		}
	}

	packageMeta := report.PackageMeta{}
	if inv.Package.FlexAppXML != nil && inv.Package.FlexAppXML.VersionMajorMinorBuildRevision != nil {
		packageMeta.VersionMajorMinorBuildRevision = *inv.Package.FlexAppXML.VersionMajorMinorBuildRevision
	}
	packageMeta.ScanFinishedUTC = inv.Package.ScanFinishedUTC
	if err := report.RenderPDF(pdfPath, packageName, packageMeta, cov, vulnMatches); err != nil {
		return nil, err
	}

	if sink != nil {
		written := []string{sbomPath, coveragePath, findingsPath, pdfPath}
		if findingsCSVPath != "" {
			written = append(written, findingsCSVPath)
		}
		for _, path := range written {
			sink.AppendLog("Wrote " + path)
		}
	}

	var allRows []report.FindingRow
	if vulnMatches != nil {
		allRows = report.BuildFindingRows(vulnMatches)
	}
	confirmedRows, heuristicRows := report.SplitByConfidence(allRows)

	return &Result{
		PackageName:    packageName,
		Coverage:       cov,
		ConfirmedRows:  confirmedRows,
		HeuristicRows:  heuristicRows,
		SeverityCounts: report.CountBySeverity(allRows),
		HasVulnMatches: vulnMatches != nil,
		InventoryPath:  inventoryPath,
		OutputDir:      outDir,
		Files: ResultFiles{
			SBOM:           sbomPath,
			CoverageReport: coveragePath,
			Findings:       findingsPath,
			PDF:            pdfPath,
			FindingsCSV:    findingsCSVPath,
		},
	}, nil
}

// LoadExistingResult rebuilds a results view from an already-completed
// scan's inventory JSON, reusing a sibling <base>.vuln-matches.json if
// one exists -- without needing to re-run resolve (no network calls).
func LoadExistingResult(inventoryPath string, mappings *cpemap.Mappings) (*Result, error) {
	inv, err := inventory.Load(inventoryPath)
	if err != nil {
		return nil, err
	}
	outBase := stemWithoutInventorySuffix(inventoryPath)
	outDir := filepath.Dir(inventoryPath)

	vulnMatchesPath := filepath.Join(outDir, outBase+".vuln-matches.json")
	var vulnMatches *resolve.Result
	if data, err := os.ReadFile(vulnMatchesPath); err == nil {
		vulnMatches = &resolve.Result{}
		if err := json.Unmarshal(data, vulnMatches); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	return WriteReports(nil, inv, inventoryPath, vulnMatches, outDir, outBase, mappings)
}

// DiffError reports that a directory given to LoadDiff can't be
// compared -- not a directory, no inventory in it, or more than one
// (ambiguous which package to compare). Meant to be shown to a user as
// a plain error message, not a stack trace.
type DiffError struct {
	Message string
}

func (e *DiffError) Error() string { return e.Message }

func findSingleInventory(dirPath string) (string, error) {
	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		return "", &DiffError{Message: fmt.Sprintf("'%s' is not a directory.", dirPath)}
	}

	matches, err := filepath.Glob(filepath.Join(dirPath, "*.inventory.json"))
	if err != nil {
		return "", err
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return "", &DiffError{Message: fmt.Sprintf("No *.inventory.json file found directly under '%s'.", dirPath)}
	}
	if len(matches) > 1 {
		return "", &DiffError{Message: fmt.Sprintf(
			"'%s' contains more than one *.inventory.json - comparison needs "+
				"a single-scan folder. Use \"Open an Existing Scan Output Folder\" instead "+
				"for a directory holding more than one package's scan.", dirPath)}
	}
	return matches[0], nil
}

// Diff is the result of comparing two single-package scan output
// directories.
type Diff struct {
	Old              *Result             `json:"old"`
	New              *Result             `json:"new"`
	NewFindings      []report.FindingRow `json:"newFindings"`
	ResolvedFindings []report.FindingRow `json:"resolvedFindings"`
	UnchangedCount   int                 `json:"unchangedCount"`
}

// LoadDiff compares two single-package scan output directories: which
// findings are new in newDir that weren't in oldDir, which were
// resolved (present in oldDir, gone in newDir), and how many are
// unchanged. Returns a *DiffError if either directory isn't a
// comparable single-scan folder.
func LoadDiff(oldDir, newDir string, mappings *cpemap.Mappings) (*Diff, error) {
	oldInventoryPath, err := findSingleInventory(oldDir)
	if err != nil {
		return nil, err
	}
	newInventoryPath, err := findSingleInventory(newDir)
	if err != nil {
		return nil, err
	}

	oldResult, err := LoadExistingResult(oldInventoryPath, mappings)
	if err != nil {
		return nil, err
	}
	newResult, err := LoadExistingResult(newInventoryPath, mappings)
	if err != nil {
		return nil, err
	}

	oldRows := append(append([]report.FindingRow{}, oldResult.ConfirmedRows...), oldResult.HeuristicRows...)
	newRows := append(append([]report.FindingRow{}, newResult.ConfirmedRows...), newResult.HeuristicRows...)
	findingDiff := report.DiffFindingRows(oldRows, newRows)

	return &Diff{
		Old:              oldResult,
		New:              newResult,
		NewFindings:      findingDiff.NewFindings,
		ResolvedFindings: findingDiff.ResolvedFindings,
		UnchangedCount:   findingDiff.UnchangedCount,
	}, nil
}

func stemWithoutInventorySuffix(inventoryPath string) string {
	base := filepath.Base(inventoryPath)
	base = strings.TrimSuffix(base, filepath.Ext(base)) // drop .json
	return strings.TrimSuffix(base, ".inventory")
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
