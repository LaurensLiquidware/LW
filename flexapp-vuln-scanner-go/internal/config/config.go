// Package config loads server configuration from the process environment.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config is the full set of settings the server needs to start.
type Config struct {
	// HTTPAddr is the address the HTTP server binds to. Defaults to
	// "127.0.0.1:8743" (loopback only -- this is a local single-user tool,
	// not a multi-tenant service) unless FVS_HTTP_ADDR overrides it.
	HTTPAddr string

	// Environment is a free-form deployment label ("development",
	// "production", ...). It sets LogLevel's default (see defaultLogLevel)
	// when FVS_LOG_LEVEL isn't set explicitly.
	Environment string

	// LogLevel is one of "debug", "info", "warn", "error".
	LogLevel string

	// LogFile is where logs are written, in addition to stderr. Relative
	// to the process's working directory.
	LogFile string

	// DefaultOutputDir is the scan output folder pre-filled in the New Scan
	// screen when the user hasn't chosen one yet.
	DefaultOutputDir string

	// CacheDir holds the on-disk OSV/NVD response cache.
	CacheDir string

	// NVDAPIKey is the optional NVD 2.0 API key; unset means unauthenticated
	// (rate-limited) NVD requests.
	NVDAPIKey string

	// ScanHistoryFile is where the list of past scans is persisted (JSON),
	// so scan history survives restarting the app.
	ScanHistoryFile string

	// StageOneScript is the path to the PowerShell Stage 1 inventory
	// script this process shells out to.
	StageOneScript string

	// CPEMappingsPath is the manual vendor/product -> CPE override table
	// (see internal/cpemap) used during vulnerability matching and SBOM
	// building.
	CPEMappingsPath string

	// PickerAddr is the address the native file/folder picker (hosted by
	// cmd/tray on Windows, see cmd/tray/picker_windows.go) listens on.
	// The server only reports this to the frontend via GET /api/config --
	// it never starts the picker itself, since the picker needs a Windows
	// GUI thread to show dialogs from, which the headless server doesn't
	// have. Unreachable (not launched via tray, or on a non-Windows dev
	// machine) simply means the frontend's Browse buttons don't appear.
	PickerAddr string
}

const (
	envHTTPAddr         = "FVS_HTTP_ADDR"
	envEnvironment      = "FVS_ENVIRONMENT"
	envLogLevel         = "FVS_LOG_LEVEL"
	envLogFile          = "FVS_LOG_FILE"
	envDefaultOutputDir = "FVS_DEFAULT_OUTPUT_DIR"
	envCacheDir         = "FVS_CACHE_DIR"
	envNVDAPIKey        = "FVS_NVD_API_KEY"
	envScanHistoryFile  = "FVS_SCAN_HISTORY_FILE"
	envStageOneScript   = "FVS_STAGE1_SCRIPT"
	envCPEMappingsPath  = "FVS_CPE_MAPPINGS_PATH"
	envPickerAddr       = "FVS_PICKER_ADDR"
)

var validLogLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	environment := firstNonEmpty(os.Getenv(envEnvironment), "development")
	cfg := Config{
		HTTPAddr:         firstNonEmpty(os.Getenv(envHTTPAddr), DefaultHTTPAddr),
		Environment:      environment,
		LogLevel:         firstNonEmpty(os.Getenv(envLogLevel), defaultLogLevel(environment)),
		LogFile:          firstNonEmpty(os.Getenv(envLogFile), DefaultLogFile),
		DefaultOutputDir: firstNonEmpty(os.Getenv(envDefaultOutputDir), "./scan-out"),
		CacheDir:         firstNonEmpty(os.Getenv(envCacheDir), "./cache"),
		NVDAPIKey:        strings.TrimSpace(os.Getenv(envNVDAPIKey)),
		ScanHistoryFile:  firstNonEmpty(os.Getenv(envScanHistoryFile), "./scan-history.json"),
		StageOneScript:   firstNonEmpty(os.Getenv(envStageOneScript), "./stage1-extract/Invoke-FlexAppInventory.ps1"),
		CPEMappingsPath:  firstNonEmpty(os.Getenv(envCPEMappingsPath), "./config/cpe-mappings.yaml"),
		PickerAddr:       firstNonEmpty(os.Getenv(envPickerAddr), DefaultPickerAddr),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.HTTPAddr == "" {
		return fmt.Errorf("%s must not be empty", envHTTPAddr)
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("%s must be one of debug, info, warn, error (got %q)", envLogLevel, c.LogLevel)
	}
	return nil
}

// DefaultLogFile is LogFile's default when FVS_LOG_FILE is unset. Exported
// so cmd/tray can resolve the same default without duplicating the literal.
const DefaultLogFile = "./flexapp-vuln-scanner.log"

// DefaultHTTPAddr is HTTPAddr's default when FVS_HTTP_ADDR is unset.
// Exported for the same reason as DefaultLogFile.
const DefaultHTTPAddr = "127.0.0.1:8743"

// DefaultPickerAddr is PickerAddr's default when FVS_PICKER_ADDR is
// unset. Exported so cmd/tray's picker server and this package's Load
// agree on the same default without duplicating the literal.
const DefaultPickerAddr = "127.0.0.1:8745"

// defaultLogLevel is the LogLevel default when FVS_LOG_LEVEL is unset:
// verbose in development, quieter in every other environment.
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
