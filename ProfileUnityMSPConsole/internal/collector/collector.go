// Package collector runs one tenant's license collection: build a
// ProfileUnity client, call it, and turn whatever happened into a
// snapshot.Snapshot. It never returns a Go error for a failed poll — a
// failed poll is a first-class outcome (project brief §7.2), not an
// exception. CollectOne always returns a Snapshot describing exactly what
// happened, success or not.
package collector

import (
	"context"
	"errors"
	"fmt"
	"time"

	"profileunity-msp-console/internal/profileunity"
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

const (
	maxAttempts    = 3
	retryBaseDelay = 500 * time.Millisecond
)

// BuildBaseURL constructs a tenant's console URL. Per the project brief
// §3, the console is always addressed over HTTPS.
func BuildBaseURL(t tenant.Tenant) string {
	return fmt.Sprintf("https://%s:%d", t.Hostname, t.Port)
}

// CollectOne polls one tenant and returns the resulting snapshot. ctx
// should already carry the per-tenant deadline (project brief §7.2: "one
// dead tenant must never stall the run") — CollectOne retries transient
// failures within whatever budget ctx leaves, and never blocks past it.
func CollectOne(ctx context.Context, t tenant.Tenant, creds *tenant.Credentials, now time.Time, loc *time.Location) snapshot.Snapshot {
	s := snapshot.Snapshot{
		TenantID:       t.ID,
		CollectionDate: snapshot.CollectionDateFor(now, loc),
		CollectedAtUTC: now.UTC(),
	}

	var opts []profileunity.Option
	if t.TLSSkipVerify {
		opts = append(opts, profileunity.WithInsecureSkipVerify(true))
	}
	if creds != nil {
		opts = append(opts, profileunity.WithCredentials(creds.Username, creds.Password))
	}

	client, err := profileunity.New(BuildBaseURL(t), opts...)
	if err != nil {
		s.Status = snapshot.StatusError
		s.ErrorMessage = err.Error()
		return s
	}

	info, authPath, rawBody, err := collectWithRetry(ctx, client)
	s.RawPayload = string(rawBody)
	if err != nil {
		s.Status, s.ErrorMessage = classify(err)
		return s
	}

	s.Status = snapshot.StatusSuccess
	s.AuthPath = string(authPath)
	s.RegisteredTo = info.RegisteredTo
	s.LicenseMode = info.LicenseMode
	s.LicenseProduct = info.LicenseProduct
	s.ConsoleVersion = info.ConsoleVersion.Raw
	s.SupportEndsISO = info.SupportEnds.ISO
	if info.TotalLicenses.Valid {
		v := info.TotalLicenses.Value
		s.TotalLicenses = &v
	}
	if info.UsedLicenses.Valid {
		v := info.UsedLicenses.Value
		s.UsedLicenses = &v
	}
	evaluation := info.Evaluation
	s.Evaluation = &evaluation
	isTrialExpired := info.IsTrialExpired
	s.IsTrialExpired = &isTrialExpired
	isTrial := info.IsTrial
	s.IsTrial = &isTrial
	isProUOnly := info.IsProUOnly
	s.IsProUOnly = &isProUOnly
	isFlexOnly := info.IsFlexOnly
	s.IsFlexOnly = &isFlexOnly

	return s
}

// collectWithRetry retries CollectLicenseInfo for transient failures
// (unreachable, timeout) only — retrying a rejected credential or a
// malformed response wastes the tenant's time budget on an outcome that
// will not change.
func collectWithRetry(ctx context.Context, client *profileunity.Client) (info profileunity.LicenseInfo, authPath profileunity.AuthPath, rawBody []byte, err error) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		info, authPath, rawBody, err = client.CollectLicenseInfo(ctx)
		if err == nil || attempt == maxAttempts || !isRetryable(err) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryBaseDelay * time.Duration(attempt)):
		}
	}
	return
}

func isRetryable(err error) bool {
	var unreachable *profileunity.UnreachableError
	var timeout *profileunity.TimeoutError
	return errors.As(err, &unreachable) || errors.As(err, &timeout)
}

// classify maps a profileunity error to a stored status and message. The
// message is the error's own text — never a tenant's RegisteredTo or any
// other field the project brief §9 calls out as PII, since none of these
// error types carry that field.
func classify(err error) (snapshot.Status, string) {
	var unreachable *profileunity.UnreachableError
	var timeout *profileunity.TimeoutError
	var tlsErr *profileunity.TLSError
	var authRejected *profileunity.AuthRejectedError
	var authRequired *profileunity.AuthRequiredError
	var malformed *profileunity.MalformedPayloadError

	switch {
	case errors.As(err, &unreachable):
		return snapshot.StatusUnreachable, err.Error()
	case errors.As(err, &timeout):
		return snapshot.StatusTimeout, err.Error()
	case errors.As(err, &tlsErr):
		return snapshot.StatusTLSError, err.Error()
	case errors.As(err, &authRejected):
		return snapshot.StatusAuthRejected, err.Error()
	case errors.As(err, &authRequired):
		return snapshot.StatusAuthRequired, err.Error()
	case errors.As(err, &malformed):
		return snapshot.StatusMalformed, err.Error()
	default:
		return snapshot.StatusError, err.Error()
	}
}
