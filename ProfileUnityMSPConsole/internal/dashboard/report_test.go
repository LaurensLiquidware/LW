package dashboard

import (
	"testing"

	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

func TestBuildTenantMonthlyReport_CompleteMonth(t *testing.T) {
	tn := tenant.Tenant{ID: "a", DisplayName: "Acme"}
	points := []snapshot.Snapshot{
		successPoint("2026-08-01", 10, 4),
		successPoint("2026-08-02", 10, 6),
		successPoint("2026-08-03", 10, 2),
	}

	r := BuildTenantMonthlyReport(tn, 2026, 8, 3, points)

	if r.Coverage != CoverageComplete {
		t.Errorf("Coverage = %v, want complete", r.Coverage)
	}
	if r.DaysCollected != 3 || r.DaysFailed != 0 || r.DaysNeverAttempted != 0 {
		t.Errorf("days: collected=%d failed=%d neverAttempted=%d", r.DaysCollected, r.DaysFailed, r.DaysNeverAttempted)
	}
	if r.PeakUsed == nil || *r.PeakUsed != 6 || r.PeakUsedDate != "2026-08-02" {
		t.Errorf("peak = %v on %v, want 6 on 2026-08-02", r.PeakUsed, r.PeakUsedDate)
	}
	if r.AverageUsed == nil || *r.AverageUsed != 4 {
		t.Errorf("average = %v, want 4", r.AverageUsed)
	}
	if r.EntitledAtMonthEnd == nil || *r.EntitledAtMonthEnd != 2 {
		t.Errorf("entitled at month end = %v, want 2 (used licenses on the last day)", r.EntitledAtMonthEnd)
	}
	if r.MaximumUsersAtMonthEnd == nil || *r.MaximumUsersAtMonthEnd != 10 {
		t.Errorf("maximum users at month end = %v, want 10", r.MaximumUsersAtMonthEnd)
	}
}

func TestBuildTenantMonthlyReport_LicenseProductAtMonthEndFromLastSuccess(t *testing.T) {
	tn := tenant.Tenant{ID: "a"}
	points := []snapshot.Snapshot{
		{CollectionDate: "2026-08-01", Status: snapshot.StatusSuccess, LicenseProduct: "ProU"},
		{CollectionDate: "2026-08-02", Status: snapshot.StatusUnreachable, LicenseProduct: "ignored-on-a-failed-day"},
		{CollectionDate: "2026-08-03", Status: snapshot.StatusSuccess, LicenseProduct: "ProU+FlexApp"},
	}

	r := BuildTenantMonthlyReport(tn, 2026, 8, 3, points)

	if r.LicenseProductAtMonthEnd != "ProU+FlexApp" {
		t.Errorf("LicenseProductAtMonthEnd = %q, want %q (the last successful day's value)", r.LicenseProductAtMonthEnd, "ProU+FlexApp")
	}
}

func TestBuildTenantMonthlyReport_LicenseProductAtMonthEndEmptyWithNoSuccesses(t *testing.T) {
	tn := tenant.Tenant{ID: "a"}
	points := []snapshot.Snapshot{failedPoint("2026-08-01"), failedPoint("2026-08-02")}

	r := BuildTenantMonthlyReport(tn, 2026, 8, 2, points)

	if r.LicenseProductAtMonthEnd != "" {
		t.Errorf("LicenseProductAtMonthEnd = %q, want empty with zero successes", r.LicenseProductAtMonthEnd)
	}
}

func TestBuildTenantMonthlyReport_PartialMonthWithFailuresAndGaps(t *testing.T) {
	tn := tenant.Tenant{ID: "a"}
	points := []snapshot.Snapshot{
		successPoint("2026-08-01", 10, 4),
		failedPoint("2026-08-02"),
		// 2026-08-03 never attempted at all
	}

	r := BuildTenantMonthlyReport(tn, 2026, 8, 3, points)

	if r.Coverage != CoveragePartial {
		t.Errorf("Coverage = %v, want partial", r.Coverage)
	}
	if r.DaysCollected != 1 || r.DaysFailed != 1 || r.DaysNeverAttempted != 1 {
		t.Errorf("days: collected=%d failed=%d neverAttempted=%d, want 1/1/1", r.DaysCollected, r.DaysFailed, r.DaysNeverAttempted)
	}
}

func TestBuildTenantMonthlyReport_NoSuccessfulDays(t *testing.T) {
	tn := tenant.Tenant{ID: "a"}
	points := []snapshot.Snapshot{failedPoint("2026-08-01"), failedPoint("2026-08-02")}

	r := BuildTenantMonthlyReport(tn, 2026, 8, 2, points)

	if r.Coverage != CoverageNone {
		t.Errorf("Coverage = %v, want none", r.Coverage)
	}
	if r.PeakUsed != nil || r.AverageUsed != nil || r.EntitledAtMonthEnd != nil || r.MaximumUsersAtMonthEnd != nil {
		t.Errorf("expected nil figures with zero successes, got peak=%v avg=%v entitled=%v maxUsers=%v", r.PeakUsed, r.AverageUsed, r.EntitledAtMonthEnd, r.MaximumUsersAtMonthEnd)
	}
}

func TestBuildTenantMonthlyReport_NeverCollectedAtAll(t *testing.T) {
	tn := tenant.Tenant{ID: "a"}
	r := BuildTenantMonthlyReport(tn, 2026, 8, 31, nil)

	if r.Coverage != CoverageNone {
		t.Errorf("Coverage = %v, want none", r.Coverage)
	}
	if r.DaysNeverAttempted != 31 {
		t.Errorf("DaysNeverAttempted = %d, want 31", r.DaysNeverAttempted)
	}
}

func TestBuildTenantMonthlyReport_EntitlementChangeWithinMonth(t *testing.T) {
	tn := tenant.Tenant{ID: "a"}
	points := []snapshot.Snapshot{
		successPoint("2026-08-01", 10, 1),
		successPoint("2026-08-15", 15, 1),
	}

	r := BuildTenantMonthlyReport(tn, 2026, 8, 15, points)

	if len(r.EntitlementChanges) != 1 {
		t.Fatalf("got %+v, want 1 change", r.EntitlementChanges)
	}
	if r.EntitlementChanges[0] != (EntitlementChange{Date: "2026-08-15", FromTotal: 10, ToTotal: 15}) {
		t.Errorf("got %+v", r.EntitlementChanges[0])
	}
}

func TestBuildTenantMonthlyReport_OutOfOrderInputIsSortedFirst(t *testing.T) {
	tn := tenant.Tenant{ID: "a"}
	points := []snapshot.Snapshot{
		successPoint("2026-08-03", 10, 9),
		successPoint("2026-08-01", 10, 1),
		successPoint("2026-08-02", 10, 5),
	}

	r := BuildTenantMonthlyReport(tn, 2026, 8, 3, points)

	if r.PeakUsed == nil || *r.PeakUsed != 9 || r.PeakUsedDate != "2026-08-03" {
		t.Errorf("peak = %v on %v, want 9 on 2026-08-03", r.PeakUsed, r.PeakUsedDate)
	}
}

func TestBuildPortfolioMonthlyReport_SumsAcrossTenants(t *testing.T) {
	tenantA := BuildTenantMonthlyReport(tenant.Tenant{ID: "a"}, 2026, 8, 2, []snapshot.Snapshot{
		{TenantID: "a", CollectionDate: "2026-08-01", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(10), UsedLicenses: intPtr(3)},
		{TenantID: "a", CollectionDate: "2026-08-02", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(10), UsedLicenses: intPtr(4)},
	})
	tenantB := BuildTenantMonthlyReport(tenant.Tenant{ID: "b"}, 2026, 8, 2, []snapshot.Snapshot{
		{TenantID: "b", CollectionDate: "2026-08-01", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(20), UsedLicenses: intPtr(5)},
		// tenant b failed on 08-02
		{TenantID: "b", CollectionDate: "2026-08-02", Status: snapshot.StatusUnreachable},
	})

	allSnapshots := []snapshot.Snapshot{
		{TenantID: "a", CollectionDate: "2026-08-01", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(10), UsedLicenses: intPtr(3)},
		{TenantID: "b", CollectionDate: "2026-08-01", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(20), UsedLicenses: intPtr(5)},
		{TenantID: "a", CollectionDate: "2026-08-02", Status: snapshot.StatusSuccess, TotalLicenses: intPtr(10), UsedLicenses: intPtr(4)},
		{TenantID: "b", CollectionDate: "2026-08-02", Status: snapshot.StatusUnreachable},
	}

	r := BuildPortfolioMonthlyReport(2026, 8, 2, []TenantMonthlyReport{tenantA, tenantB}, allSnapshots)

	// day 1: 3+5=8, day 2: only tenant a succeeded, so 4.
	if r.PeakTotalUsed == nil || *r.PeakTotalUsed != 8 || r.PeakTotalUsedDate != "2026-08-01" {
		t.Errorf("peak = %v on %v, want 8 on 2026-08-01", r.PeakTotalUsed, r.PeakTotalUsedDate)
	}
	if r.AverageTotalUsed == nil || *r.AverageTotalUsed != 6 {
		t.Errorf("average = %v, want 6 ((8+4)/2)", r.AverageTotalUsed)
	}
	// tenant a's entitled (used licenses) at month end = 4 (08-02), tenant b's = 5 (08-01, its only success)
	if r.TotalEntitledAtMonthEnd == nil || *r.TotalEntitledAtMonthEnd != 9 {
		t.Errorf("total entitled = %v, want 9", r.TotalEntitledAtMonthEnd)
	}
	// tenant a's maximum users at month end = 10 (08-02), tenant b's = 20 (08-01, its only success)
	if r.TotalMaximumUsersAtMonthEnd == nil || *r.TotalMaximumUsersAtMonthEnd != 30 {
		t.Errorf("total maximum users = %v, want 30", r.TotalMaximumUsersAtMonthEnd)
	}
}

func TestBuildPortfolioMonthlyReport_NoTenantsReported(t *testing.T) {
	r := BuildPortfolioMonthlyReport(2026, 8, 3, nil, nil)
	if r.PeakTotalUsed != nil || r.AverageTotalUsed != nil || r.TotalEntitledAtMonthEnd != nil || r.TotalMaximumUsersAtMonthEnd != nil {
		t.Errorf("expected all nil with no data, got %+v", r)
	}
	if r.TenantsRegistered != 3 {
		t.Errorf("TenantsRegistered = %d, want 3", r.TenantsRegistered)
	}
}
