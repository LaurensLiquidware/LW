// Package config loads server configuration from the process environment.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	pumccrypto "profileunity-msp-console/internal/crypto"
)

// Config is the full set of settings the server needs to start.
type Config struct {
	// HTTPAddr is the address the HTTP server binds to. Defaults to
	// "0.0.0.0:8443" (all interfaces) so the console works out of the box;
	// set PUMC_HTTP_ADDR explicitly to bind to a specific interface/port.
	HTTPAddr string

	// Environment is a free-form deployment label ("development",
	// "production", ...). It sets LogLevel's default (see defaultLogLevel)
	// when PUMC_LOG_LEVEL isn't set explicitly.
	Environment string

	// DBDriver selects the storage backend: "sqlite" or "postgres".
	DBDriver string

	// DBDSN is the sqlite file path or the postgres connection string.
	DBDSN string

	// LogLevel is one of "debug", "info", "warn", "error". Defaults to
	// "debug" in development and "info" in any other Environment (see
	// defaultLogLevel) unless PUMC_LOG_LEVEL overrides it explicitly.
	LogLevel string

	// LogFile is where logs are written, in addition to stderr. Relative
	// to the process's working directory, same convention as DBDSN.
	LogFile string

	// CollectionInterval is how often the scheduler checks whether it's
	// time to collect. Snapshots are keyed by calendar day (in
	// CollectionTimezone) and upserted, so ticking more often than daily
	// is safe — it just keeps refreshing today's row.
	CollectionInterval time.Duration

	// CollectionTimezone names the IANA zone used to compute the
	// collection-day boundary (project brief §7.2). Stored values
	// themselves are always UTC (§11.2); only the day-boundary
	// calculation uses this zone.
	CollectionTimezone string
	CollectionLocation *time.Location

	// CollectionConcurrency caps how many tenants are polled at once.
	CollectionConcurrency int

	// CollectionTenantTimeout bounds a single tenant's poll (including
	// retries), so one dead tenant can never stall the run.
	CollectionTenantTimeout time.Duration

	// CredentialEncryptionKey encrypts stored tenant credentials at rest
	// (project brief §9). Nil here means "not supplied via
	// PUMC_CREDENTIAL_ENCRYPTION_KEY" -- cmd/server/main.go then resolves
	// the real key via internal/crypto.EnsureKey(CredentialEncryptionKeyFile),
	// auto-generating and persisting one on first boot rather than leaving
	// this nil at runtime. Tenants may still be registered without
	// credentials; storing a credential without a resolved key is an
	// error, not a silent plaintext write.
	CredentialEncryptionKey []byte

	// CredentialEncryptionKeyFile is where the auto-generated credential
	// encryption key is persisted when PUMC_CREDENTIAL_ENCRYPTION_KEY
	// isn't set explicitly. Relative to the working directory, same
	// convention as DBDSN/TLSCertFile. Losing or replacing this file
	// makes every previously stored tenant credential permanently
	// undecryptable.
	CredentialEncryptionKeyFile string

	// SessionIdleTimeout and SessionAbsoluteTimeout bound a console
	// operator's login session (project brief §9/§6's carried-over idle
	// timeout pattern) — idle resets on activity, absolute does not.
	SessionIdleTimeout     time.Duration
	SessionAbsoluteTimeout time.Duration

	// BootstrapAdminUsername/Password create the first operator account
	// at startup if the users table is empty. Leaving both unset defaults
	// to a built-in "LiquidwareMSP"/"LiquidwareMSP" account (see
	// defaultBootstrapAdmin) — change its password from the change-
	// password screen right after first login. Setting one but not the
	// other is still a config error (see validate).
	BootstrapAdminUsername string
	BootstrapAdminPassword string

	// TLSCertFile/TLSKeyFile are used as-is if both already exist;
	// otherwise a self-signed pair is generated there at first startup
	// (project brief §9). Supply your own CA-signed files here to skip
	// self-signing entirely.
	TLSCertFile string
	TLSKeyFile  string

	// SMTPHost/SMTPPort/SMTPUsername/SMTPPassword/SMTPFrom and
	// ReportRecipients configure the monthly automatic emailing of the
	// portfolio PDF report. SMTPHost empty means the feature is disabled
	// entirely -- nothing is ever sent, and the scheduler doesn't start.
	// Username/password may stay empty for a relay that doesn't require
	// auth; every other field here is required once SMTPHost is set.
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// SMTPSecurity selects how the SMTP connection is secured:
	// "starttls" (plain connection upgraded via STARTTLS, the common case
	// for port 587), "tls" (implicit TLS from the first byte, port 465),
	// or "none" (unencrypted, for a local relay only).
	SMTPSecurity string

	// ReportRecipients is who the monthly portfolio PDF is emailed to.
	ReportRecipients []string

	// ReportEmailDay is the day of the month (in CollectionTimezone) the
	// previous month's portfolio report is sent, e.g. 1 to send on the
	// 1st for the month that just ended.
	ReportEmailDay int
}

// ReportEmailEnabled reports whether the monthly report-email feature is
// configured at all -- SMTPHost is the single on/off switch.
func (c Config) ReportEmailEnabled() bool {
	return c.SMTPHost != ""
}

const (
	envHTTPAddr                = "PUMC_HTTP_ADDR"
	envEnvironment             = "PUMC_ENVIRONMENT"
	envDBDriver                = "PUMC_DB_DRIVER"
	envDBDSN                   = "PUMC_DB_DSN"
	envLogLevel                = "PUMC_LOG_LEVEL"
	envLogFile                 = "PUMC_LOG_FILE"
	envCollectionInterval      = "PUMC_COLLECTION_INTERVAL"
	envCollectionTimezone      = "PUMC_COLLECTION_TIMEZONE"
	envCollectionConcurrency   = "PUMC_COLLECTION_CONCURRENCY"
	envCollectionTenantTimeout = "PUMC_COLLECTION_TENANT_TIMEOUT"
	envCredentialEncryptionKey     = "PUMC_CREDENTIAL_ENCRYPTION_KEY"
	envCredentialEncryptionKeyFile = "PUMC_CREDENTIAL_ENCRYPTION_KEY_FILE"
	envSessionIdleTimeout      = "PUMC_SESSION_IDLE_TIMEOUT"
	envSessionAbsoluteTimeout  = "PUMC_SESSION_ABSOLUTE_TIMEOUT"
	envBootstrapAdminUsername  = "PUMC_BOOTSTRAP_ADMIN_USERNAME"
	envBootstrapAdminPassword  = "PUMC_BOOTSTRAP_ADMIN_PASSWORD"
	envTLSCertFile             = "PUMC_TLS_CERT_FILE"
	envTLSKeyFile              = "PUMC_TLS_KEY_FILE"
	envSMTPHost                = "PUMC_SMTP_HOST"
	envSMTPPort                = "PUMC_SMTP_PORT"
	envSMTPUsername            = "PUMC_SMTP_USERNAME"
	envSMTPPassword            = "PUMC_SMTP_PASSWORD"
	envSMTPFrom                = "PUMC_SMTP_FROM"
	envSMTPSecurity            = "PUMC_SMTP_SECURITY"
	envReportRecipients        = "PUMC_REPORT_RECIPIENTS"
	envReportEmailDay          = "PUMC_REPORT_EMAIL_DAY"
)

var validDBDrivers = map[string]bool{"sqlite": true, "postgres": true}
var validLogLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
var validSMTPSecurity = map[string]bool{"starttls": true, "tls": true, "none": true}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	environment := firstNonEmpty(os.Getenv(envEnvironment), "development")
	cfg := Config{
		HTTPAddr:           firstNonEmpty(os.Getenv(envHTTPAddr), DefaultHTTPAddr),
		Environment:        environment,
		DBDriver:           firstNonEmpty(os.Getenv(envDBDriver), "sqlite"),
		DBDSN:              firstNonEmpty(os.Getenv(envDBDSN), "./profileunity-msp-console.db"),
		LogLevel:           firstNonEmpty(os.Getenv(envLogLevel), defaultLogLevel(environment)),
		LogFile:            firstNonEmpty(os.Getenv(envLogFile), DefaultLogFile),
		CollectionTimezone: firstNonEmpty(os.Getenv(envCollectionTimezone), "UTC"),
	}

	interval, err := parseDurationDefault(os.Getenv(envCollectionInterval), time.Hour)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", envCollectionInterval, err)
	}
	cfg.CollectionInterval = interval

	tenantTimeout, err := parseDurationDefault(os.Getenv(envCollectionTenantTimeout), 30*time.Second)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", envCollectionTenantTimeout, err)
	}
	cfg.CollectionTenantTimeout = tenantTimeout

	concurrency, err := parseIntDefault(os.Getenv(envCollectionConcurrency), 5)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", envCollectionConcurrency, err)
	}
	cfg.CollectionConcurrency = concurrency

	loc, err := time.LoadLocation(cfg.CollectionTimezone)
	if err != nil {
		return Config{}, fmt.Errorf("%s: unknown IANA timezone %q: %w", envCollectionTimezone, cfg.CollectionTimezone, err)
	}
	cfg.CollectionLocation = loc

	sessionIdle, err := parseDurationDefault(os.Getenv(envSessionIdleTimeout), 30*time.Minute)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", envSessionIdleTimeout, err)
	}
	cfg.SessionIdleTimeout = sessionIdle

	sessionAbsolute, err := parseDurationDefault(os.Getenv(envSessionAbsoluteTimeout), 12*time.Hour)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", envSessionAbsoluteTimeout, err)
	}
	cfg.SessionAbsoluteTimeout = sessionAbsolute

	cfg.BootstrapAdminUsername, cfg.BootstrapAdminPassword = defaultBootstrapAdmin(
		strings.TrimSpace(os.Getenv(envBootstrapAdminUsername)),
		os.Getenv(envBootstrapAdminPassword),
	)
	cfg.TLSCertFile = firstNonEmpty(os.Getenv(envTLSCertFile), "./tls-cert.pem")
	cfg.TLSKeyFile = firstNonEmpty(os.Getenv(envTLSKeyFile), "./tls-key.pem")

	cfg.SMTPHost = strings.TrimSpace(os.Getenv(envSMTPHost))
	cfg.SMTPUsername = strings.TrimSpace(os.Getenv(envSMTPUsername))
	cfg.SMTPPassword = os.Getenv(envSMTPPassword)
	cfg.SMTPFrom = strings.TrimSpace(os.Getenv(envSMTPFrom))
	cfg.SMTPSecurity = firstNonEmpty(os.Getenv(envSMTPSecurity), "starttls")
	cfg.ReportRecipients = splitAndTrim(os.Getenv(envReportRecipients))

	smtpPort, err := parseIntDefault(os.Getenv(envSMTPPort), 587)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", envSMTPPort, err)
	}
	cfg.SMTPPort = smtpPort

	reportEmailDay, err := parseIntDefault(os.Getenv(envReportEmailDay), 1)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", envReportEmailDay, err)
	}
	cfg.ReportEmailDay = reportEmailDay

	if raw := strings.TrimSpace(os.Getenv(envCredentialEncryptionKey)); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: not valid base64: %w", envCredentialEncryptionKey, err)
		}
		if len(key) != pumccrypto.KeySize {
			return Config{}, fmt.Errorf("%s: decoded key must be %d bytes, got %d", envCredentialEncryptionKey, pumccrypto.KeySize, len(key))
		}
		cfg.CredentialEncryptionKey = key
	}
	cfg.CredentialEncryptionKeyFile = firstNonEmpty(os.Getenv(envCredentialEncryptionKeyFile), "./credential-encryption.key")

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	// Load() always defaults HTTPAddr to "0.0.0.0:8443", so this never
	// triggers via the normal path -- kept as a defensive backstop for
	// any caller constructing a Config directly.
	if c.HTTPAddr == "" {
		return fmt.Errorf("%s must not be empty", envHTTPAddr)
	}
	if !validDBDrivers[c.DBDriver] {
		return fmt.Errorf("%s must be one of sqlite, postgres (got %q)", envDBDriver, c.DBDriver)
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("%s must be one of debug, info, warn, error (got %q)", envLogLevel, c.LogLevel)
	}
	if c.CollectionConcurrency < 1 {
		return fmt.Errorf("%s must be at least 1 (got %d)", envCollectionConcurrency, c.CollectionConcurrency)
	}
	if c.CollectionInterval <= 0 {
		return fmt.Errorf("%s must be positive (got %s)", envCollectionInterval, c.CollectionInterval)
	}
	if c.CollectionTenantTimeout <= 0 {
		return fmt.Errorf("%s must be positive (got %s)", envCollectionTenantTimeout, c.CollectionTenantTimeout)
	}
	if c.SessionIdleTimeout <= 0 {
		return fmt.Errorf("%s must be positive (got %s)", envSessionIdleTimeout, c.SessionIdleTimeout)
	}
	if c.SessionAbsoluteTimeout <= 0 {
		return fmt.Errorf("%s must be positive (got %s)", envSessionAbsoluteTimeout, c.SessionAbsoluteTimeout)
	}
	if (c.BootstrapAdminUsername == "") != (c.BootstrapAdminPassword == "") {
		return fmt.Errorf("%s and %s must be both set or both empty", envBootstrapAdminUsername, envBootstrapAdminPassword)
	}
	if !validSMTPSecurity[c.SMTPSecurity] {
		return fmt.Errorf("%s must be one of starttls, tls, none (got %q)", envSMTPSecurity, c.SMTPSecurity)
	}
	if c.ReportEmailEnabled() {
		if c.SMTPFrom == "" {
			return fmt.Errorf("%s is required once %s is set", envSMTPFrom, envSMTPHost)
		}
		if len(c.ReportRecipients) == 0 {
			return fmt.Errorf("%s is required once %s is set", envReportRecipients, envSMTPHost)
		}
		if c.SMTPPort <= 0 {
			return fmt.Errorf("%s must be positive (got %d)", envSMTPPort, c.SMTPPort)
		}
		if c.ReportEmailDay < 1 || c.ReportEmailDay > 28 {
			return fmt.Errorf("%s must be between 1 and 28 (got %d) -- capped at 28 so it exists in every month", envReportEmailDay, c.ReportEmailDay)
		}
	} else if c.SMTPFrom != "" || len(c.ReportRecipients) > 0 {
		return fmt.Errorf("%s is required when %s or %s is set", envSMTPHost, envSMTPFrom, envReportRecipients)
	}
	return nil
}

// splitAndTrim splits a comma-separated list, trims whitespace from each
// item, and drops empty items -- e.g. "a@x.com, , b@x.com" -> [a@x.com
// b@x.com]. Returns nil (not an empty non-nil slice) for a blank input, so
// len(cfg.ReportRecipients) == 0 is the reliable "unset" check.
func splitAndTrim(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// DefaultBootstrapAdminUsername/Password are the built-in operator
// account created at first startup when no PUMC_BOOTSTRAP_ADMIN_* env
// vars are set, so the console is usable out of the box with zero
// configuration. Change the password from the change-password screen
// right after first login -- this is a fixed, publicly-known default,
// not a secret.
const (
	DefaultBootstrapAdminUsername = "LiquidwareMSP"
	DefaultBootstrapAdminPassword = "LiquidwareMSP"
)

// DefaultLogFile is LogFile's default when PUMC_LOG_FILE is unset.
// Exported so other entry points (e.g. cmd/tray, which tails this same
// file for its "Show Log" viewer without importing the full config
// loader) can resolve the same default without duplicating the literal.
const DefaultLogFile = "./profileunity-msp-console.log"

// DefaultHTTPAddr is HTTPAddr's default when PUMC_HTTP_ADDR is unset.
// Exported for the same reason as DefaultLogFile -- cmd/tray resolves
// the console's own URL for its clickable link without duplicating
// this literal.
const DefaultHTTPAddr = "0.0.0.0:8443"

// defaultBootstrapAdmin applies the built-in default only when *both*
// raw env values are empty, so setting exactly one still falls through
// to validate()'s "must be both set or both empty" check rather than
// silently pairing an operator-supplied value with the built-in default
// for the other field.
func defaultBootstrapAdmin(rawUsername, rawPassword string) (username, password string) {
	if rawUsername == "" && rawPassword == "" {
		return DefaultBootstrapAdminUsername, DefaultBootstrapAdminPassword
	}
	return rawUsername, rawPassword
}

// defaultLogLevel is the LogLevel default when PUMC_LOG_LEVEL is unset:
// verbose in development, quieter in every other environment. Still
// overridable explicitly via PUMC_LOG_LEVEL in either case.
func defaultLogLevel(environment string) string {
	if environment == "development" {
		return "debug"
	}
	return "info"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseDurationDefault(raw string, def time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	return time.ParseDuration(raw)
}

func parseIntDefault(raw string, def int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}
