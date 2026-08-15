package collector

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"profileunity-msp-console/internal/profileunity"
)

// ConnectivityOutcome is what actually happened on a "test connection"
// call — project brief §7.1: "Connectivity test on save reporting
// precisely what happened... Not a boolean."
type ConnectivityOutcome string

const (
	ConnUnauthenticatedSuccess ConnectivityOutcome = "unauthenticated_success"
	ConnAuthenticatedSuccess   ConnectivityOutcome = "authenticated_success"
	ConnTLSFailure             ConnectivityOutcome = "tls_failure"
	ConnTimeout                ConnectivityOutcome = "timeout"
	ConnAuthRejected           ConnectivityOutcome = "auth_rejected"
	ConnAuthRequired           ConnectivityOutcome = "auth_required"
	ConnMalformedResponse      ConnectivityOutcome = "malformed_response"
	ConnUnreachable            ConnectivityOutcome = "unreachable"
	ConnError                  ConnectivityOutcome = "error"
)

// TestConnectionParams mirrors the subset of a tenant's configuration
// that affects connectivity — no ID, since this runs both for a
// not-yet-saved tenant (creating) and an existing one (re-testing).
type TestConnectionParams struct {
	Hostname      string
	Port          int
	TLSSkipVerify bool
	Username      string
	Password      string
}

// TestConnection makes exactly one attempt (no retries — this backs an
// interactive "Test Connection" click, not the scheduler) and reports
// precisely what happened.
func TestConnection(ctx context.Context, p TestConnectionParams) (ConnectivityOutcome, string) {
	var opts []profileunity.Option
	if p.TLSSkipVerify {
		opts = append(opts, profileunity.WithInsecureSkipVerify(true))
	}
	if p.Username != "" && p.Password != "" {
		opts = append(opts, profileunity.WithCredentials(p.Username, p.Password))
	}

	client, err := profileunity.New(fmt.Sprintf("https://%s:%d", p.Hostname, p.Port), opts...)
	if err != nil {
		return ConnError, err.Error()
	}

	info, authPath, _, err := client.CollectLicenseInfo(ctx)
	if err == nil {
		if authPath == profileunity.AuthPathAuthenticated {
			return ConnAuthenticatedSuccess, "Connected and authenticated successfully." + licenseWarnings(info)
		}
		return ConnUnauthenticatedSuccess, "Connected successfully (no authentication was required)." + licenseWarnings(info)
	}

	var unreachable *profileunity.UnreachableError
	var timeout *profileunity.TimeoutError
	var tlsErr *profileunity.TLSError
	var authRejected *profileunity.AuthRejectedError
	var authRequired *profileunity.AuthRequiredError
	var malformed *profileunity.MalformedPayloadError

	switch {
	case errors.As(err, &unreachable):
		return ConnUnreachable, "Could not reach the console: " + err.Error()
	case errors.As(err, &timeout):
		return ConnTimeout, "The console did not respond in time: " + err.Error()
	case errors.As(err, &tlsErr):
		return ConnTLSFailure, "TLS certificate verification failed: " + err.Error()
	case errors.As(err, &authRejected):
		return ConnAuthRejected, "The console rejected the supplied credentials: " + err.Error()
	case errors.As(err, &authRequired):
		return ConnAuthRequired, "The console requires authentication and no credentials were supplied."
	case errors.As(err, &malformed):
		return ConnMalformedResponse, "The console did not return a valid response: " + err.Error()
	default:
		return ConnError, err.Error()
	}
}

// licenseWarnings flags license data that looks off in a console's
// /licenseinfo response -- data Test Connection already fetches but
// would otherwise discard. It never changes the outcome (still a
// *_success case): these are advisory, not connectivity failures.
func licenseWarnings(info profileunity.LicenseInfo) string {
	var warnings []string
	if !info.TotalLicenses.Valid || info.TotalLicenses.Value == 0 {
		warnings = append(warnings, "the console reported no licensed seats")
	}
	if info.LicenseProduct == "" {
		warnings = append(warnings, "the console did not report a license product")
	}
	if info.IsTrialExpired {
		warnings = append(warnings, "this console's trial has already expired")
	}
	if len(warnings) == 0 {
		return ""
	}
	return " Warning: " + strings.Join(warnings, "; ") + "."
}
