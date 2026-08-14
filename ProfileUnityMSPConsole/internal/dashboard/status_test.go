package dashboard

import (
	"testing"
	"time"

	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

func intPtr(v int) *int { return &v }

var fixedNow = time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)

func successSnapshot(collectionDate string, used, total int, supportEndsISO string) *snapshot.Snapshot {
	return &snapshot.Snapshot{
		CollectionDate: collectionDate,
		Status:         snapshot.StatusSuccess,
		UsedLicenses:   intPtr(used),
		TotalLicenses:  intPtr(total),
		SupportEndsISO: supportEndsISO,
	}
}

func TestCompute_NeverCollected(t *testing.T) {
	ts := Compute(tenant.Tenant{}, nil, nil, fixedNow, time.UTC)
	if ts.Data != DataNeverCollected {
		t.Errorf("Data = %q, want never_collected", ts.Data)
	}
	if ts.Usage != UsageUnknown || ts.Expiry != ExpiryUnknown {
		t.Errorf("Usage = %q, Expiry = %q, want unknown/unknown", ts.Usage, ts.Expiry)
	}
}

func TestCompute_LatestAttemptFailing(t *testing.T) {
	latest := &snapshot.Snapshot{CollectionDate: "2026-08-14", Status: snapshot.StatusUnreachable}
	latestSuccess := successSnapshot("2026-08-10", 1, 5, "2027-01-01")
	ts := Compute(tenant.Tenant{}, latest, latestSuccess, fixedNow, time.UTC)

	if ts.Data != DataFailing {
		t.Errorf("Data = %q, want failing", ts.Data)
	}
	// Usage/expiry still come from the last success, even though the
	// most recent attempt itself failed.
	if ts.Usage != UsageGood {
		t.Errorf("Usage = %q, want good", ts.Usage)
	}
}

func TestCompute_FreshSuccess(t *testing.T) {
	latest := successSnapshot("2026-08-14", 1, 5, "2027-01-01")
	ts := Compute(tenant.Tenant{}, latest, latest, fixedNow, time.UTC)
	if ts.Data != DataOK {
		t.Errorf("Data = %q, want ok", ts.Data)
	}
}

func TestCompute_StaleSuccess(t *testing.T) {
	// Latest attempt succeeded, but it was 5 collection-days ago.
	latest := successSnapshot("2026-08-09", 1, 5, "2027-01-01")
	ts := Compute(tenant.Tenant{}, latest, latest, fixedNow, time.UTC)
	if ts.Data != DataStale {
		t.Errorf("Data = %q, want stale", ts.Data)
	}
}

func TestCompute_StaleBoundary(t *testing.T) {
	// Exactly StaleAfterDays old is still "ok"; one more day is "stale".
	okLatest := successSnapshot("2026-08-12", 1, 5, "2027-01-01") // 2 days old
	ts := Compute(tenant.Tenant{}, okLatest, okLatest, fixedNow, time.UTC)
	if ts.Data != DataOK {
		t.Errorf("2 days old: Data = %q, want ok", ts.Data)
	}

	staleLatest := successSnapshot("2026-08-11", 1, 5, "2027-01-01") // 3 days old
	ts2 := Compute(tenant.Tenant{}, staleLatest, staleLatest, fixedNow, time.UTC)
	if ts2.Data != DataStale {
		t.Errorf("3 days old: Data = %q, want stale", ts2.Data)
	}
}

func TestCompute_UsageThresholds(t *testing.T) {
	cases := []struct {
		name    string
		used    int
		total   int
		want    UsageStatus
		wantPct float64
	}{
		{"comfortably under", 1, 5, UsageGood, 0.2},
		{"just under near-limit", 89, 100, UsageGood, 0.89},
		{"at near-limit boundary", 90, 100, UsageFair, 0.90},
		{"between near-limit and full", 99, 100, UsageFair, 0.99},
		{"exactly at limit", 100, 100, UsagePoor, 1.0},
		{"over limit", 110, 100, UsagePoor, 1.10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			latest := successSnapshot("2026-08-14", c.used, c.total, "2027-01-01")
			ts := Compute(tenant.Tenant{}, latest, latest, fixedNow, time.UTC)
			if ts.Usage != c.want {
				t.Errorf("Usage = %q, want %q", ts.Usage, c.want)
			}
			if ts.UtilizationPercent == nil || *ts.UtilizationPercent != c.wantPct {
				t.Errorf("UtilizationPercent = %v, want %v", ts.UtilizationPercent, c.wantPct)
			}
		})
	}
}

func TestCompute_UsageUnknownWhenTotalIsZeroOrMissing(t *testing.T) {
	zeroTotal := successSnapshot("2026-08-14", 0, 0, "2027-01-01")
	ts := Compute(tenant.Tenant{}, zeroTotal, zeroTotal, fixedNow, time.UTC)
	if ts.Usage != UsageUnknown {
		t.Errorf("Usage = %q, want unknown for a zero total", ts.Usage)
	}

	missing := &snapshot.Snapshot{CollectionDate: "2026-08-14", Status: snapshot.StatusSuccess}
	ts2 := Compute(tenant.Tenant{}, missing, missing, fixedNow, time.UTC)
	if ts2.Usage != UsageUnknown {
		t.Errorf("Usage = %q, want unknown for missing figures", ts2.Usage)
	}
}

func TestCompute_ExpiryThresholds(t *testing.T) {
	cases := []struct {
		name         string
		supportEnds  string
		want         ExpiryStatus
		wantDaysSign int // -1 negative, 0 zero-or-positive-small, 1 positive-large
	}{
		{"far in the future", "2027-06-01", ExpiryOK, 1},
		{"exactly 30 days out", "2026-09-13", ExpiryExpiringSoon, 0},
		{"31 days out", "2026-09-14", ExpiryOK, 1},
		{"tomorrow", "2026-08-15", ExpiryExpiringSoon, 0},
		{"today", "2026-08-14", ExpiryExpiringSoon, 0},
		{"yesterday", "2026-08-13", ExpiryExpired, -1},
		{"long expired", "2020-01-01", ExpiryExpired, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			latest := successSnapshot("2026-08-14", 1, 5, c.supportEnds)
			ts := Compute(tenant.Tenant{}, latest, latest, fixedNow, time.UTC)
			if ts.Expiry != c.want {
				t.Errorf("Expiry = %q, want %q (runway=%v)", ts.Expiry, c.want, ts.ExpiryRunwayDays)
			}
			if ts.ExpiryRunwayDays == nil {
				t.Fatal("ExpiryRunwayDays should not be nil")
			}
		})
	}
}

func TestCompute_ExpiryUnknownWhenSupportEndsMissing(t *testing.T) {
	latest := &snapshot.Snapshot{CollectionDate: "2026-08-14", Status: snapshot.StatusSuccess, UsedLicenses: intPtr(1), TotalLicenses: intPtr(5)}
	ts := Compute(tenant.Tenant{}, latest, latest, fixedNow, time.UTC)
	if ts.Expiry != ExpiryUnknown {
		t.Errorf("Expiry = %q, want unknown", ts.Expiry)
	}
	if ts.ExpiryRunwayDays != nil {
		t.Errorf("ExpiryRunwayDays = %v, want nil", ts.ExpiryRunwayDays)
	}
}

func TestDaysBetween(t *testing.T) {
	cases := []struct {
		from, to string
		want     int
	}{
		{"2026-08-14", "2026-08-14", 0},
		{"2026-08-14", "2026-08-15", 1},
		{"2026-08-15", "2026-08-14", -1},
		{"2026-01-01", "2026-03-01", 59}, // 2026 is not a leap year
	}
	for _, c := range cases {
		got, err := daysBetween(c.from, c.to)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("daysBetween(%q, %q) = %d, want %d", c.from, c.to, got, c.want)
		}
	}
}
