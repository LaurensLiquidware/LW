// Package licensepush records and drives pushes of a signed license to a
// tenant's ProfileUnity License Server (see internal/licenseserver). The
// License Server itself keeps no history of what was installed before a
// push replaced it — this package IS that history.
package licensepush

import "time"

// Outcome is the result of one push attempt.
type Outcome string

const (
	OutcomeSuccess     Outcome = "success"
	OutcomeAuthFailed  Outcome = "auth_failed"
	OutcomeRejected    Outcome = "rejected"
	OutcomeUnreachable Outcome = "unreachable"
	OutcomeError       Outcome = "error"
)

// Record is one row of push history: what was pushed, by whom, when, and
// what happened. Stores the full base64 license code that was pushed so
// a past push can be redone (e.g. after a server rebuild) without having
// to re-locate the original license file.
type Record struct {
	ID                string
	TenantID          string
	PushedAtUTC       time.Time
	OperatorUsername  string
	Outcome           Outcome
	ErrorMessage      string
	LicenseCodeBase64 string

	Organization string
	ContactName  string
	ContactEmail string
	ValidUntil   string
	LicenseType  string
	MaxUsers     *int
	IsMachine    bool
	IsConcurrent bool
}
