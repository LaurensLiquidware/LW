package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_RequiresOutput(t *testing.T) {
	err := run([]string{"-package", "x.vhdx"})
	if err == nil || !strings.Contains(err.Error(), "-output is required") {
		t.Errorf("err = %v", err)
	}
}

func TestRun_RequiresPackageOrRefresh(t *testing.T) {
	err := run([]string{"-output", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "either -package") {
		t.Errorf("err = %v", err)
	}
}

func TestRun_PackageAndRefreshAreMutuallyExclusive(t *testing.T) {
	err := run([]string{"-package", "x.vhdx", "-refresh", "x.inventory.json", "-output", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("err = %v", err)
	}
}

// TestRun_RefreshAgainstEmptyInventorySucceedsWithNoNetwork proves the
// full CLI path (config load, CPE mappings, Stage 2, report writing)
// works end-to-end without needing network access: an inventory with
// no files has nothing to resolve, so resolve.Resolve never calls
// OSV/NVD at all.
func TestRun_RefreshAgainstEmptyInventorySucceedsWithNoNetwork(t *testing.T) {
	dir := t.TempDir()
	inventoryPath := filepath.Join(dir, "empty.inventory.json")
	if err := os.WriteFile(inventoryPath, []byte(`{
		"schemaVersion": "1.0",
		"package": {
			"sourcePath": "test.vhdx", "packageType": "classic-vhdx", "flexAppXml": null,
			"scanStartedUtc": "2026-08-21T00:00:00Z", "scanFinishedUtc": "2026-08-21T00:01:00Z",
			"toolVersion": "0.1.0"
		},
		"files": []
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(dir, "out")
	if err := run([]string{
		"-refresh", inventoryPath,
		"-output", outputDir,
		"-cache-dir", filepath.Join(dir, "cache"),
		"-cpe-mappings", filepath.Join(dir, "nonexistent-mappings.yaml"),
	}); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, name := range []string{
		"empty.sbom.cdx.json", "empty.coverage-report.md", "empty.findings.md",
		"empty.report.pdf", "empty.findings.csv", "empty.vuln-matches.json",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); err != nil {
			t.Errorf("%s not written: %v", name, err)
		}
	}
}
