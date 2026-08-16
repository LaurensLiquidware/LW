package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

const testEndDate = "2026-08-16"

func newGeneratedDB(t *testing.T, tenantCount, months int, seed uint64) (*tenant.Repo, *snapshot.Repo, []tenant.Tenant) {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "demo.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	tenantRepo := tenant.NewRepo(sqlDB, make([]byte, 32))
	snapshotRepo := snapshot.NewRepo(sqlDB)

	endDate, err := time.Parse("2006-01-02", testEndDate)
	if err != nil {
		t.Fatal(err)
	}
	if err := generate(context.Background(), tenantRepo, snapshotRepo, tenantCount, months, endDate, seed); err != nil {
		t.Fatalf("generate: %v", err)
	}

	tenants, err := tenantRepo.List(context.Background())
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	return tenantRepo, snapshotRepo, tenants
}

// deterministicChecksum hashes every generated snapshot's *deterministic*
// fields (collection date, status, license figures, product/mode) across
// every tenant, identified by roster order (tenant DisplayName) rather
// than by database ID. Row IDs (crypto/rand-based UUIDs) and the
// encrypted credential blob (a fresh random AES-GCM nonce per call) are
// deliberately excluded -- both are non-deterministic by design, a
// security property, not something a demo generator should ever make
// reproducible. See main.go's package doc comment.
func deterministicChecksum(t *testing.T, snapshotRepo *snapshot.Repo, tenants []tenant.Tenant, months int) string {
	t.Helper()
	h := sha256.New()
	for _, tn := range tenants {
		endDate, _ := time.Parse("2006-01-02", testEndDate)
		totalDays := months * 30
		from := endDate.AddDate(0, 0, -(totalDays - 1)).Format("2006-01-02")
		points, err := snapshotRepo.ListByTenantInRange(context.Background(), tn.ID, from, testEndDate)
		if err != nil {
			t.Fatalf("list snapshots for %s: %v", tn.DisplayName, err)
		}
		for _, s := range points {
			fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s|%s|%s|%s\n",
				tn.DisplayName, s.CollectionDate, s.Status, s.LicenseMode, s.LicenseProduct,
				s.ErrorMessage, intPtrString(s.TotalLicenses), intPtrString(s.UsedLicenses), s.SupportEndsISO)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func intPtrString(v *int) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *v)
}

func TestGenerate_SameSeedProducesIdenticalDeterministicContent(t *testing.T) {
	_, snapshotRepoA, tenantsA := newGeneratedDB(t, len(roster), 6, defaultSeed)
	_, snapshotRepoB, tenantsB := newGeneratedDB(t, len(roster), 6, defaultSeed)

	sumA := deterministicChecksum(t, snapshotRepoA, tenantsA, 6)
	sumB := deterministicChecksum(t, snapshotRepoB, tenantsB, 6)

	if sumA != sumB {
		t.Errorf("checksums differ across two runs with the same seed:\n  run 1: %s\n  run 2: %s", sumA, sumB)
	}
}

func TestGenerate_DifferentSeedProducesDifferentContent(t *testing.T) {
	_, snapshotRepoA, tenantsA := newGeneratedDB(t, len(roster), 6, defaultSeed)
	_, snapshotRepoB, tenantsB := newGeneratedDB(t, len(roster), 6, defaultSeed+1)

	sumA := deterministicChecksum(t, snapshotRepoA, tenantsA, 6)
	sumB := deterministicChecksum(t, snapshotRepoB, tenantsB, 6)

	if sumA == sumB {
		t.Error("checksums are identical across two different seeds -- --seed isn't actually affecting output")
	}
}

// TestGenerate_RowCounts is the "golden" row-count assertion: every
// roster entry gets exactly 180 days (6 months x 30) except Zonneveld
// Techniek, whose late-onboarding story means its first ~4.5 months are
// absent rows, not zeros.
func TestGenerate_RowCounts(t *testing.T) {
	_, snapshotRepo, tenants := newGeneratedDB(t, len(roster), 6, defaultSeed)

	if len(tenants) != len(roster) {
		t.Fatalf("tenant count = %d, want %d", len(tenants), len(roster))
	}

	byName := make(map[string]tenant.Tenant, len(tenants))
	for _, tn := range tenants {
		byName[tn.DisplayName] = tn
	}

	endDate, _ := time.Parse("2006-01-02", testEndDate)
	from := endDate.AddDate(0, 0, -179).Format("2006-01-02")

	for _, spec := range roster {
		tn, ok := byName[spec.displayName]
		if !ok {
			t.Fatalf("tenant %q was not created", spec.displayName)
		}
		points, err := snapshotRepo.ListByTenantInRange(context.Background(), tn.ID, from, testEndDate)
		if err != nil {
			t.Fatalf("list snapshots for %s: %v", spec.displayName, err)
		}

		want := 180
		if spec.story == storyLateOnboard {
			want = 42
		}
		if len(points) != want {
			t.Errorf("%s: %d snapshot rows, want %d", spec.displayName, len(points), want)
		}
	}
}
