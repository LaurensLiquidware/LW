package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flexapp-vuln-scanner/internal/cpemap"
)

const fixturePath = "../inventory/testdata/sample.inventory.json"

// fakeSink is a minimal stand-in for ProgressSink.
type fakeSink struct {
	status string
	log    []string
}

func (s *fakeSink) SetStatus(status string)                   { s.status = status }
func (s *fakeSink) AppendLog(line string)                     { s.log = append(s.log, line) }
func (s *fakeSink) SetProgress(phase string, done, total int) {}

func copyFixture(t *testing.T, dstDir string) string {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dstDir, "sample.inventory.json")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

// TestRunStage1_MissingScriptRaises mirrors
// ../../../flexapp-vuln-scanner/stage2-resolve/tests/test_pipeline.py's
// test_run_stage1_missing_script_raises.
func TestRunStage1_MissingScriptRaises(t *testing.T) {
	sink := &fakeSink{}
	_, err := RunStage1(sink, "/nonexistent/Invoke-FlexAppInventory.ps1", "whatever.vhdx", t.TempDir())
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "not found") {
		t.Errorf("error = %q, want it to mention 'not found'", got)
	}
	if sink.status != "stage1" {
		t.Errorf("sink.status = %q, want stage1", sink.status)
	}
}

func TestLoadExistingResult_NoVulnMatches(t *testing.T) {
	dir := t.TempDir()
	inventoryPath := copyFixture(t, dir)

	result, err := LoadExistingResult(inventoryPath, cpemap.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.HasVulnMatches {
		t.Error("HasVulnMatches = true, want false")
	}
	if len(result.ConfirmedRows) != 0 {
		t.Errorf("ConfirmedRows = %v, want empty", result.ConfirmedRows)
	}
	for kind, path := range map[string]string{
		"sbom": result.Files.SBOM, "coverage": result.Files.CoverageReport,
		"findings": result.Files.Findings, "pdf": result.Files.PDF,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s not written at %s: %v", kind, path, err)
		}
	}
	if result.Files.FindingsCSV != "" {
		t.Errorf("FindingsCSV = %q, want empty (no vuln-matches supplied)", result.Files.FindingsCSV)
	}
}

func TestLoadExistingResult_WithVulnMatches(t *testing.T) {
	dir := t.TempDir()
	inventoryPath := copyFixture(t, dir)

	vulnMatches := map[string]any{
		"generatedUtc": "2026-08-13T00:00:00Z",
		"package":      map[string]any{},
		"components": []map[string]any{{
			"relativePath": "a.jar",
			"identity":     map[string]any{"product": "a", "version": "1.0"},
			"purl":         "pkg:maven/a/a@1.0",
			"confidence":   "exact-purl",
			"vulnerabilities": []map[string]any{{
				"id": "GHSA-aaaa", "summary": "Bad", "severity": []any{}, "severityLevel": "HIGH", "source": "osv",
			}},
		}},
	}
	data, err := json.Marshal(vulnMatches)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample.vuln-matches.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := LoadExistingResult(inventoryPath, cpemap.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasVulnMatches {
		t.Error("HasVulnMatches = false, want true")
	}
	if len(result.ConfirmedRows) != 1 || result.ConfirmedRows[0].ID != "GHSA-aaaa" {
		t.Fatalf("ConfirmedRows = %+v", result.ConfirmedRows)
	}
	if len(result.ConfirmedRows[0].RelativePaths) != 1 || result.ConfirmedRows[0].RelativePaths[0] != "a.jar" {
		t.Errorf("RelativePaths = %v", result.ConfirmedRows[0].RelativePaths)
	}
}

func TestLoadDiff_RaisesDiffErrorForNonDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadDiff(filepath.Join(dir, "nope"), dir, cpemap.New(nil))
	if err == nil {
		t.Fatal("expected a DiffError")
	}
	if _, ok := err.(*DiffError); !ok {
		t.Fatalf("err = %T, want *DiffError", err)
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestLoadDiff_ReportsNewAndResolved(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()
	copyFixture(t, oldDir)
	copyFixture(t, newDir)

	write := func(dir, vulnID string) {
		vm := map[string]any{
			"generatedUtc": "2026-08-13T00:00:00Z",
			"package":      map[string]any{},
			"components": []map[string]any{{
				"relativePath": "a.jar",
				"identity":     map[string]any{"product": "a", "version": "1.0"},
				"confidence":   "exact-purl",
				"vulnerabilities": []map[string]any{{
					"id": vulnID, "summary": "x", "severity": []any{}, "severityLevel": "HIGH", "source": "osv",
				}},
			}},
		}
		data, err := json.Marshal(vm)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sample.vuln-matches.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(oldDir, "GHSA-old-only")
	write(newDir, "GHSA-new-only")

	diff, err := LoadDiff(oldDir, newDir, cpemap.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.NewFindings) != 1 || diff.NewFindings[0].ID != "GHSA-new-only" {
		t.Errorf("NewFindings = %+v", diff.NewFindings)
	}
	if len(diff.ResolvedFindings) != 1 || diff.ResolvedFindings[0].ID != "GHSA-old-only" {
		t.Errorf("ResolvedFindings = %+v", diff.ResolvedFindings)
	}
}
