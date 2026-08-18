// Package tenant manages registered ProfileUnity consoles (project brief
// §7.1). Credentials are encrypted at rest and never travel through the
// Tenant type itself — only Repo.GetCredentials, used solely by the
// collector, ever produces a plaintext password.
package tenant

import "time"

// Tenant is one registered customer console. It deliberately has no
// password field: per the project brief §9, stored credentials must never
// be returned once saved, even masked. HasPassword tells a caller whether
// one is configured without exposing it.
type Tenant struct {
	ID            string
	DisplayName   string
	Hostname      string
	Port          int
	Username      string
	HasPassword   bool
	TLSSkipVerify bool
	Enabled       bool
	Tags          []string
	Notes         string
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// LicenseServer* describe this tenant's ProfileUnity License Server
	// connection (a distinct host from the console above, authenticated
	// with that server's own Mongo-connection-string credential -- see
	// internal/licenseserver). Configured separately from the rest of
	// the tenant via Repo.UpdateLicenseServer, never through
	// Create/Update.
	LicenseServerHostname      string
	LicenseServerPort          int
	LicenseServerUsername      string
	LicenseServerHasPassword   bool
	LicenseServerTLSSkipVerify bool
}

// Credentials is the plaintext form, returned only to the collector.
type Credentials struct {
	Username string
	Password string
}

// CreateInput describes a new tenant registration. Username and Password
// must be both empty or both non-empty — a stored username with no
// password (or vice versa) cannot authenticate against anything.
type CreateInput struct {
	DisplayName   string
	Hostname      string
	Port          int
	Username      string
	Password      string
	TLSSkipVerify bool
	Enabled       bool
	Tags          []string
	Notes         string
}

// UpdateInput describes a tenant edit. Password uses pointer semantics:
// nil leaves the stored credential untouched, a pointer to "" clears it,
// and a pointer to a non-empty string replaces it. Username has no such
// three-way distinction because an empty username unambiguously means
// "no credentials" and always clears the password too.
type UpdateInput struct {
	DisplayName   string
	Hostname      string
	Port          int
	Username      string
	Password      *string
	TLSSkipVerify bool
	Enabled       bool
	Tags          []string
	Notes         string
}
