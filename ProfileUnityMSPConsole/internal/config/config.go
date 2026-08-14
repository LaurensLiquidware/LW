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
	// HTTPAddr is the address the HTTP server binds to, e.g. "0.0.0.0:8443".
	// There is no default — see README.md "Configuration" for why.
	HTTPAddr string

	// Environment is a free-form deployment label ("development",
	// "production", ...). It currently only affects logging verbosity.
	Environment string

	// DBDriver selects the storage backend: "sqlite" or "postgres".
	DBDriver string

	// DBDSN is the sqlite file path or the postgres connection string.
	DBDSN string

	// LogLevel is one of "debug", "info", "warn", "error".
	LogLevel string

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
	// (project brief §9). Nil if unset — tenants may still be registered
	// without credentials; storing a credential without this key set is
	// an error, not a silent plaintext write.
	CredentialEncryptionKey []byte
}

const (
	envHTTPAddr                = "PUMC_HTTP_ADDR"
	envEnvironment             = "PUMC_ENVIRONMENT"
	envDBDriver                = "PUMC_DB_DRIVER"
	envDBDSN                   = "PUMC_DB_DSN"
	envLogLevel                = "PUMC_LOG_LEVEL"
	envCollectionInterval      = "PUMC_COLLECTION_INTERVAL"
	envCollectionTimezone      = "PUMC_COLLECTION_TIMEZONE"
	envCollectionConcurrency   = "PUMC_COLLECTION_CONCURRENCY"
	envCollectionTenantTimeout = "PUMC_COLLECTION_TENANT_TIMEOUT"
	envCredentialEncryptionKey = "PUMC_CREDENTIAL_ENCRYPTION_KEY"
)

var validDBDrivers = map[string]bool{"sqlite": true, "postgres": true}
var validLogLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}

// Load reads configuration from environment variables. It intentionally
// requires PUMC_HTTP_ADDR explicitly rather than defaulting to a localhost
// address: this is a continuously running, multi-user server.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:           strings.TrimSpace(os.Getenv(envHTTPAddr)),
		Environment:        firstNonEmpty(os.Getenv(envEnvironment), "development"),
		DBDriver:           firstNonEmpty(os.Getenv(envDBDriver), "sqlite"),
		DBDSN:              firstNonEmpty(os.Getenv(envDBDSN), "./profileunity-msp-console.db"),
		LogLevel:           firstNonEmpty(os.Getenv(envLogLevel), "info"),
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

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.HTTPAddr == "" {
		return fmt.Errorf("%s is required (no default listen address is provided by design; see README.md)", envHTTPAddr)
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
	return nil
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
