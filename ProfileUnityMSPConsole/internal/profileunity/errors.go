package profileunity

import "fmt"

// MalformedPayloadError means the response body was not the JSON envelope
// §3.1 describes — an HTML error page, truncated JSON, or anything else
// that fails to parse. Distinct from *APIError (a well-formed envelope
// that says Type != "success") and from transport-level failures like
// *UnreachableError.
type MalformedPayloadError struct {
	Body  []byte
	Cause error
}

func (e *MalformedPayloadError) Error() string {
	const maxSnippet = 200
	snippet := string(e.Body)
	if len(snippet) > maxSnippet {
		snippet = snippet[:maxSnippet] + "…"
	}
	return fmt.Sprintf("profileunity: response was not a valid JSON envelope: %v (body: %q)", e.Cause, snippet)
}

func (e *MalformedPayloadError) Unwrap() error { return e.Cause }

// UnreachableError means the console could not be reached at all —
// connection refused, DNS failure, or similar. Never treat this as "zero
// usage": per the project brief, a failed poll must be recorded as
// unknown, distinct from a genuine zero.
type UnreachableError struct {
	Cause error
}

func (e *UnreachableError) Error() string {
	return fmt.Sprintf("profileunity: console unreachable: %v", e.Cause)
}

func (e *UnreachableError) Unwrap() error { return e.Cause }

// TimeoutError means the request exceeded its deadline.
type TimeoutError struct {
	Cause error
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("profileunity: request timed out: %v", e.Cause)
}

func (e *TimeoutError) Unwrap() error { return e.Cause }

// TLSError means the TLS handshake failed — most commonly a self-signed
// or otherwise untrusted certificate. Per the project brief, TLS
// verification defaults to on; this is what surfaces when a tenant's
// console needs the verification override.
type TLSError struct {
	Cause error
}

func (e *TLSError) Error() string {
	return fmt.Sprintf("profileunity: TLS error: %v", e.Cause)
}

func (e *TLSError) Unwrap() error { return e.Cause }

// AuthRejectedError means /authenticate was called and the console
// rejected the credentials (Type != "success" on that specific call).
type AuthRejectedError struct {
	Message string
}

func (e *AuthRejectedError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("profileunity: authentication rejected: %s", e.Message)
	}
	return "profileunity: authentication rejected"
}

// AuthRequiredError means an authenticated endpoint was called without an
// authenticated session (HTTP 401, per §3.3/§3.4).
type AuthRequiredError struct{}

func (e *AuthRequiredError) Error() string {
	return "profileunity: authentication required (401)"
}
