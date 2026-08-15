// Package settings stores the operator-editable runtime settings shown
// on the Settings screen: SMTP/report-email configuration, collection
// tunables, session timeouts, and the active TLS certificate. It is
// deliberately separate from internal/config: config.Config is read
// once from the environment at process start because some of it (listen
// address, DB driver/DSN, the credential encryption key) has to exist
// before this database can even be opened; everything in this package
// is safe to change after that, without a restart, and is persisted so
// the change survives one.
package settings

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"profileunity-msp-console/internal/config"
)

// Settings is the full set of operator-editable runtime settings.
type Settings struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SMTPSecurity string

	ReportRecipients []string
	ReportEmailDay   int

	CollectionInterval      time.Duration
	CollectionTimezone      string
	CollectionConcurrency   int
	CollectionTenantTimeout time.Duration

	SessionIdleTimeout     time.Duration
	SessionAbsoluteTimeout time.Duration

	// TLSCertPEM/TLSKeyPEM are the active certificate and key, PEM
	// encoded. Empty until something (the bootstrap self-signed
	// generator, or an operator upload) sets them.
	TLSCertPEM string
	TLSKeyPEM  string
}

// ReportEmailEnabled reports whether monthly report emailing is
// configured at all -- SMTPHost is the single on/off switch, mirroring
// config.Config.ReportEmailEnabled.
func (s Settings) ReportEmailEnabled() bool {
	return s.SMTPHost != ""
}

// FromConfig builds the initial Settings from a loaded config.Config --
// used only to seed the runtime_settings row the very first time the
// server starts against a fresh database.
func FromConfig(cfg config.Config) Settings {
	return Settings{
		SMTPHost:                cfg.SMTPHost,
		SMTPPort:                cfg.SMTPPort,
		SMTPUsername:            cfg.SMTPUsername,
		SMTPPassword:            cfg.SMTPPassword,
		SMTPFrom:                cfg.SMTPFrom,
		SMTPSecurity:            cfg.SMTPSecurity,
		ReportRecipients:        cfg.ReportRecipients,
		ReportEmailDay:          cfg.ReportEmailDay,
		CollectionInterval:      cfg.CollectionInterval,
		CollectionTimezone:      cfg.CollectionTimezone,
		CollectionConcurrency:   cfg.CollectionConcurrency,
		CollectionTenantTimeout: cfg.CollectionTenantTimeout,
		SessionIdleTimeout:      cfg.SessionIdleTimeout,
		SessionAbsoluteTimeout:  cfg.SessionAbsoluteTimeout,
	}
}

var validSMTPSecurity = map[string]bool{"starttls": true, "tls": true, "none": true}

// Validate applies the same cross-field rules config.Config.validate
// enforces for these same settings at boot -- kept in sync deliberately,
// since a Settings-screen update must reject exactly what a fresh
// PUMC_* environment would have rejected at startup.
func (s Settings) Validate() error {
	if !validSMTPSecurity[s.SMTPSecurity] {
		return fmt.Errorf("SMTP security must be one of starttls, tls, none (got %q)", s.SMTPSecurity)
	}
	if s.ReportEmailEnabled() {
		if s.SMTPFrom == "" {
			return fmt.Errorf("an SMTP from address is required once an SMTP host is set")
		}
		if len(s.ReportRecipients) == 0 {
			return fmt.Errorf("at least one report recipient is required once an SMTP host is set")
		}
		if s.SMTPPort <= 0 {
			return fmt.Errorf("SMTP port must be positive (got %d)", s.SMTPPort)
		}
	} else if s.SMTPFrom != "" || len(s.ReportRecipients) > 0 {
		return fmt.Errorf("an SMTP host is required when a from address or report recipients are set")
	}
	if s.ReportEmailDay < 1 || s.ReportEmailDay > 28 {
		return fmt.Errorf("report email day must be between 1 and 28 (got %d) -- capped at 28 so it exists in every month", s.ReportEmailDay)
	}
	if _, err := time.LoadLocation(s.CollectionTimezone); err != nil {
		return fmt.Errorf("unknown IANA timezone %q: %w", s.CollectionTimezone, err)
	}
	if s.CollectionInterval <= 0 {
		return fmt.Errorf("collection interval must be positive (got %s)", s.CollectionInterval)
	}
	if s.CollectionConcurrency < 1 {
		return fmt.Errorf("collection concurrency must be at least 1 (got %d)", s.CollectionConcurrency)
	}
	if s.CollectionTenantTimeout <= 0 {
		return fmt.Errorf("collection tenant timeout must be positive (got %s)", s.CollectionTenantTimeout)
	}
	if s.SessionIdleTimeout <= 0 {
		return fmt.Errorf("session idle timeout must be positive (got %s)", s.SessionIdleTimeout)
	}
	if s.SessionAbsoluteTimeout <= 0 {
		return fmt.Errorf("session absolute timeout must be positive (got %s)", s.SessionAbsoluteTimeout)
	}
	return nil
}

// Location parses CollectionTimezone. Callers should call Validate first
// so this can never fail in practice; it still returns the error rather
// than panicking, since "should never happen" isn't "cannot happen".
func (s Settings) Location() (*time.Location, error) {
	return time.LoadLocation(s.CollectionTimezone)
}

// Store persists Settings in the runtime_settings singleton row.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Load reads the current settings. ok is false if the row doesn't exist
// yet (a fresh database before EnsureSeeded has run).
func (s *Store) Load(ctx context.Context) (settings Settings, ok bool, err error) {
	var recipients string
	var intervalSeconds, tenantTimeoutSeconds, idleSeconds, absoluteSeconds int64

	row := s.db.QueryRowContext(ctx, `
		SELECT smtp_host, smtp_port, smtp_username, smtp_password, smtp_from, smtp_security,
		       report_recipients, report_email_day,
		       collection_interval_seconds, collection_timezone, collection_concurrency, collection_tenant_timeout_seconds,
		       session_idle_timeout_seconds, session_absolute_timeout_seconds,
		       tls_cert_pem, tls_key_pem
		FROM runtime_settings WHERE id = 1`)
	err = row.Scan(
		&settings.SMTPHost, &settings.SMTPPort, &settings.SMTPUsername, &settings.SMTPPassword, &settings.SMTPFrom, &settings.SMTPSecurity,
		&recipients, &settings.ReportEmailDay,
		&intervalSeconds, &settings.CollectionTimezone, &settings.CollectionConcurrency, &tenantTimeoutSeconds,
		&idleSeconds, &absoluteSeconds,
		&settings.TLSCertPEM, &settings.TLSKeyPEM,
	)
	if err == sql.ErrNoRows {
		return Settings{}, false, nil
	}
	if err != nil {
		return Settings{}, false, fmt.Errorf("settings: load: %w", err)
	}

	settings.ReportRecipients = splitAndTrim(recipients)
	settings.CollectionInterval = time.Duration(intervalSeconds) * time.Second
	settings.CollectionTenantTimeout = time.Duration(tenantTimeoutSeconds) * time.Second
	settings.SessionIdleTimeout = time.Duration(idleSeconds) * time.Second
	settings.SessionAbsoluteTimeout = time.Duration(absoluteSeconds) * time.Second
	return settings, true, nil
}

// EnsureSeeded inserts seed (typically settings.FromConfig(cfg)) as the
// runtime_settings row if one doesn't already exist. Returns the settings
// now in effect either way -- the freshly seeded ones, or the ones
// already stored from a previous run.
func (s *Store) EnsureSeeded(ctx context.Context, seed Settings) (Settings, error) {
	existing, ok, err := s.Load(ctx)
	if err != nil {
		return Settings{}, err
	}
	if ok {
		return existing, nil
	}
	if err := s.insert(ctx, seed); err != nil {
		return Settings{}, err
	}
	return seed, nil
}

func (s *Store) insert(ctx context.Context, v Settings) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runtime_settings (
			id, smtp_host, smtp_port, smtp_username, smtp_password, smtp_from, smtp_security,
			report_recipients, report_email_day,
			collection_interval_seconds, collection_timezone, collection_concurrency, collection_tenant_timeout_seconds,
			session_idle_timeout_seconds, session_absolute_timeout_seconds,
			tls_cert_pem, tls_key_pem, updated_at_utc
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.SMTPHost, v.SMTPPort, v.SMTPUsername, v.SMTPPassword, v.SMTPFrom, v.SMTPSecurity,
		strings.Join(v.ReportRecipients, ","), v.ReportEmailDay,
		int64(v.CollectionInterval/time.Second), v.CollectionTimezone, v.CollectionConcurrency, int64(v.CollectionTenantTimeout/time.Second),
		int64(v.SessionIdleTimeout/time.Second), int64(v.SessionAbsoluteTimeout/time.Second),
		v.TLSCertPEM, v.TLSKeyPEM, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("settings: seed: %w", err)
	}
	return nil
}

// Update overwrites the stored settings. Callers are expected to have
// called Validate first.
func (s *Store) Update(ctx context.Context, v Settings) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE runtime_settings SET
			smtp_host = ?, smtp_port = ?, smtp_username = ?, smtp_password = ?, smtp_from = ?, smtp_security = ?,
			report_recipients = ?, report_email_day = ?,
			collection_interval_seconds = ?, collection_timezone = ?, collection_concurrency = ?, collection_tenant_timeout_seconds = ?,
			session_idle_timeout_seconds = ?, session_absolute_timeout_seconds = ?,
			tls_cert_pem = ?, tls_key_pem = ?, updated_at_utc = ?
		WHERE id = 1`,
		v.SMTPHost, v.SMTPPort, v.SMTPUsername, v.SMTPPassword, v.SMTPFrom, v.SMTPSecurity,
		strings.Join(v.ReportRecipients, ","), v.ReportEmailDay,
		int64(v.CollectionInterval/time.Second), v.CollectionTimezone, v.CollectionConcurrency, int64(v.CollectionTenantTimeout/time.Second),
		int64(v.SessionIdleTimeout/time.Second), int64(v.SessionAbsoluteTimeout/time.Second),
		v.TLSCertPEM, v.TLSKeyPEM, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("settings: update: %w", err)
	}
	return nil
}

// UpdateTLSCert overwrites only the stored certificate/key, leaving
// every other setting untouched -- used by the certificate-upload
// endpoint, which has no reason to touch anything else.
func (s *Store) UpdateTLSCert(ctx context.Context, certPEM, keyPEM string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE runtime_settings SET tls_cert_pem = ?, tls_key_pem = ?, updated_at_utc = ? WHERE id = 1`,
		certPEM, keyPEM, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("settings: update TLS cert: %w", err)
	}
	return nil
}

func splitAndTrim(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
