package snapshot

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/tenant"
)

func newTestDB(t *testing.T) (*Repo, string) {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	tenantRepo := tenant.NewRepo(sqlDB, nil)
	tn, err := tenantRepo.Create(context.Background(), tenant.CreateInput{DisplayName: "x", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	return NewRepo(sqlDB), tn.ID
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func TestRepo_Upsert_InsertsThenIsIdempotentSameDay(t *testing.T) {
	repo, tenantID := newTestDB(t)
	ctx := context.Background()

	s1 := Snapshot{
		TenantID:       tenantID,
		CollectionDate: "2026-08-14",
		CollectedAtUTC: time.Date(2026, 8, 14, 3, 0, 0, 0, time.UTC),
		Status:         StatusSuccess,
		AuthPath:       "unauthenticated",
		RawPayload:     `{"first":true}`,
		TotalLicenses:  intPtr(5),
		UsedLicenses:   intPtr(1),
	}
	first, err := repo.Upsert(ctx, s1)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if first.ID == "" {
		t.Fatal("expected a generated ID")
	}

	s2 := s1
	s2.CollectedAtUTC = time.Date(2026, 8, 14, 15, 0, 0, 0, time.UTC) // later re-run, same day
	s2.RawPayload = `{"second":true}`
	s2.UsedLicenses = intPtr(2)
	second, err := repo.Upsert(ctx, s2)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("ID changed across same-day re-run: %q -> %q", first.ID, second.ID)
	}
	if *second.UsedLicenses != 2 {
		t.Errorf("UsedLicenses = %v, want updated value 2", second.UsedLicenses)
	}

	rows, err := repo.ListByTenant(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows for one collection day, want exactly 1 (idempotency)", len(rows))
	}
}

func TestRepo_Upsert_DifferentDaysProduceSeparateRows(t *testing.T) {
	repo, tenantID := newTestDB(t)
	ctx := context.Background()

	for _, date := range []string{"2026-08-13", "2026-08-14"} {
		_, err := repo.Upsert(ctx, Snapshot{
			TenantID:       tenantID,
			CollectionDate: date,
			CollectedAtUTC: time.Now().UTC(),
			Status:         StatusSuccess,
			TotalLicenses:  intPtr(5),
			UsedLicenses:   intPtr(1),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	rows, err := repo.ListByTenant(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
}

// TestRepo_Upsert_FailedPollHasNilLicenseFigures asserts a failed poll is
// stored as unknown, never as a zero — project brief §2.
func TestRepo_Upsert_FailedPollHasNilLicenseFigures(t *testing.T) {
	repo, tenantID := newTestDB(t)
	ctx := context.Background()

	got, err := repo.Upsert(ctx, Snapshot{
		TenantID:       tenantID,
		CollectionDate: "2026-08-14",
		CollectedAtUTC: time.Now().UTC(),
		Status:         StatusUnreachable,
		ErrorMessage:   "connection refused",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalLicenses != nil || got.UsedLicenses != nil {
		t.Errorf("expected nil license figures for a failed poll, got Total=%v Used=%v", got.TotalLicenses, got.UsedLicenses)
	}
	if got.Status != StatusUnreachable {
		t.Errorf("Status = %q, want unreachable", got.Status)
	}
}

func TestRepo_GetByTenantAndDate_NotFound(t *testing.T) {
	repo, tenantID := newTestDB(t)
	if _, err := repo.GetByTenantAndDate(context.Background(), tenantID, "2026-01-01"); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRepo_BooleanFieldsRoundTrip(t *testing.T) {
	repo, tenantID := newTestDB(t)
	ctx := context.Background()

	got, err := repo.Upsert(ctx, Snapshot{
		TenantID:       tenantID,
		CollectionDate: "2026-08-14",
		CollectedAtUTC: time.Now().UTC(),
		Status:         StatusSuccess,
		Evaluation:     boolPtr(true),
		IsTrialExpired: boolPtr(false),
		IsTrial:        boolPtr(false),
		IsProUOnly:     boolPtr(false),
		IsFlexOnly:     boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Evaluation == nil || !*got.Evaluation {
		t.Errorf("Evaluation = %v, want true", got.Evaluation)
	}
	if got.IsFlexOnly == nil || !*got.IsFlexOnly {
		t.Errorf("IsFlexOnly = %v, want true", got.IsFlexOnly)
	}
	if got.IsTrial == nil || *got.IsTrial {
		t.Errorf("IsTrial = %v, want false", got.IsTrial)
	}
}
