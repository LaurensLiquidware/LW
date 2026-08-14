package profileunity

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// Only these four endpoints are ever called. This is a deliberate
// whitelist, not an oversight — the client exposes one method per
// endpoint and no generic "GET arbitrary path" escape hatch, mirroring
// the reference project's proxy pattern. §3.7 lists endpoints that must
// never be used (e.g. /api/audit); they are simply absent here.
const (
	pathLicenseInfo     = "/licenseinfo"
	pathAuthenticate    = "/authenticate"
	pathServerLicensing = "/api/server/licensing"
	pathLicenseServer   = "/api/licenseserver"
)

// AuthPath records which of the two paths a collection actually used, per
// §4: "design for both: attempt unauthenticated, fall back to an
// authenticated session, record which path each collection used."
type AuthPath string

const (
	AuthPathUnauthenticated AuthPath = "unauthenticated"
	AuthPathAuthenticated   AuthPath = "authenticated"
)

// Client talks to a single ProfileUnity console. It never issues a
// POST/PUT/DELETE against tenant data — only GET, plus POST /authenticate
// (project brief §9: "Never write to a ProfileUnity console").
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client

	authenticated bool
}

// Option configures a Client.
type Option func(*Client)

// WithCredentials configures the client to fall back to an authenticated
// session when an unauthenticated call needs one, and enables the
// §3.3/§3.4 enrichment calls. Omit it for tenants where no credentials
// were supplied — per §3.7, authenticated polling writes audit-trail
// entries in the customer's own environment, so it is used only when the
// extra fields are genuinely wanted or unauthenticated access stops working.
func WithCredentials(username, password string) Option {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

// WithInsecureSkipVerify disables TLS certificate verification for this
// client. TLS verification is on by default (project brief §9); this must
// only be set from a per-tenant, explicitly-chosen, persisted override —
// never as a blanket default.
func WithInsecureSkipVerify(skip bool) Option {
	return func(c *Client) {
		transport := c.httpClient.Transport.(*http.Transport)
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = skip
	}
}

// WithTimeout sets the per-request timeout. Defaults to 15s.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// New creates a Client for the console at baseURL, e.g.
// "https://console.example.com:8000".
func New(baseURL string, opts ...Option) (*Client, error) {
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("profileunity: invalid base URL %q: %w", baseURL, err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("profileunity: create cookie jar: %w", err)
	}

	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Jar:       jar,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{}},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// GetLicenseInfoUnauthenticated calls GET /licenseinfo (§3.2), which
// requires no authentication on 6.9.5.9678. Whether that stays true on a
// future console version is an open question (§4); if the console starts
// requiring a session, this call returns an *AuthRequiredError.
//
// It returns the exact response body alongside the parsed result — the
// collector retains it per §7.2 ("store the raw JSON payload alongside
// parsed fields"), so a future field-mapping change never has to work
// from parsed-and-possibly-wrong history.
func (c *Client) GetLicenseInfoUnauthenticated(ctx context.Context) (LicenseInfo, []byte, error) {
	rawBody, tag, err := c.doGET(ctx, pathLicenseInfo)
	if err != nil {
		return LicenseInfo{}, rawBody, err
	}

	var rows []licenseInfoRaw
	if err := json.Unmarshal(tag, &rows); err != nil {
		return LicenseInfo{}, rawBody, &MalformedPayloadError{Body: tag, Cause: err}
	}
	if len(rows) == 0 {
		return LicenseInfo{}, rawBody, &MalformedPayloadError{Body: tag, Cause: fmt.Errorf("licenseinfo Tag array is empty")}
	}
	return normalizeLicenseInfo(rows[0]), rawBody, nil
}

// CollectLicenseInfo is the primary collection entry point: it attempts
// the unauthenticated call first, and only falls back to authenticating
// if the console demands a session and credentials were configured (§4).
// The returned AuthPath records which one actually produced the result,
// and rawBody is the exact response body that produced info, both for
// the collector to store alongside the snapshot (§7.2).
func (c *Client) CollectLicenseInfo(ctx context.Context) (info LicenseInfo, authPath AuthPath, rawBody []byte, err error) {
	info, rawBody, err = c.GetLicenseInfoUnauthenticated(ctx)
	if err == nil {
		return info, AuthPathUnauthenticated, rawBody, nil
	}

	var authRequired *AuthRequiredError
	if !errors.As(err, &authRequired) {
		return LicenseInfo{}, "", rawBody, err
	}
	if c.username == "" {
		return LicenseInfo{}, "", rawBody, err
	}

	if err := c.EnsureAuthenticated(ctx); err != nil {
		return LicenseInfo{}, "", nil, err
	}
	info, rawBody, err = c.GetLicenseInfoUnauthenticated(ctx)
	if err != nil {
		return LicenseInfo{}, "", rawBody, err
	}
	return info, AuthPathAuthenticated, rawBody, nil
}

// Authenticate calls POST /authenticate (§3.6) and, on success, retains
// the session cookie for subsequent calls on this Client. Success is
// Type == "success" in the body, never the HTTP status.
//
// The console also returns an X-CSRF-TOKEN header on success, required
// for mutating calls — this client deliberately never makes one (project
// brief §9: GET only, except this endpoint), so it is not captured.
func (c *Client) Authenticate(ctx context.Context) error {
	if c.username == "" {
		return fmt.Errorf("profileunity: no credentials configured")
	}

	form := url.Values{}
	form.Set("username", c.username)
	form.Set("password", c.password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+pathAuthenticate, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("profileunity: build authenticate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return classifyTransportError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return classifyTransportError(err)
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return &MalformedPayloadError{Body: body, Cause: err}
	}
	if env.Type != "success" {
		return &AuthRejectedError{Message: env.Message}
	}

	c.authenticated = true
	return nil
}

// EnsureAuthenticated authenticates only if this Client has not already
// done so successfully.
func (c *Client) EnsureAuthenticated(ctx context.Context) error {
	if c.authenticated {
		return nil
	}
	return c.Authenticate(ctx)
}

// GetServerLicensing calls GET /api/server/licensing (§3.3): optional
// enrichment (Organization, ContactName, ContactEmail, ContactNumber)
// beyond what /licenseinfo provides. Requires an authenticated session;
// call EnsureAuthenticated first, or expect *AuthRequiredError.
func (c *Client) GetServerLicensing(ctx context.Context) (ServerLicensing, error) {
	_, tag, err := c.doGET(ctx, pathServerLicensing)
	if err != nil {
		return ServerLicensing{}, err
	}
	var raw serverLicensingRaw
	if err := json.Unmarshal(tag, &raw); err != nil {
		return ServerLicensing{}, &MalformedPayloadError{Body: tag, Cause: err}
	}
	return normalizeServerLicensing(raw), nil
}

// GetLicenseServers calls GET /api/licenseserver (§3.4): license service
// health, keyed on the LastKnownRunningUTC heartbeat. Requires an
// authenticated session.
func (c *Client) GetLicenseServers(ctx context.Context) ([]LicenseServer, error) {
	_, tag, err := c.doGET(ctx, pathLicenseServer)
	if err != nil {
		return nil, err
	}
	var rows rowsTag[licenseServerRowRaw]
	if err := json.Unmarshal(tag, &rows); err != nil {
		return nil, &MalformedPayloadError{Body: tag, Cause: err}
	}
	result := make([]LicenseServer, 0, len(rows.Rows))
	for _, row := range rows.Rows {
		result = append(result, normalizeLicenseServer(row))
	}
	return result, nil
}

// doGET performs a GET against one of the whitelisted paths and returns
// both the raw response body and the envelope's Tag, having already
// checked Type == "success" — never the HTTP status (§3.1) — except for
// HTTP 401, which the API contract (§3.3/§3.4) uses as a real, meaningful
// status for "no session".
func (c *Client) doGET(ctx context.Context, path string) (rawBody []byte, tag json.RawMessage, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("profileunity: build request for %s: %w", path, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, classifyTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, nil, &AuthRequiredError{}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, classifyTransportError(err)
	}

	tag, err = decodeEnvelope(body)
	return body, tag, err
}

// classifyTransportError turns a transport-level failure (network,
// timeout, TLS) into one of this package's typed errors, so a caller can
// distinguish "unreachable" from "TLS failure" from "timeout" without
// string-matching. A failed poll must be recorded as unknown, never as a
// zero — the collector needs this distinction to do that (project brief §2).
func classifyTransportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &TimeoutError{Cause: err}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &TimeoutError{Cause: err}
	}

	var certVerifyErr *tls.CertificateVerificationError
	if errors.As(err, &certVerifyErr) {
		return &TLSError{Cause: err}
	}
	var unknownAuthErr x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthErr) {
		return &TLSError{Cause: err}
	}
	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return &TLSError{Cause: err}
	}
	var certInvalidErr x509.CertificateInvalidError
	if errors.As(err, &certInvalidErr) {
		return &TLSError{Cause: err}
	}

	return &UnreachableError{Cause: err}
}
