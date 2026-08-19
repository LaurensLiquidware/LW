// Package dashboard computes the at-a-glance state of a tenant from its
// registration plus its collection history — pure functions, decoupled
// from HTTP/UI, so the dashboard, monthly reports, and exports (project
// brief §6: "business logic as pure functions... this paid off when one
// calculation had to serve a screen, a PDF report, and eight other
// views") always agree on what "near limit" or "stale" means.
package dashboard

import (
	"time"

	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

// UsageStatus is how close a tenant is to its license limit. It uses the
// Good/Fair/Poor language from the design system (project brief §10)
// rather than a bespoke palette.
type UsageStatus string

const (
	UsageGood    UsageStatus = "good"
	UsageFair    UsageStatus = "fair"
	UsagePoor    UsageStatus = "poor"
	UsageUnknown UsageStatus = "unknown" // no reliable used/total figures

	// UsageUnlimited: the license has no seat cap (see IsUnlimitedLicense).
	// Deliberately not part of the Good/Fair/Poor palette -- like
	// DataStatus's non-GFP states, this isn't "good" or "bad", it's a
	// different kind of license entirely, with no utilization percentage
	// to speak of.
	UsageUnlimited UsageStatus = "unlimited"
)

// IsUnlimitedLicense reports whether total represents ProfileUnity's own
// "unlimited seats" convention -- a successful collection where
// TotalLicenses is exactly 0. ProfileUnity's /licenseinfo API has no
// dedicated "unlimited" flag; confirmed against a real ProfileUnity
// console that it reports a literal 0 for an unlimited-seat license, and
// a genuine zero-seat license is not a state that API produces -- so a
// successfully-collected 0 always means unlimited, never "broken".
func IsUnlimitedLicense(total *int) bool {
	return total != nil && *total == 0
}

// ExpiryStatus is how close a tenant's support entitlement is to lapsing.
type ExpiryStatus string

const (
	ExpiryOK           ExpiryStatus = "ok"
	ExpiryExpiringSoon ExpiryStatus = "expiring_soon"
	ExpiryExpired      ExpiryStatus = "expired"
	ExpiryUnknown      ExpiryStatus = "unknown" // no SupportEnds figure
)

// DataStatus is whether the figures above can be trusted at all. This is
// deliberately never rendered with the GFP palette (project brief §10:
// "keep 'stale data' and 'collection failing' visually distinct from
// 'poor', because they mean we don't know, not bad") — a console that's
// unreachable must never look like a console that's merely over its
// license limit.
type DataStatus string

const (
	DataOK             DataStatus = "ok"
	DataStale          DataStatus = "stale"
	DataFailing        DataStatus = "failing"
	DataNeverCollected DataStatus = "never_collected"
)

// Tuning constants. Nothing in the project brief pins exact thresholds;
// these are reasonable operational defaults, factored out so a future
// phase can make them configurable without touching the logic itself.
const (
	// NearLimitThreshold: usage at or above this fraction of the license
	// limit is "fair" (near limit); at or above 1.0 it is "poor" (at limit).
	NearLimitThreshold = 0.90

	// ExpiringSoonDays: a support entitlement lapsing within this many
	// days (but not yet lapsed) is "expiring soon".
	ExpiringSoonDays = 30

	// StaleAfterDays: a tenant whose most recent collection succeeded,
	// but that success is older than this many collection days, is
	// "stale" rather than "ok" — the dashboard is showing real data, just
	// not necessarily today's.
	StaleAfterDays = 2
)

// TenantStatus is one tenant's full at-a-glance state.
type TenantStatus struct {
	Tenant tenant.Tenant

	// Latest is the most recent collection attempt regardless of
	// outcome; nil if this tenant has never been collected.
	Latest *snapshot.Snapshot

	// LatestSuccess is the most recent successful collection, which may
	// be older than Latest (e.g. today's attempt failed but yesterday's
	// succeeded) or nil if there has never been a success.
	LatestSuccess *snapshot.Snapshot

	Usage  UsageStatus
	Expiry ExpiryStatus
	Data   DataStatus

	// UtilizationPercent is UsedLicenses/TotalLicenses from LatestSuccess,
	// nil when unknown. It can exceed 1.0 (over the limit). Also nil when
	// Usage is UsageUnlimited -- there's no ceiling to divide against.
	UtilizationPercent *float64

	// ExpiryRunwayDays is days until SupportEnds (negative if already
	// past), nil when unknown.
	ExpiryRunwayDays *int
}

// Compute derives a TenantStatus. now/loc set the "as of" instant and the
// timezone collection days are measured in (project brief §7.2's
// configured collection timezone) — always pass the same loc used by the
// scheduler, so "today" agrees everywhere.
func Compute(t tenant.Tenant, latest, latestSuccess *snapshot.Snapshot, now time.Time, loc *time.Location) TenantStatus {
	ts := TenantStatus{Tenant: t, Latest: latest, LatestSuccess: latestSuccess}
	ts.Data = computeDataStatus(latest, now, loc)
	ts.Usage, ts.UtilizationPercent = computeUsageStatus(latestSuccess)
	ts.Expiry, ts.ExpiryRunwayDays = computeExpiryStatus(latestSuccess, now, loc)
	return ts
}

func computeDataStatus(latest *snapshot.Snapshot, now time.Time, loc *time.Location) DataStatus {
	if latest == nil {
		return DataNeverCollected
	}
	if latest.Status != snapshot.StatusSuccess {
		return DataFailing
	}
	today := snapshot.CollectionDateFor(now, loc)
	days, err := daysBetween(latest.CollectionDate, today)
	if err != nil || days > StaleAfterDays {
		return DataStale
	}
	return DataOK
}

func computeUsageStatus(latestSuccess *snapshot.Snapshot) (UsageStatus, *float64) {
	if latestSuccess == nil || latestSuccess.TotalLicenses == nil || latestSuccess.UsedLicenses == nil {
		return UsageUnknown, nil
	}
	if IsUnlimitedLicense(latestSuccess.TotalLicenses) {
		return UsageUnlimited, nil
	}
	if *latestSuccess.TotalLicenses <= 0 {
		return UsageUnknown, nil
	}
	pct := float64(*latestSuccess.UsedLicenses) / float64(*latestSuccess.TotalLicenses)
	switch {
	case pct >= 1.0:
		return UsagePoor, &pct
	case pct >= NearLimitThreshold:
		return UsageFair, &pct
	default:
		return UsageGood, &pct
	}
}

func computeExpiryStatus(latestSuccess *snapshot.Snapshot, now time.Time, loc *time.Location) (ExpiryStatus, *int) {
	if latestSuccess == nil || latestSuccess.SupportEndsISO == "" {
		return ExpiryUnknown, nil
	}
	today := snapshot.CollectionDateFor(now, loc)
	days, err := daysBetween(today, latestSuccess.SupportEndsISO)
	if err != nil {
		return ExpiryUnknown, nil
	}
	switch {
	case days < 0:
		return ExpiryExpired, &days
	case days <= ExpiringSoonDays:
		return ExpiryExpiringSoon, &days
	default:
		return ExpiryOK, &days
	}
}

// daysBetween returns the number of days from "from" to "to", both
// YYYY-MM-DD, computed on calendar dates rather than elapsed duration —
// safe against DST and the sub-day noise a straight time.Sub would add.
func daysBetween(from, to string) (int, error) {
	const layout = "2006-01-02"
	fromT, err := time.Parse(layout, from)
	if err != nil {
		return 0, err
	}
	toT, err := time.Parse(layout, to)
	if err != nil {
		return 0, err
	}
	return int(toT.Sub(fromT).Hours() / 24), nil
}
