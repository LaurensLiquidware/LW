package main

import (
	"fmt"
	"math/rand/v2"
	"time"

	"profileunity-msp-console/internal/snapshot"
)

// scatteredFailure is one of the 2-3 single-day failures sprinkled across
// tenants other than Havenbedrijf's dedicated outage -- see roster.go's
// storyOutage and generateHistory's outage handling for that one.
type scatteredFailure struct {
	tenantIndex int
	dayOffset   int // days after the tenant's own history start
	status      snapshot.Status
}

// scatteredFailures is fixed (not seed-derived) so the specific days/causes
// affected are easy to reason about and document, while the *usage*
// figures around them still vary with --seed. Chosen to land inside every
// affected tenant's non-absent history window regardless of --months (each
// offset is small and relative to that tenant's own start, not the global
// start).
var scatteredFailures = []scatteredFailure{
	{tenantIndex: 1, dayOffset: 40, status: snapshot.StatusTimeout},     // Vaandel
	{tenantIndex: 3, dayOffset: 75, status: snapshot.StatusUnreachable}, // Zandpoort
	{tenantIndex: 6, dayOffset: 110, status: snapshot.StatusTLSError},   // Delta
}

// outageStatuses is Havenbedrijf's 5 consecutive days, one distinct cause
// per day, in the order the task brief lists them: timeout, TLS
// verification failure, HTTP 401, connection refused, HTML error page
// instead of JSON.
var outageStatuses = []snapshot.Status{
	snapshot.StatusTimeout,
	snapshot.StatusTLSError,
	snapshot.StatusAuthRequired,
	snapshot.StatusUnreachable,
	snapshot.StatusMalformed,
}

// errorMessageFor returns a realistic, plausible ErrorMessage for a
// failure status -- mirroring the wording internal/profileunity's error
// types actually produce (see internal/profileunity/errors.go), so a demo
// failure row reads the same as a real one.
func errorMessageFor(status snapshot.Status) string {
	switch status {
	case snapshot.StatusUnreachable:
		return "dial tcp: connect: connection refused"
	case snapshot.StatusTimeout:
		return "context deadline exceeded"
	case snapshot.StatusTLSError:
		return "tls: failed to verify certificate: x509: certificate signed by unknown authority"
	case snapshot.StatusAuthRequired:
		return "unexpected status 401"
	case snapshot.StatusMalformed:
		return "malformed response: unexpected content type text/html"
	default:
		return "unexpected error"
	}
}

// rawPayloadForFailure returns what raw_payload holds for a failure --
// empty for transport-level failures where no body was ever read, and a
// garbage HTML body for "malformed", matching what the real collector
// leaves behind for each case (see internal/collector/collector.go:57-58's
// "raw payload is set before the error is classified" behavior).
func rawPayloadForFailure(status snapshot.Status) string {
	if status == snapshot.StatusMalformed {
		return "<html><head><title>500 Internal Server Error</title></head><body><h1>Internal Server Error</h1></body></html>"
	}
	return ""
}

// weekdayFactor models how a tenant's used-license count moves over a
// week: Concurrent-mode tenants drop sharply at weekends (seats are
// released when nobody's logged in), Named User tenants barely move
// (entitlements are assigned, not concurrently shared).
func weekdayFactor(mode string, day time.Weekday) float64 {
	weekend := day == time.Saturday || day == time.Sunday
	if mode == "Concurrent" {
		if weekend {
			return 0.35
		}
		return 1.0
	}
	if weekend {
		return 0.92
	}
	return 1.0
}

// holidayFactor applies a dip around late December and the summer period,
// on top of whatever a tenant's own story already does for that stretch
// (storySeasonal's summer floor stacks with this, which is realistic --
// both effects reduce usage in July/August).
func holidayFactor(day time.Time) float64 {
	month, d := day.Month(), day.Day()
	switch {
	case month == time.December && d >= 20:
		return 0.5
	case month == time.July && d >= 15:
		return 0.7
	case month == time.August && d <= 15:
		return 0.7
	default:
		return 1.0
	}
}

// baseUtilization returns the fraction of that day's TotalLicenses in use,
// before weekday/holiday/jitter adjustments -- trend-plus-noise per the
// task brief, not a straight line and not a random walk: each story is a
// slow underlying trend (linear ramp, constant, or a seasonal cycle), with
// jitter layered on afterward by the caller.
func baseUtilization(spec tenantSpec, dayIndex, totalDays int, day time.Time) float64 {
	progress := 0.0
	if totalDays > 1 {
		progress = float64(dayIndex) / float64(totalDays-1)
	}
	switch spec.story {
	case storyGrowth:
		return lerp(0.55, 0.68, progress)
	case storyAmber:
		return 0.94
	case storyOversubscribed:
		return lerp(0.85, 1.08, progress)
	case storyFlat:
		return 0.40
	case storyRenewalWarning:
		return 0.60
	case storyTrial:
		return 0.35
	case storySeasonal:
		if day.Month() == time.July || day.Month() == time.August {
			return 0.25
		}
		return 0.75
	case storyFlexOnly:
		return 0.55
	case storyLateOnboard:
		return 0.50
	case storyOutage:
		return lerp(0.55, 0.65, progress)
	default:
		return 0.5
	}
}

func lerp(from, to, progress float64) float64 {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	return from + (to-from)*progress
}

// totalLicensesOnDay returns spec's entitlement in effect on a given day --
// constant for every tenant except entitlementChangeTenant, which steps up
// exactly once, partway through the window (a seat purchase; see
// roster.go).
func totalLicensesOnDay(specIndex, dayIndex, totalDays int, spec tenantSpec) int {
	if specIndex != entitlementChangeTenant {
		return spec.totalLicenses
	}
	changeAt := totalDays * 2 / 5 // roughly two-fifths through the window
	if dayIndex < changeAt {
		return entitlementChangeBeforeTotal
	}
	return spec.totalLicenses
}

// licenseInfoPayload builds a /licenseinfo-shaped raw response for a
// successful day, matching the exact contract confirmed against
// internal/profileunity/testdata/licenseinfo_success.json: every value a
// string, Evaluation as "Yes"/"No" while every other boolean-shaped field
// is "true"/"false", SupportEnds as US M/D/YYYY.
func licenseInfoPayload(spec tenantSpec, totalLicenses, usedLicenses int, supportEnds time.Time, trialExpired bool) string {
	yesNo := func(b bool) string {
		if b {
			return "Yes"
		}
		return "No"
	}
	boolStr := func(b bool) string {
		if b {
			return "true"
		}
		return "false"
	}
	return fmt.Sprintf(`{ "WebMessageType": 2, "Type": "success", "Message": "", "MessageKey": null, "Tag": [ {
	"RegisteredTo": %q, "LicenseMode": %q, "LicenseProduct": %q,
	"SupportEnds": %q, "TotalLicenses": %q, "UsedLicenses": %q,
	"Evaluation": %q, "ConsoleVersion": %q,
	"IsTrialExpired": %q, "IsTrial": %q, "IsProUOnly": %q, "IsFlexOnly": %q
} ] }`,
		spec.registeredTo, spec.mode, spec.product,
		fmt.Sprintf("%d/%d/%d", supportEnds.Month(), supportEnds.Day(), supportEnds.Year()),
		fmt.Sprintf("%d", totalLicenses), fmt.Sprintf("%d", usedLicenses),
		yesNo(spec.mode == "Evaluation"), spec.consoleVersion,
		boolStr(trialExpired), boolStr(spec.isTrial), boolStr(false), boolStr(spec.isFlexOnly))
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

// dayPlan is everything generateHistory decides for one tenant-day before
// turning it into a snapshot.Snapshot (or skipping the day entirely).
type dayPlan struct {
	absent bool
	status snapshot.Status
}

// planDay decides whether tenantIndex's day at dayIndex (0-based from the
// window start) is absent, a failure, or a success -- the three distinct
// states the task brief requires never to be conflated.
func planDay(tenantIndex, dayIndex, totalDays int) dayPlan {
	spec := roster[tenantIndex]

	if spec.story == storyLateOnboard {
		onboardedDaysAgo := 42 // 6 weeks
		if dayIndex < totalDays-onboardedDaysAgo {
			return dayPlan{absent: true}
		}
	}

	if spec.story == storyOutage {
		outageStart := totalDays / 2
		if dayIndex >= outageStart && dayIndex < outageStart+len(outageStatuses) {
			return dayPlan{status: outageStatuses[dayIndex-outageStart]}
		}
	}

	for _, f := range scatteredFailures {
		if f.tenantIndex == tenantIndex && dayIndex == f.dayOffset {
			return dayPlan{status: f.status}
		}
	}

	return dayPlan{status: snapshot.StatusSuccess}
}

// buildSnapshot renders one tenant-day into a Snapshot, given the plan
// planDay produced and a seeded RNG for this day's usage jitter.
func buildSnapshot(tenantIndex, dayIndex, totalDays int, day time.Time, tenantID string, plan dayPlan, endDate time.Time, rng *rand.Rand) snapshot.Snapshot {
	spec := roster[tenantIndex]
	collectedAt := time.Date(day.Year(), day.Month(), day.Day(), 6, 0, 0, 0, time.UTC)

	s := snapshot.Snapshot{
		TenantID:       tenantID,
		CollectionDate: day.Format("2006-01-02"),
		CollectedAtUTC: collectedAt,
		Status:         plan.status,
	}

	if plan.status != snapshot.StatusSuccess {
		s.ErrorMessage = errorMessageFor(plan.status)
		s.RawPayload = rawPayloadForFailure(plan.status)
		return s
	}

	total := totalLicensesOnDay(tenantIndex, dayIndex, totalDays, spec)
	frac := baseUtilization(spec, dayIndex, totalDays, day)
	frac *= weekdayFactor(spec.mode, day.Weekday())
	frac *= holidayFactor(day)
	frac *= 1 + (rng.Float64()-0.5)*0.06 // +/-3% jitter
	if frac < 0 {
		frac = 0
	}
	used := int(frac * float64(total))
	if used < 0 {
		used = 0
	}

	trialExpired := false
	if spec.story == storyTrial {
		// The trial expires 30 days before --end-date, so the demo shows
		// both states within the 6-month window.
		expiresAt := totalDays - 30
		trialExpired = dayIndex >= expiresAt
	}

	supportEnds := endDate.AddDate(0, 0, spec.supportEndsOffsetDays)

	s.AuthPath = "unauthenticated"
	s.RegisteredTo = spec.registeredTo
	s.LicenseMode = spec.mode
	s.LicenseProduct = spec.product
	s.TotalLicenses = intPtr(total)
	s.UsedLicenses = intPtr(used)
	s.Evaluation = boolPtr(spec.mode == "Evaluation")
	s.ConsoleVersion = spec.consoleVersion
	s.IsTrialExpired = boolPtr(trialExpired)
	s.IsTrial = boolPtr(spec.isTrial)
	s.IsProUOnly = boolPtr(false)
	s.IsFlexOnly = boolPtr(spec.isFlexOnly)
	s.SupportEndsISO = supportEnds.Format("2006-01-02")
	s.RawPayload = licenseInfoPayload(spec, total, used, supportEnds, trialExpired)
	return s
}
