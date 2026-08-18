package licensepush

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/tenant"
)

func newTestRepo(t *testing.T) (*Repo, string) {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	tenants := tenant.NewRepo(sqlDB, nil)
	tn, err := tenants.Create(context.Background(), tenant.CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return NewRepo(sqlDB), tn.ID
}

func TestRepo_CreateAndListForTenant(t *testing.T) {
	repo, tenantID := newTestRepo(t)
	ctx := context.Background()

	maxUsers := 450
	rec, err := repo.Create(ctx, Record{
		TenantID:          tenantID,
		OperatorUsername:  "LiquidwareMSP",
		Outcome:           OutcomeSuccess,
		LicenseCodeBase64: "dGVzdA==",
		Organization:      "Bakhuis Retail Group",
		ContactEmail:      "e.bakhuis@example.com",
		ValidUntil:        "12/31/2027",
		LicenseType:       "Perpetual",
		MaxUsers:          &maxUsers,
		IsConcurrent:      true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.ID == "" {
		t.Error("expected a generated ID")
	}
	if rec.PushedAtUTC.IsZero() {
		t.Error("expected PushedAtUTC to be set")
	}

	list, err := repo.ListForTenant(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListForTenant: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d records, want 1", len(list))
	}
	got := list[0]
	if got.Organization != "Bakhuis Retail Group" || got.Outcome != OutcomeSuccess || got.MaxUsers == nil || *got.MaxUsers != 450 || !got.IsConcurrent {
		t.Errorf("got %+v", got)
	}
}

func TestRepo_ListForTenant_NewestFirst(t *testing.T) {
	repo, tenantID := newTestRepo(t)
	ctx := context.Background()

	first, err := repo.Create(ctx, Record{TenantID: tenantID, OperatorUsername: "op", Outcome: OutcomeSuccess, LicenseCodeBase64: "a", PushedAtUTC: time.Now().UTC().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.Create(ctx, Record{TenantID: tenantID, OperatorUsername: "op", Outcome: OutcomeRejected, LicenseCodeBase64: "b"})
	if err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListForTenant(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != second.ID || list[1].ID != first.ID {
		t.Errorf("got %+v, want newest-first [%s, %s]", list, second.ID, first.ID)
	}
}

func TestRepo_ListForTenant_EmptyWhenNoPushes(t *testing.T) {
	repo, tenantID := newTestRepo(t)
	list, err := repo.ListForTenant(context.Background(), tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("got %d records, want 0", len(list))
	}
}
