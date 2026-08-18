package licensepush

import (
	"context"
	"errors"
	"net"
	"strings"

	"profileunity-msp-console/internal/licenseserver"
)

// Result is what Push returns: the classified outcome plus whatever
// license fields could be decoded locally, regardless of whether the
// push itself succeeded (a rejection is still worth recording with the
// license that was rejected).
type Result struct {
	Outcome Outcome
	Message string
	Fields  licenseserver.Fields
}

// Push decodes licenseBase64 locally (for the audit record and the
// caller's response), then pushes it via client -- a destructive replace
// on the target License Server (see internal/licenseserver's package
// doc). It never returns a Go error for an ordinary push failure
// (auth/rejection/unreachable) -- those are all valid Outcomes; err is
// reserved for "the license code itself could not even be decoded",
// which the caller should treat as a client-side validation failure
// distinct from a push attempt.
func Push(ctx context.Context, client *licenseserver.Client, licenseBase64 string) (Result, error) {
	fields, decodeErr := licenseserver.DecodeLicenseFields(licenseBase64)
	if decodeErr != nil {
		return Result{}, decodeErr
	}

	_, err := client.AddLicenseEncoded(ctx, licenseBase64)
	if err == nil {
		return Result{Outcome: OutcomeSuccess, Message: "License installed successfully.", Fields: fields}, nil
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "401 Unauthorized"):
		return Result{Outcome: OutcomeAuthFailed, Message: msg, Fields: fields}, nil
	case strings.Contains(msg, "license server rejected license"):
		return Result{Outcome: OutcomeRejected, Message: msg, Fields: fields}, nil
	case isUnreachable(err):
		return Result{Outcome: OutcomeUnreachable, Message: msg, Fields: fields}, nil
	default:
		return Result{Outcome: OutcomeError, Message: msg, Fields: fields}, nil
	}
}

func isUnreachable(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}
