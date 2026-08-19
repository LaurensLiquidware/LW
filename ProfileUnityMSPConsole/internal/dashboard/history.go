package dashboard

import (
	"sort"

	"profileunity-msp-console/internal/snapshot"
)

// EntitlementChange marks a day where a tenant's TotalLicenses changed
// between two successful collections — a contract change and an
// MSP-relevant event (project brief §7.4), not just a silent step in a
// usage line.
type EntitlementChange struct {
	Date      string
	FromTotal int
	ToTotal   int

	// FromUnlimited/ToUnlimited mirror IsUnlimitedLicense's meaning for
	// FromTotal/ToTotal, so callers never have to re-derive the "0 means
	// unlimited" convention themselves.
	FromUnlimited bool
	ToUnlimited   bool
}

// DetectEntitlementChanges walks points (assumed sorted by CollectionDate
// ascending, as snapshot.Repo.ListByTenant already returns them) and
// reports every day TotalLicenses differs from the last successful
// reading. Failed/unknown days are skipped when comparing — they carry
// no TotalLicenses to compare, and must never be treated as a change to
// or from zero.
func DetectEntitlementChanges(points []snapshot.Snapshot) []EntitlementChange {
	var changes []EntitlementChange
	haveLast := false
	lastTotal := 0

	for _, p := range points {
		if p.Status != snapshot.StatusSuccess || p.TotalLicenses == nil {
			continue
		}
		if haveLast && *p.TotalLicenses != lastTotal {
			changes = append(changes, EntitlementChange{
				Date:          p.CollectionDate,
				FromTotal:     lastTotal,
				ToTotal:       *p.TotalLicenses,
				FromUnlimited: IsUnlimitedLicense(&lastTotal),
				ToUnlimited:   IsUnlimitedLicense(p.TotalLicenses),
			})
		}
		lastTotal = *p.TotalLicenses
		haveLast = true
	}
	return changes
}

// PortfolioPoint is one collection day's totals across every tenant that
// reported successfully that day — the "aggregate view across all
// tenants" from project brief §7.4.
type PortfolioPoint struct {
	Date             string
	TotalUsed        int
	TotalEntitled    int
	TenantsReporting int

	// TenantsUnlimited is how many of that day's successfully-reporting
	// tenants have an unlimited license (see IsUnlimitedLicense). Such a
	// tenant contributes 0 to TotalEntitled -- harmless to the sum, but it
	// means TotalEntitled understates the true combined ceiling whenever
	// this is > 0, so callers should say so rather than presenting
	// TotalEntitled as a complete figure.
	TenantsUnlimited int

	// TenantsRegistered is how many tenants are registered now. It is a
	// simplification, not a historical count: a tenant added last week
	// makes every prior date look like it was "missing" that tenant, when
	// really it just didn't exist yet. Good enough to show reporting
	// coverage at a glance; not a substitute for the report coverage
	// language required in §7.5.
	TenantsRegistered int
}

// BuildPortfolioHistory groups every successful snapshot (across all
// tenants) by collection date and sums used/entitled licenses per day.
// tenantsRegistered is the current tenant count, attached to every point
// as context for how many were expected to report (see the caveat on
// TenantsRegistered above).
func BuildPortfolioHistory(tenantsRegistered int, allSuccess []snapshot.Snapshot) []PortfolioPoint {
	byDate := make(map[string]*PortfolioPoint)
	var dates []string

	for _, s := range allSuccess {
		if s.TotalLicenses == nil || s.UsedLicenses == nil {
			continue
		}
		p, ok := byDate[s.CollectionDate]
		if !ok {
			p = &PortfolioPoint{Date: s.CollectionDate, TenantsRegistered: tenantsRegistered}
			byDate[s.CollectionDate] = p
			dates = append(dates, s.CollectionDate)
		}
		p.TotalUsed += *s.UsedLicenses
		p.TotalEntitled += *s.TotalLicenses
		p.TenantsReporting++
		if IsUnlimitedLicense(s.TotalLicenses) {
			p.TenantsUnlimited++
		}
	}

	sort.Strings(dates)
	result := make([]PortfolioPoint, 0, len(dates))
	for _, d := range dates {
		result = append(result, *byDate[d])
	}
	return result
}
