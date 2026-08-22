package inventory

import "testing"

func TestLoad_SampleFixture(t *testing.T) {
	inv, err := Load("testdata/sample.inventory.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(inv.Files) != 4 {
		t.Fatalf("len(Files) = %d, want 4", len(inv.Files))
	}
	if inv.Package.SourcePath == "" {
		t.Error("Package.SourcePath should not be empty")
	}
}

func TestNonExcludedFiles(t *testing.T) {
	inv, err := Load("testdata/sample.inventory.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	candidates := inv.NonExcludedFiles()
	if len(candidates) != 3 {
		t.Fatalf("len(candidates) = %d, want 3 (kernel32.dll excluded)", len(candidates))
	}
	for _, f := range candidates {
		if f.RelativePath == `Windows\System32\kernel32.dll` {
			t.Error("excluded file kernel32.dll should not be in candidates")
		}
	}
}

func TestLoad_SampleFixtureHasNoMalwareScan(t *testing.T) {
	inv, err := Load("testdata/sample.inventory.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if inv.Package.MalwareScan != nil {
		t.Errorf("MalwareScan = %+v, want nil (older/skipped-scan inventories omit this field)", inv.Package.MalwareScan)
	}
}

func TestLoad_MalwareScanThreatsFound(t *testing.T) {
	inv, err := Load("testdata/with-malware-scan.inventory.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	scan := inv.Package.MalwareScan
	if scan == nil {
		t.Fatal("MalwareScan is nil, want a populated result")
	}
	if scan.Status != "threats-found" {
		t.Errorf("Status = %q, want threats-found", scan.Status)
	}
	if !scan.Ran {
		t.Error("Ran = false, want true")
	}
	if len(scan.Threats) != 1 {
		t.Fatalf("len(Threats) = %d, want 1", len(scan.Threats))
	}
	if scan.Threats[0].ThreatName != "Trojan:Win32/Fake" {
		t.Errorf("Threats[0].ThreatName = %q, want Trojan:Win32/Fake", scan.Threats[0].ThreatName)
	}
	if scan.PathScanned == nil || *scan.PathScanned != `C:\mount` {
		t.Errorf("PathScanned = %v, want C:\\mount", scan.PathScanned)
	}
	if scan.DurationSeconds == nil || *scan.DurationSeconds != 12.3 {
		t.Errorf("DurationSeconds = %v, want 12.3", scan.DurationSeconds)
	}
	if scan.SignatureVersion == nil || *scan.SignatureVersion != "1.423.100.0" {
		t.Errorf("SignatureVersion = %v, want 1.423.100.0", scan.SignatureVersion)
	}
	if scan.Details == nil || *scan.Details == "" {
		t.Errorf("Details = %v, want non-empty scan output", scan.Details)
	}
}

func TestDisplayName_FallsBackToSourcePathStem(t *testing.T) {
	inv, err := Load("testdata/sample.inventory.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Fixture has flexAppXml: null, so DisplayName falls back to the
	// source path's stem.
	if got := inv.DisplayName(); got != "sample_20260101000000" {
		t.Errorf("DisplayName() = %q, want %q", got, "sample_20260101000000")
	}
}
