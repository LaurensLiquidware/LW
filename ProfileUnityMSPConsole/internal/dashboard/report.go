package dashboard

import (
	"sort"

	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

// CoverageStatus summarizes how much of a reporting month's data can be
// trusted (project brief §7.5 requires reports to say this explicitly,
// not just show numbers as if every day were collected). It is
// deliberately a distinct type from DataStatus: a report describes a
// whole month, a dashboard describes "right now".
type CoverageStatus string

const (
	// CoverageComplete: every day in the month has a successful
	// collection.
	CoverageComplete CoverageStatus = "complete"
	// CoveragePartial: some days succeeded and some failed or are
	// missing (e.g. the tenant was registered mid-month, or the
	// scheduler missed a day).
	CoveragePartial CoverageStatus = "partial"
	// CoverageNone: not a single successful collection this month.
	CoverageNone CoverageStatus = "none"
)

// TenantMonthlyReport is one tenant's usage/entitlement summary for a
// single calendar month, built only from that month's snapshots — it
// never reaches outside the requested range, so a report for August
// never leaks a September entitlement change into "changes this month".
type TenantMonthlyReport struct {
	Tenant tenant.Tenant

	Year  int
	Month int // 1-12

	DaysInMonth        int
	DaysCollected      int // successful collections this month
	DaysFailed         int // attempted but failed this month
	DaysNeverAttempted int
	Coverage           CoverageStatus

	// PeakUsed/PeakUsedDate and AverageUsed are computed only from
	// successful days; nil/zero when Coverage is CoverageNone.
	PeakUsed     *int
	PeakUsedDate string
	AverageUsed  *float64

	// EntitledAtMonthEnd is TotalLicenses from the last successful
	// collection in the month, nil if there was none.
	EntitledAtMonthEnd *int

	EntitlementChanges []EntitlementChange
}

// BuildTenantMonthlyReport computes a TenantMonthlyReport from every
// snapshot in [from, to] (inclusive, both "YYYY-MM-DD", the calendar
// month's first and last day) for one tenant, regardless of status —
// the failed/missing days are what makes the Coverage field meaningful.
func BuildTenantMonthlyReport(t tenant.Tenant, year, month, daysInMonth int, monthSnapshots []snapshot.Snapshot) TenantMonthlyReport {
	r := TenantMonthlyReport{
		Tenant:      t,
		Year:        year,
		Month:       month,
		DaysInMonth: daysInMonth,
	}

	sorted := make([]snapshot.Snapshot, len(monthSnapshots))
	copy(sorted, monthSnapshots)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CollectionDate < sorted[j].CollectionDate })

	var usedSum, usedCount int
	for _, s := range sorted {
		if s.Status != snapshot.StatusSuccess {
			r.DaysFailed++
			continue
		}
		r.DaysCollected++
		if s.UsedLicenses != nil {
			usedSum += *s.UsedLicenses
			usedCount++
			if r.PeakUsed == nil || *s.UsedLicenses > *r.PeakUsed {
				v := *s.UsedLicenses
				r.PeakUsed = &v
				r.PeakUsedDate = s.CollectionDate
			}
		}
		if s.TotalLicenses != nil {
			v := *s.TotalLicenses
			r.EntitledAtMonthEnd = &v
		}
	}
	r.DaysNeverAttempted = daysInMonth - r.DaysCollected - r.DaysFailed
	if usedCount > 0 {
		avg := float64(usedSum) / float64(usedCount)
		r.AverageUsed = &avg
	}

	switch {
	case r.DaysCollected == 0:
		r.Coverage = CoverageNone
	case r.DaysCollected == daysInMonth:
		r.Coverage = CoverageComplete
	default:
		r.Coverage = CoveragePartial
	}

	r.EntitlementChanges = DetectEntitlementChanges(sorted)
	return r
}

// PortfolioMonthlyReport is the MSP-wide summary for one month: totals
// across every tenant, plus which tenants had entitlement changes or
// data gaps — the things an MSP operator needs to call out, not just a
// sum of numbers (project brief §7.5).
type PortfolioMonthlyReport struct {
	Year  int
	Month int

	TenantsRegistered int

	// PeakTotalUsed/PeakTotalUsedDate and AverageTotalUsed are computed
	// from each day's summed successful usage across tenants; a day with
	// zero successful tenants contributes nothing (never a zero).
	PeakTotalUsed     *int
	PeakTotalUsedDate string
	AverageTotalUsed  *float64

	TotalEntitledAtMonthEnd *int

	// TenantReports is every tenant's own monthly report, so the
	// portfolio view can show per-tenant coverage/entitlement-change
	// detail without a second round trip.
	TenantReports []TenantMonthlyReport
}

// BuildPortfolioMonthlyReport aggregates tenantReports (already built
// per-tenant via BuildTenantMonthlyReport) plus allMonthSnapshots (every
// tenant's snapshots for the month, successes and failures alike) into a
// portfolio-wide summary.
func BuildPortfolioMonthlyReport(year, month, tenantsRegistered int, tenantReports []TenantMonthlyReport, allMonthSnapshots []snapshot.Snapshot) PortfolioMonthlyReport {
	r := PortfolioMonthlyReport{
		Year:              year,
		Month:             month,
		TenantsRegistered: tenantsRegistered,
		TenantReports:     tenantReports,
	}

	dailyUsed := make(map[string]int)
	var dates []string
	for _, s := range allMonthSnapshots {
		if s.Status != snapshot.StatusSuccess || s.UsedLicenses == nil || s.TotalLicenses == nil {
			continue
		}
		if _, seen := dailyUsed[s.CollectionDate]; !seen {
			dates = append(dates, s.CollectionDate)
		}
		dailyUsed[s.CollectionDate] += *s.UsedLicenses
	}
	sort.Strings(dates)

	var sum, count int
	for _, d := range dates {
		used := dailyUsed[d]
		if r.PeakTotalUsed == nil || used > *r.PeakTotalUsed {
			v := used
			r.PeakTotalUsed = &v
			r.PeakTotalUsedDate = d
		}
		sum += used
		count++
	}
	if count > 0 {
		avg := float64(sum) / float64(count)
		r.AverageTotalUsed = &avg
	}

	// Summed from each tenant's own EntitledAtMonthEnd (its last
	// successful reading that month) rather than any single day's
	// snapshot rows — tenants can have their last success on different
	// days, and a single-day sum would silently drop anyone whose last
	// success wasn't the portfolio's single latest date.
	var entitledTotal int
	var haveAnyEntitled bool
	for _, tr := range tenantReports {
		if tr.EntitledAtMonthEnd != nil {
			entitledTotal += *tr.EntitledAtMonthEnd
			haveAnyEntitled = true
		}
	}
	if haveAnyEntitled {
		r.TotalEntitledAtMonthEnd = &entitledTotal
	}

	return r
}
