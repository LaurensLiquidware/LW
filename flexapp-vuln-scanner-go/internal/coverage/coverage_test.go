package coverage

import (
	"math"
	"testing"

	"flexapp-vuln-scanner/internal/inventory"
)

const fixturePath = "../inventory/testdata/sample.inventory.json"

// TestCompute_MatchesFixtureExactly mirrors
// stage2-resolve/tests/test_coverage.py's
// test_compute_coverage_matches_fixture_exactly, against the same
// fixture, for output parity with the Python implementation.
func TestCompute_MatchesFixtureExactly(t *testing.T) {
	inv, err := inventory.Load(fixturePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cov := Compute(inv)

	if cov.TotalFilesScanned != 4 {
		t.Errorf("TotalFilesScanned = %d, want 4", cov.TotalFilesScanned)
	}
	if cov.ExcludedCount != 1 {
		t.Errorf("ExcludedCount = %d, want 1", cov.ExcludedCount)
	}
	if want := map[string]int{"os-system-path": 1}; !mapsEqual(cov.ExcludedByReason, want) {
		t.Errorf("ExcludedByReason = %v, want %v", cov.ExcludedByReason, want)
	}
	if cov.CandidateComponents != 3 {
		t.Errorf("CandidateComponents = %d, want 3", cov.CandidateComponents)
	}
	if cov.ResolvedComponents != 2 {
		t.Errorf("ResolvedComponents = %d, want 2", cov.ResolvedComponents)
	}
	if cov.UnresolvedComponents != 1 {
		t.Errorf("UnresolvedComponents = %d, want 1", cov.UnresolvedComponents)
	}
	if cov.CoveragePercent == nil || math.Abs(*cov.CoveragePercent-2.0/3.0*100) > 1e-9 {
		t.Errorf("CoveragePercent = %v, want ~66.67", cov.CoveragePercent)
	}
	if want := map[string]int{"jar-pom-properties": 1, "string-signature": 1}; !mapsEqual(cov.ResolvedByMethod, want) {
		t.Errorf("ResolvedByMethod = %v, want %v", cov.ResolvedByMethod, want)
	}
	if len(cov.UnresolvedFiles) != 1 || cov.UnresolvedFiles[0].RelativePath != `Program Files\App\unresolved.bin` {
		t.Errorf("UnresolvedFiles = %+v", cov.UnresolvedFiles)
	}
}

func TestCompute_ZeroCandidatesReportsNoneNotError(t *testing.T) {
	inv := &inventory.Inventory{
		Files: []inventory.File{
			{RelativePath: "x", Excluded: true, ExclusionReason: strPtr("font-file"), ComponentType: "unknown"},
		},
	}
	cov := Compute(inv)
	if cov.CandidateComponents != 0 {
		t.Errorf("CandidateComponents = %d, want 0", cov.CandidateComponents)
	}
	if cov.CoveragePercent != nil {
		t.Errorf("CoveragePercent = %v, want nil", cov.CoveragePercent)
	}
}

func TestCompute_FullResolutionIs100Percent(t *testing.T) {
	product := "a"
	version := "1.0"
	inv := &inventory.Inventory{
		Files: []inventory.File{
			{
				RelativePath:  "a.jar",
				Excluded:      false,
				ComponentType: "jar",
				Identity:      &inventory.Identity{Method: "jar-pom-properties", Product: &product, Version: &version},
			},
		},
	}
	cov := Compute(inv)
	if cov.CoveragePercent == nil || *cov.CoveragePercent != 100.0 {
		t.Errorf("CoveragePercent = %v, want 100.0", cov.CoveragePercent)
	}
}

func strPtr(s string) *string { return &s }

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
