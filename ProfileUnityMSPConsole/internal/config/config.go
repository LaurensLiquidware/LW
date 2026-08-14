// Package config loads server configuration from the process environment.
package config

import (
	"fmt"
	"os"
	"strings"
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
}

const (
	envHTTPAddr    = "PUMC_HTTP_ADDR"
	envEnvironment = "PUMC_ENVIRONMENT"
	envDBDriver    = "PUMC_DB_DRIVER"
	envDBDSN       = "PUMC_DB_DSN"
	envLogLevel    = "PUMC_LOG_LEVEL"
)

var validDBDrivers = map[string]bool{"sqlite": true, "postgres": true}
var validLogLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}

// Load reads configuration from environment variables. It intentionally
// requires PUMC_HTTP_ADDR explicitly rather than defaulting to a localhost
// address: this is a continuously running, multi-user server.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:    strings.TrimSpace(os.Getenv(envHTTPAddr)),
		Environment: firstNonEmpty(os.Getenv(envEnvironment), "development"),
		DBDriver:    firstNonEmpty(os.Getenv(envDBDriver), "sqlite"),
		DBDSN:       firstNonEmpty(os.Getenv(envDBDSN), "./profileunity-msp-console.db"),
		LogLevel:    firstNonEmpty(os.Getenv(envLogLevel), "info"),
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
