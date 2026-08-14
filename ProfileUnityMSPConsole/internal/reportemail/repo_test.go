package reportemail

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"profileunity-msp-console/internal/db"
)

func newTestRepo(t *testing.T) *Repo {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewRepo(sqlDB)
}

func TestAlreadySent_FalseWhenNeverRecorded(t *testing.T) {
	repo := newTestRepo(t)
	sent, err := repo.AlreadySent(context.Background(), 2026, 7)
	if err != nil {
		t.Fatalf("AlreadySent: %v", err)
	}
	if sent {
		t.Error("AlreadySent = true for a month never marked sent")
	}
}

func TestMarkSent_ThenAlreadySentIsTrue(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	if err := repo.MarkSent(ctx, 2026, 7, []string{"msp@liquidware.eu"}, time.Now()); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}

	sent, err := repo.AlreadySent(ctx, 2026, 7)
	if err != nil {
		t.Fatalf("AlreadySent: %v", err)
	}
	if !sent {
		t.Error("AlreadySent = false right after MarkSent for the same month")
	}

	// A different month is unaffected.
	sent, err = repo.AlreadySent(ctx, 2026, 8)
	if err != nil {
		t.Fatalf("AlreadySent: %v", err)
	}
	if sent {
		t.Error("AlreadySent = true for a different month that was never marked sent")
	}
}

func TestMarkSent_IsIdempotentAcrossDuplicateCalls(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	if err := repo.MarkSent(ctx, 2026, 7, []string{"msp@liquidware.eu"}, time.Now()); err != nil {
		t.Fatalf("first MarkSent: %v", err)
	}
	// The UNIQUE(year, month) constraint means a second, buggy call for
	// the same month must fail loudly rather than silently duplicate --
	// callers are expected to check AlreadySent first (as the scheduler
	// does), so this proves the constraint is actually in effect.
	if err := repo.MarkSent(ctx, 2026, 7, []string{"msp@liquidware.eu"}, time.Now()); err == nil {
		t.Fatal("expected the UNIQUE(year, month) constraint to reject a duplicate MarkSent")
	}
}
