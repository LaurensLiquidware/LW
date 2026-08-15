// Package snapshot stores one license-usage record per tenant per
// collection day (project brief §7.2). ProfileUnity keeps no history of
// its own — this package IS the time series.
package snapshot

import "time"

// Status is the outcome of a single collection attempt. Only Success has
// meaningful license figures attached — every other status means the
// figures are absent and must be rendered as unknown, never as a zero
// (project brief §2, the single most important correctness rule here).
type Status string

const (
	StatusSuccess      Status = "success"
	StatusUnreachable  Status = "unreachable"
	StatusTimeout      Status = "timeout"
	StatusTLSError     Status = "tls_error"
	StatusAuthRejected Status = "auth_rejected"
	StatusAuthRequired Status = "auth_required"
	StatusMalformed    Status = "malformed"
	// StatusError is a catch-all for a transport/response failure that
	// does not fit any of the more specific statuses above.
	StatusError Status = "error"
)

// Snapshot is one row: a tenant's license state on one collection day, or
// the record of why that state could not be obtained.
type Snapshot struct {
	ID             string
	TenantID       string
	CollectionDate string // YYYY-MM-DD, in the configured collection timezone
	CollectedAtUTC time.Time
	Status         Status
	AuthPath       string // "unauthenticated" or "authenticated"; empty when Status != Success
	ErrorMessage   string
	RawPayload     string // exact response body, retained for future re-parsing

	RegisteredTo   string
	LicenseMode    string
	LicenseProduct string
	TotalLicenses  *int
	UsedLicenses   *int
	Evaluation     *bool
	ConsoleVersion string
	IsTrialExpired *bool
	IsTrial        *bool
	IsProUOnly     *bool
	IsFlexOnly     *bool
	SupportEndsISO string
}

// CollectionDateFor computes the collection-day key for instant t in loc.
// This is the only place "what day is it" is decided — the project brief
// §7.2 requires an explicit collection-day boundary in a configured
// timezone, computed once and used consistently, not re-derived ad hoc.
func CollectionDateFor(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("2006-01-02")
}
