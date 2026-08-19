package dashboard

import (
	"testing"

	"profileunity-msp-console/internal/snapshot"
)

func successPoint(date string, total, used int) snapshot.Snapshot {
	return snapshot.Snapshot{CollectionDate: date, Status: snapshot.StatusSuccess, TotalLicenses: intPtr(total), UsedLicenses: intPtr(used)}
}

func failedPoint(date string) snapshot.Snapshot {
	return snapshot.Snapshot{CollectionDate: date, Status: snapshot.StatusUnreachable}
}

func TestDetectEntitlementChanges_NoChange(t *testing.T) {
	points := []snapshot.Snapshot{
		successPoint("2026-08-01", 10, 1),
		successPoint("2026-08-02", 10, 2),
		successPoint("2026-08-03", 10, 3),
	}
	if got := DetectEntitlementChanges(points); len(got) != 0 {
		t.Errorf("got %+v, want none", got)
	}
}

func TestDetectEntitlementChanges_OneChange(t *testing.T) {
	points := []snapshot.Snapshot{
		successPoint("2026-08-01", 10, 1),
		successPoint("2026-08-02", 10, 2),
		successPoint("2026-08-03", 15, 3),
		successPoint("2026-08-04", 15, 4),
	}
	got := DetectEntitlementChanges(points)
	if len(got) != 1 {
		t.Fatalf("got %+v, want 1 change", got)
	}
	if got[0] != (EntitlementChange{Date: "2026-08-03", FromTotal: 10, ToTotal: 15}) {
		t.Errorf("got %+v", got[0])
	}
}

func TestDetectEntitlementChanges_FailedDaysDoNotCountAsChanges(t *testing.T) {
	points := []snapshot.Snapshot{
		successPoint("2026-08-01", 10, 1),
		failedPoint("2026-08-02"),
		failedPoint("2026-08-03"),
		successPoint("2026-08-04", 10, 4), // same total as the last success — no change
	}
	if got := DetectEntitlementChanges(points); len(got) != 0 {
		t.Errorf("got %+v, want none (gaps must not be compared as 0)", got)
	}
}

func TestDetectEntitlementChanges_FirstPointNeverCounts(t *testing.T) {
	points := []snapshot.Snapshot{successPoint("2026-08-01", 10, 1)}
	if got := DetectEntitlementChanges(points); len(got) != 0 {
		t.Errorf("got %+v, want none (nothing to compare the first point against)", got)
	}
}

func TestDetectEntitlementChanges_BecameUnlimited(t *testing.T) {
	points := []snapshot.Snapshot{
		successPoint("2026-08-01", 5, 3),
		successPoint("2026-08-02", 5, 4),
		successPoint("2026-08-03", 0, 4), // became unlimited
		successPoint("2026-08-04", 5, 5), // back to a real cap
	}
	got := DetectEntitlementChanges(points)
	if len(got) != 2 {
		t.Fatalf("got %+v, want 2 changes", got)
	}
	if !got[0].ToUnlimited || got[0].FromUnlimited {
		t.Errorf("first change = %+v, want ToUnlimited=true, FromUnlimited=false", got[0])
	}
	if !got[1].FromUnlimited || got[1].ToUnlimited {
		t.Errorf("second change = %+v, want FromUnlimited=true, ToUnlimited=false", got[1])
	}
}

func TestBuildPortfolioHistory_CountsUnlimitedTenants(t *testing.T) {
	all := []snapshot.Snapshot{
		{TenantID: "a", CollectionDate: "2026-08-01", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(10), UsedLicenses: intPtr(1)},
		{TenantID: "b", CollectionDate: "2026-08-01", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(0), UsedLicenses: intPtr(5)},
	}
	got := BuildPortfolioHistory(2, all)
	if len(got) != 1 {
		t.Fatalf("got %d points, want 1", len(got))
	}
	if got[0].TenantsUnlimited != 1 {
		t.Errorf("TenantsUnlimited = %d, want 1", got[0].TenantsUnlimited)
	}
	if got[0].TotalEntitled != 10 {
		t.Errorf("TotalEntitled = %d, want 10 (unlimited tenant contributes 0)", got[0].TotalEntitled)
	}
}

func TestBuildPortfolioHistory(t *testing.T) {
	all := []snapshot.Snapshot{
		{TenantID: "a", CollectionDate: "2026-08-01", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(10), UsedLicenses: intPtr(1)},
		{TenantID: "b", CollectionDate: "2026-08-01", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(20), UsedLicenses: intPtr(5)},
		{TenantID: "a", CollectionDate: "2026-08-02", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(10), UsedLicenses: intPtr(2)},
		// tenant b missing on 08-02 -- only one tenant reporting that day
	}

	got := BuildPortfolioHistory(2, all)
	if len(got) != 2 {
		t.Fatalf("got %d points, want 2", len(got))
	}
	if got[0].Date != "2026-08-01" || got[0].TotalUsed != 6 || got[0].TotalEntitled != 30 || got[0].TenantsReporting != 2 {
		t.Errorf("day 1 = %+v", got[0])
	}
	if got[1].Date != "2026-08-02" || got[1].TotalUsed != 2 || got[1].TotalEntitled != 10 || got[1].TenantsReporting != 1 {
		t.Errorf("day 2 = %+v", got[1])
	}
	if got[0].TenantsRegistered != 2 {
		t.Errorf("TenantsRegistered = %d, want 2", got[0].TenantsRegistered)
	}
}

func TestBuildPortfolioHistory_SortedByDate(t *testing.T) {
	all := []snapshot.Snapshot{
		{TenantID: "a", CollectionDate: "2026-08-03", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(1), UsedLicenses: intPtr(1)},
		{TenantID: "a", CollectionDate: "2026-08-01", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(1), UsedLicenses: intPtr(1)},
		{TenantID: "a", CollectionDate: "2026-08-02", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(1), UsedLicenses: intPtr(1)},
	}
	got := BuildPortfolioHistory(1, all)
	dates := []string{got[0].Date, got[1].Date, got[2].Date}
	want := []string{"2026-08-01", "2026-08-02", "2026-08-03"}
	for i := range want {
		if dates[i] != want[i] {
			t.Errorf("dates = %v, want %v", dates, want)
		}
	}
}
