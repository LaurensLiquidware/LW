// Package licenseserver is a minimal client for the ProfileUnity License
// Server admin API (build 6.9.5.x -- see LICENSE_PUSH_INTEGRATION_SPEC.md
// for the full contract this was derived from).
//
// IMPORTANT operational notes derived from the server implementation:
//   - POST /api/addlicense is a REPLACE: the server deletes every existing
//     license and purges their users before inserting the new one. There
//     is one active license per server -- this Console models one License
//     Server per tenant, and every push is a destructive operation.
//   - The License payload must be a genuine Liquidware-signed blob. The
//     server RSA-verifies <signature> against an embedded public
//     certificate; tampered or synthesized licenses are rejected. This
//     client relays signed codes, it never creates or edits one.
//   - The server has no separate API identity store -- Basic-Auth
//     credentials here are the username/password embedded in that
//     server's own MongoDB connection string. Do not rely on the
//     server's fail-open behavior on blank credentials: this client
//     refuses to send an empty username/password.
package licenseserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a single tenant's License Server.
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithInsecureSkipVerify disables TLS certificate verification for this
// client. TLS verification is on by default; this must only be set from
// a per-tenant, explicitly-chosen, persisted override -- never a blanket
// default (matches internal/profileunity's WithInsecureSkipVerify).
func WithInsecureSkipVerify(skip bool) Option {
	return func(c *Client) {
		transport := c.httpClient.Transport.(*http.Transport)
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = skip
	}
}

// WithTimeout sets the per-request timeout. Defaults to 30s.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// New creates a Client for the License Server at baseURL, e.g.
// "https://ld-lw01.example.com". username/password are that server's own
// Mongo-connection-string credential.
func New(baseURL, username, password string, opts ...Option) (*Client, error) {
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("licenseserver: invalid base URL %q: %w", baseURL, err)
	}
	c := &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{}},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// addLicenseRequest mirrors the server's LicensingProxy model: a single field.
type addLicenseRequest struct {
	License string `json:"License"`
}

// AddLicenseResponse mirrors the server's AddLicenseResponse. On failure the
// server returns ErrorMsg populated and License nil.
type AddLicenseResponse struct {
	License  *LicenseInfo `json:"License"`
	ErrorMsg *string      `json:"ErrorMsg"`
}

// LicenseInfo is the parsed license the server echoes back on success.
type LicenseInfo struct {
	Id           string `json:"Id"`
	Organization string `json:"Organization"`
	ContactName  string `json:"ContactName"`
	ContactEmail string `json:"ContactEmail"`
	ValidUntil   string `json:"ValidUntil"`
	LicenseType  string `json:"LicenseType"`
	MaxUsers     int    `json:"MaxUsers"`
	ProductType  string `json:"ProductType"`
	IsMachine    bool   `json:"IsMachine"`
	IsConcurrent bool   `json:"IsConcurrent"`
	Signature    string `json:"Signature"`
	RawLicense   string `json:"RawLicense"`
	Mode         string `json:"Mode"`
}

// UnmarshalJSON decodes through flexString for every text field (see its
// doc comment) so a server sending a JSON number/boolean where the spec
// promised a string doesn't fail the whole decode -- the exported fields
// stay plain strings for every caller.
func (l *LicenseInfo) UnmarshalJSON(data []byte) error {
	var w struct {
		Id           flexString `json:"Id"`
		Organization flexString `json:"Organization"`
		ContactName  flexString `json:"ContactName"`
		ContactEmail flexString `json:"ContactEmail"`
		ValidUntil   flexString `json:"ValidUntil"`
		LicenseType  flexString `json:"LicenseType"`
		MaxUsers     int        `json:"MaxUsers"`
		ProductType  flexString `json:"ProductType"`
		IsMachine    bool       `json:"IsMachine"`
		IsConcurrent bool       `json:"IsConcurrent"`
		Signature    flexString `json:"Signature"`
		RawLicense   flexString `json:"RawLicense"`
		Mode         flexString `json:"Mode"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*l = LicenseInfo{
		Id:           w.Id.String(),
		Organization: w.Organization.String(),
		ContactName:  w.ContactName.String(),
		ContactEmail: w.ContactEmail.String(),
		ValidUntil:   w.ValidUntil.String(),
		LicenseType:  w.LicenseType.String(),
		MaxUsers:     w.MaxUsers,
		ProductType:  w.ProductType.String(),
		IsMachine:    w.IsMachine,
		IsConcurrent: w.IsConcurrent,
		Signature:    w.Signature.String(),
		RawLicense:   w.RawLicense.String(),
		Mode:         w.Mode.String(),
	}
	return nil
}

func (c *Client) basicAuth() string {
	raw := c.username + ":" + c.password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// AddLicense pushes a license to the target License Server.
//
// signedLicenseXML is the raw, Liquidware-signed license XML (the
// <license>...</license> document). It is base64-encoded here to form the
// wire payload. If you already hold the base64 form, use
// AddLicenseEncoded instead.
func (c *Client) AddLicense(ctx context.Context, signedLicenseXML []byte) (*AddLicenseResponse, error) {
	return c.AddLicenseEncoded(ctx, base64.StdEncoding.EncodeToString(signedLicenseXML))
}

// AddLicenseEncoded pushes an already-base64-encoded license code. This is
// a destructive replace on the target server -- see the package doc.
func (c *Client) AddLicenseEncoded(ctx context.Context, base64License string) (*AddLicenseResponse, error) {
	if c.username == "" || c.password == "" {
		return nil, fmt.Errorf("licenseserver: refusing to send request with empty credentials -- " +
			"resolve the target server's API credential (Mongo connection-string user:pass) first")
	}

	body, err := json.Marshal(addLicenseRequest{License: base64License})
	if err != nil {
		return nil, fmt.Errorf("licenseserver: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/addlicense", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("licenseserver: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.basicAuth())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("licenseserver: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("licenseserver: 401 Unauthorized -- the API credential (Mongo user:pass) " +
			"was rejected or is not configured on this server")
	}

	var out AddLicenseResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("licenseserver: decode response (HTTP %d): %w", resp.StatusCode, err)
	}

	// The server returns 200 even on logical failure, carrying ErrorMsg.
	if out.ErrorMsg != nil && *out.ErrorMsg != "" {
		return &out, fmt.Errorf("licenseserver: license server rejected license: %s", *out.ErrorMsg)
	}
	return &out, nil
}

// Checkup calls the unauthenticated health endpoint. Useful for the
// Console to confirm reachability and Mongo connectivity before
// attempting a push.
func (c *Client) Checkup(ctx context.Context) (bool, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/checkup", nil)
	if err != nil {
		return false, "", fmt.Errorf("licenseserver: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("licenseserver: send request: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		IsWorking bool   `json:"IsWorking"`
		Message   string `json:"Message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, "", fmt.Errorf("licenseserver: decode response (HTTP %d): %w", resp.StatusCode, err)
	}
	return out.IsWorking, out.Message, nil
}

// newAuthedRequest builds a request with Basic auth, refusing to proceed
// with empty credentials (do not rely on the server's fail-open auth).
func (c *Client) newAuthedRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if c.username == "" || c.password == "" {
		return nil, fmt.Errorf("licenseserver: refusing request with empty credentials -- " +
			"resolve the target server's API credential (Mongo connection-string user:pass) first")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("licenseserver: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", c.basicAuth())
	return req, nil
}

// LicenseInfoItem mirrors the server's GetLicenseInfoResponseItem. The
// endpoint returns a bare JSON array of these (not wrapped in an
// envelope).
type LicenseInfoItem struct {
	Organization  string `json:"Organization"`
	ContactName   string `json:"ContactName"`
	ContactEmail  string `json:"ContactEmail"`
	ContactNumber string `json:"ContactNumber"`
	ValidUntil    string `json:"ValidUntil"`
	LicenseType   string `json:"LicenseType"`
	MaxUsers      int    `json:"MaxUsers"`
	UsedLicenses  int    `json:"UsedLicenses"`
}

// UnmarshalJSON decodes through flexString for every text field, for the
// same reason as LicenseInfo.UnmarshalJSON.
func (l *LicenseInfoItem) UnmarshalJSON(data []byte) error {
	var w struct {
		Organization  flexString `json:"Organization"`
		ContactName   flexString `json:"ContactName"`
		ContactEmail  flexString `json:"ContactEmail"`
		ContactNumber flexString `json:"ContactNumber"`
		ValidUntil    flexString `json:"ValidUntil"`
		LicenseType   flexString `json:"LicenseType"`
		MaxUsers      int        `json:"MaxUsers"`
		UsedLicenses  int        `json:"UsedLicenses"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*l = LicenseInfoItem{
		Organization:  w.Organization.String(),
		ContactName:   w.ContactName.String(),
		ContactEmail:  w.ContactEmail.String(),
		ContactNumber: w.ContactNumber.String(),
		ValidUntil:    w.ValidUntil.String(),
		LicenseType:   w.LicenseType.String(),
		MaxUsers:      w.MaxUsers,
		UsedLicenses:  w.UsedLicenses,
	}
	return nil
}

// GetLicenseInfo returns the installed license(s) and current seat usage
// (MaxUsers vs UsedLicenses). Call it right after AddLicense to confirm
// the new license took effect and to read seat consumption.
func (c *Client) GetLicenseInfo(ctx context.Context) ([]LicenseInfoItem, error) {
	req, err := c.newAuthedRequest(ctx, http.MethodGet, "/api/getlicenseinfo", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("licenseserver: send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("licenseserver: 401 Unauthorized -- API credential rejected or not configured")
	}
	var out []LicenseInfoItem
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("licenseserver: decode response (HTTP %d): %w", resp.StatusCode, err)
	}
	return out, nil
}

// PushResult bundles an install with the post-install verification read-back.
type PushResult struct {
	Installed   *AddLicenseResponse
	LicenseInfo []LicenseInfoItem
}

// AddLicenseAndVerify installs a license (destructive replace) and then
// reads getlicenseinfo, so the caller sees the resulting installed state
// and seat usage in one call. If the install succeeds but the verify read
// fails, the install result is still returned alongside the error.
func (c *Client) AddLicenseAndVerify(ctx context.Context, base64License string) (*PushResult, error) {
	installed, err := c.AddLicenseEncoded(ctx, base64License)
	if err != nil {
		return &PushResult{Installed: installed}, err
	}
	info, infoErr := c.GetLicenseInfo(ctx)
	if infoErr != nil {
		return &PushResult{Installed: installed}, fmt.Errorf("licenseserver: license installed but verify read failed: %w", infoErr)
	}
	return &PushResult{Installed: installed, LicenseInfo: info}, nil
}
