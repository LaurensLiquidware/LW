// Command server runs the ProfileUnity MSP Licensing Console.
package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // embed the IANA zone database so time.LoadLocation works without OS-provided zoneinfo (notably on Windows)

	"profileunity-msp-console/internal/auth"
	"profileunity-msp-console/internal/config"
	pumccrypto "profileunity-msp-console/internal/crypto"
	"profileunity-msp-console/internal/dashboard"
	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/dotenv"
	"profileunity-msp-console/internal/httpapi"
	"profileunity-msp-console/internal/logging"
	"profileunity-msp-console/internal/mailer"
	"profileunity-msp-console/internal/reportemail"
	"profileunity-msp-console/internal/reportmail"
	"profileunity-msp-console/internal/reportpdf"
	"profileunity-msp-console/internal/scheduler"
	"profileunity-msp-console/internal/settings"
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
	"profileunity-msp-console/internal/tlscert"
	"profileunity-msp-console/internal/version"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version.Version)
		return
	}

	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	if err := dotenv.Load(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, logCloser, err := logging.New(cfg)
	if err != nil {
		return fmt.Errorf("set up logging: %w", err)
	}
	defer logCloser.Close()
	slog.SetDefault(logger)

	slog.Info(fmt.Sprintf("profileunity-msp-console %s starting (environment=%s, log_level=%s, log_file=%s)", version.Version, cfg.Environment, cfg.LogLevel, cfg.LogFile))

	sqlDB, demoMode, err := openDatabase(cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close()
	if demoMode {
		slog.Warn(fmt.Sprintf("DEMO MODE: running against %s -- no real tenant data is being read or written; set %s=off to force the real database", db.DemoSidecarPath(cfg.DBDSN), "PUMC_DEMO_MODE"))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The Settings screen's runtime settings (SMTP, collection tunables,
	// session timeouts, the active TLS certificate) live in the database
	// from here on, seeded once from PUMC_* on a fresh install — see
	// internal/settings' package doc for why this is split from Config.
	settingsStore := settings.NewStore(sqlDB)
	current, err := settingsStore.EnsureSeeded(ctx, settings.FromConfig(cfg))
	if err != nil {
		return fmt.Errorf("load runtime settings: %w", err)
	}
	collectionLocation, err := current.Location()
	if err != nil {
		return fmt.Errorf("runtime settings: %w", err)
	}

	// PUMC_CREDENTIAL_ENCRYPTION_KEY, if set, always wins; otherwise a key
	// is generated once and persisted to CredentialEncryptionKeyFile on
	// first boot, then reused as-is on every later boot — same
	// generate-once-then-reuse pattern as the self-signed TLS cert below.
	if cfg.CredentialEncryptionKey == nil {
		key, generated, err := pumccrypto.EnsureKey(cfg.CredentialEncryptionKeyFile)
		if err != nil {
			return fmt.Errorf("ensure credential encryption key: %w", err)
		}
		cfg.CredentialEncryptionKey = key
		if generated {
			slog.Warn(fmt.Sprintf("generated a new tenant-credential encryption key at %s — back this file up; if it's ever lost or changed, every previously stored tenant credential becomes permanently undecryptable", cfg.CredentialEncryptionKeyFile))
		}
	}

	tenantRepo := tenant.NewRepo(sqlDB, cfg.CredentialEncryptionKey)
	snapshotRepo := snapshot.NewRepo(sqlDB)
	sched := scheduler.New(tenantRepo, snapshotRepo, current.CollectionInterval, collectionLocation, current.CollectionConcurrency, current.CollectionTenantTimeout)

	userRepo := auth.NewUserRepo(sqlDB)
	sessionRepo := auth.NewSessionRepo(sqlDB, current.SessionIdleTimeout, current.SessionAbsoluteTimeout)
	if err := bootstrapAdmin(userRepo, cfg); err != nil {
		return fmt.Errorf("bootstrap admin user: %w", err)
	}

	// The active TLS certificate is hot-swappable at runtime (see
	// internal/tlscert.Holder) so an operator can upload a real one from
	// the Settings screen with zero downtime. On a fresh install there's
	// nothing in the database yet, so fall back to the file-based
	// self-signed generator this project has always used, then copy the
	// result into settings so it's the database's problem from now on.
	certHolder := tlscert.NewHolder()
	if current.TLSCertPEM != "" && current.TLSKeyPEM != "" {
		if err := certHolder.Set([]byte(current.TLSCertPEM), []byte(current.TLSKeyPEM)); err != nil {
			return fmt.Errorf("load stored TLS certificate: %w", err)
		}
	} else {
		hosts := tlsHosts(cfg.HTTPAddr)
		generated, err := tlscert.EnsureSelfSigned(cfg.TLSCertFile, cfg.TLSKeyFile, hosts)
		if err != nil {
			return fmt.Errorf("ensure TLS certificate: %w", err)
		}
		if generated {
			slog.Info(fmt.Sprintf("generated a self-signed TLS certificate at %s (hosts: %v) — replace it with a CA-signed certificate for production use, from the Settings screen or by replacing these files", cfg.TLSCertFile, hosts))
		}
		certPEM, err := os.ReadFile(cfg.TLSCertFile)
		if err != nil {
			return fmt.Errorf("read TLS certificate: %w", err)
		}
		keyPEM, err := os.ReadFile(cfg.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("read TLS key: %w", err)
		}
		if err := certHolder.Set(certPEM, keyPEM); err != nil {
			return fmt.Errorf("load generated TLS certificate: %w", err)
		}
		current.TLSCertPEM, current.TLSKeyPEM = string(certPEM), string(keyPEM)
		if err := settingsStore.UpdateTLSCert(ctx, current.TLSCertPEM, current.TLSKeyPEM); err != nil {
			return fmt.Errorf("persist generated TLS certificate: %w", err)
		}
	}

	if demoMode {
		slog.Info("DEMO MODE: the collection scheduler is disabled -- demo tenants' hostnames are fictional and must never be polled")
	} else {
		go sched.Run(ctx)
	}

	authDeps := httpapi.AuthDeps{Users: userRepo, Sessions: sessionRepo, Secure: true, DemoMode: demoMode}
	tenantDeps := httpapi.TenantDeps{Tenants: tenantRepo}
	repos := dashboard.Repos{Tenants: tenantRepo, Snapshots: snapshotRepo}

	// The report-mail scheduler always runs, regardless of whether SMTP
	// is configured yet -- it no-ops on every check until an operator
	// sets PUMC_SMTP_HOST (or the equivalent Settings screen fields),
	// which then takes effect on the very next check with no restart.
	reportMailSmtp := mailer.Config{
		Host:     current.SMTPHost,
		Port:     current.SMTPPort,
		Username: current.SMTPUsername,
		Password: current.SMTPPassword,
		From:     current.SMTPFrom,
		Security: current.SMTPSecurity,
	}
	reportMailSched := reportmail.New(repos, reportemail.NewRepo(sqlDB), reportMailSmtp, current.ReportRecipients, current.ReportEmailDay, collectionLocation, demoMode, brandingFrom(current))
	go reportMailSched.Run(ctx)
	if current.ReportEmailEnabled() {
		slog.Info(fmt.Sprintf("monthly portfolio report emailing enabled: day %d of each month, to %v", current.ReportEmailDay, current.ReportRecipients))
	} else {
		slog.Info("SMTP is not configured — monthly portfolio report emailing is disabled (set it up from the Settings screen)")
	}

	dashboardDeps := httpapi.DashboardDeps{Repos: repos, Location: collectionLocation}
	historyDeps := httpapi.HistoryDeps{Repos: repos}
	reportDeps := httpapi.ReportDeps{
		Repos:    repos,
		DemoMode: demoMode,
		// A closure, not a fixed value, so a branding change from the
		// Settings screen is reflected on the very next report download
		// with no restart -- same reasoning as reportMailSched.SetConfig.
		Branding: func() reportpdf.Branding {
			s, _, err := settingsStore.Load(context.Background())
			if err != nil {
				return reportpdf.Branding{}
			}
			return brandingFrom(s)
		},
	}
	alertDeps := httpapi.AlertDeps{Repos: repos, Location: collectionLocation}
	schedulerStatus := func() httpapi.SchedulerStatus {
		return schedulerStatusFor(sched.Status())
	}
	collectionDeps := httpapi.CollectionDeps{Scheduler: sched, Status: schedulerStatus, DemoMode: demoMode}
	settingsDeps := httpapi.SettingsDeps{
		Store:      settingsStore,
		Sessions:   sessionRepo,
		Scheduler:  sched,
		ReportMail: reportMailSched,
		TLSCert:    certHolder,
	}
	router, err := httpapi.NewRouter(schedulerStatus, authDeps, tenantDeps, dashboardDeps, historyDeps, reportDeps, alertDeps, collectionDeps, settingsDeps)
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	// TLSConfig.GetCertificate (rather than passing file paths to
	// ListenAndServeTLS) is what makes certHolder.Set take effect on the
	// very next handshake without restarting this listener.
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: router, TLSConfig: &tls.Config{GetCertificate: certHolder.GetCertificate}}
	serveErr := make(chan error, 1)
	go func() {
		slog.Info(fmt.Sprintf("listening on https://%s", cfg.HTTPAddr))
		if err := server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}
}

// openDatabase opens either the configured production database, or a
// demo.db sidecar file next to it, and reports which. Detection only
// applies to the sqlite driver (postgres has no single-file DSN for a
// sidecar to sit next to, and isn't implemented yet regardless).
// PUMC_DEMO_MODE=off forces the real database even when demo.db is
// present. A demo database is opened via db.OpenDemo, which never
// migrates it and errors on a corrupt file or a schema mismatch (e.g. a
// demo.db generated by an older binary, before a migration this one
// expects existed) -- rather than let that take down the whole server
// and block access to real customer data too, this function logs it
// loudly and falls back to the real database, exactly as if demo.db had
// never been there.
func openDatabase(cfg config.Config) (sqlDB *sql.DB, demoMode bool, err error) {
	if cfg.DBDriver != "sqlite" || cfg.DemoModeDisabled() {
		sqlDB, err = db.Open(cfg.DBDriver, cfg.DBDSN)
		return sqlDB, false, err
	}

	demoPath := db.DemoSidecarPath(cfg.DBDSN)
	if _, statErr := os.Stat(demoPath); statErr != nil {
		sqlDB, err = db.Open(cfg.DBDriver, cfg.DBDSN)
		return sqlDB, false, err
	}

	demoDB, demoErr := db.OpenDemo(demoPath)
	if demoErr != nil {
		slog.Error(fmt.Sprintf("demo.db at %s is present but unusable (%v) -- falling back to the real database %s; regenerate it with cmd/gendemodb (or delete it) to use demo mode again", demoPath, demoErr, cfg.DBDSN))
		sqlDB, err = db.Open(cfg.DBDriver, cfg.DBDSN)
		return sqlDB, false, err
	}
	return demoDB, true, nil
}

// brandingFrom builds the reportpdf.Branding a Settings value implies --
// shared logic with internal/httpapi's own brandingFrom, kept in sync
// deliberately since both derive the same PDF-header branding from the
// same Settings fields.
func brandingFrom(s settings.Settings) reportpdf.Branding {
	return reportpdf.Branding{
		CompanyName:   s.CompanyName,
		LogoImage:     s.CompanyLogoImage,
		LogoImageType: s.CompanyLogoImageType,
	}
}

// bootstrapAdmin creates the first operator account from
// PUMC_BOOTSTRAP_ADMIN_USERNAME/PASSWORD if the users table is empty.
// It never overwrites or resets an existing account.
func bootstrapAdmin(users *auth.UserRepo, cfg config.Config) error {
	count, err := users.Count(context.Background())
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if cfg.BootstrapAdminUsername == "" {
		slog.Info("no operator accounts exist and no PUMC_BOOTSTRAP_ADMIN_USERNAME/PASSWORD were set — nobody can sign in yet")
		return nil
	}
	if _, err := users.CreateUser(context.Background(), cfg.BootstrapAdminUsername, cfg.BootstrapAdminPassword, auth.RoleOperator); err != nil {
		return err
	}
	slog.Info(fmt.Sprintf("created initial operator account %q", cfg.BootstrapAdminUsername))
	if cfg.BootstrapAdminUsername == config.DefaultBootstrapAdminUsername && cfg.BootstrapAdminPassword == config.DefaultBootstrapAdminPassword {
		slog.Warn("created the built-in LiquidwareMSP admin account with its default password — change it from the account/change-password screen as soon as you sign in")
	}
	return nil
}

// tlsHosts derives the Subject Alternative Names for the self-signed
// certificate from the configured listen address, always including
// localhost/127.0.0.1 so local access works regardless of bind address.
func tlsHosts(httpAddr string) []string {
	hosts := []string{"localhost", "127.0.0.1"}
	host, _, err := net.SplitHostPort(httpAddr)
	if err != nil || host == "" || host == "0.0.0.0" || host == "::" {
		return hosts
	}
	return append(hosts, host)
}

// schedulerStatusFor maps the scheduler's internal status to the shape
// the health endpoint exposes. Kept in main rather than in httpapi so the
// HTTP layer does not need to import the scheduler package directly.
func schedulerStatusFor(s scheduler.Status) httpapi.SchedulerStatus {
	status := s.LastRunOutcome
	if status == "" {
		status = "never_run"
	}
	out := httpapi.SchedulerStatus{
		Status:       status,
		Running:      s.Running,
		LastRunError: s.LastRunError,
		TenantCount:  s.LastRunSummary.TenantCount,
	}
	if !s.LastRunAt.IsZero() {
		out.LastRunAtUTC = s.LastRunAt.Format("2006-01-02T15:04:05Z")
	}
	if s.LastRunSummary.Counts != nil {
		out.SuccessCount = s.LastRunSummary.Counts["success"]
	}
	return out
}
