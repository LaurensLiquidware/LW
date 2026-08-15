package dashboard

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

func TestBuildAll(t *testing.T) {
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	tenantRepo := tenant.NewRepo(sqlDB, nil)
	snapshotRepo := snapshot.NewRepo(sqlDB)
	ctx := context.Background()

	healthy, err := tenantRepo.Create(ctx, tenant.CreateInput{DisplayName: "Healthy Co", Hostname: "h1", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	neverCollected, err := tenantRepo.Create(ctx, tenant.CreateInput{DisplayName: "New Co", Hostname: "h2", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if _, err := snapshotRepo.Upsert(ctx, snapshot.Snapshot{
		TenantID: healthy.ID, CollectionDate: "2026-08-14", CollectedAtUTC: now,
		Status: snapshot.StatusSuccess, TotalLicenses: intPtr(10), UsedLicenses: intPtr(1), SupportEndsISO: "2027-01-01",
	}); err != nil {
		t.Fatal(err)
	}

	statuses, err := BuildAll(ctx, Repos{Tenants: tenantRepo, Snapshots: snapshotRepo}, now, time.UTC)
	if err != nil {
		t.Fatalf("BuildAll: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2", len(statuses))
	}

	byID := map[string]TenantStatus{}
	for _, s := range statuses {
		byID[s.Tenant.ID] = s
	}

	if got := byID[healthy.ID]; got.Data != DataOK || got.Usage != UsageGood || got.Expiry != ExpiryOK {
		t.Errorf("healthy tenant = %+v", got)
	}
	if got := byID[neverCollected.ID]; got.Data != DataNeverCollected || got.Usage != UsageUnknown {
		t.Errorf("never-collected tenant = %+v", got)
	}
}
