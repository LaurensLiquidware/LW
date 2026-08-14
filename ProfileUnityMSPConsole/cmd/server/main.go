// Command server runs the ProfileUnity MSP Licensing Console.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"profileunity-msp-console/internal/auth"
	"profileunity-msp-console/internal/config"
	"profileunity-msp-console/internal/dashboard"
	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/dotenv"
	"profileunity-msp-console/internal/httpapi"
	"profileunity-msp-console/internal/mailer"
	"profileunity-msp-console/internal/reportemail"
	"profileunity-msp-console/internal/reportmail"
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

	log.Printf("profileunity-msp-console %s starting (environment=%s)", version.Version, cfg.Environment)

	sqlDB, err := db.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close()

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
			log.Printf("generated a self-signed TLS certificate at %s (hosts: %v) — replace it with a CA-signed certificate for production use, from the Settings screen or by replacing these files", cfg.TLSCertFile, hosts)
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

	go sched.Run(ctx)

	authDeps := httpapi.AuthDeps{Users: userRepo, Sessions: sessionRepo, Secure: true}
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
	reportMailSched := reportmail.New(repos, reportemail.NewRepo(sqlDB), reportMailSmtp, current.ReportRecipients, current.ReportEmailDay, collectionLocation)
	go reportMailSched.Run(ctx)
	if current.ReportEmailEnabled() {
		log.Printf("monthly portfolio report emailing enabled: day %d of each month, to %v", current.ReportEmailDay, current.ReportRecipients)
	} else {
		log.Print("SMTP is not configured — monthly portfolio report emailing is disabled (set it up from the Settings screen)")
	}

	dashboardDeps := httpapi.DashboardDeps{Repos: repos, Location: collectionLocation}
	historyDeps := httpapi.HistoryDeps{Repos: repos}
	reportDeps := httpapi.ReportDeps{Repos: repos}
	alertDeps := httpapi.AlertDeps{Repos: repos, Location: collectionLocation}
	schedulerStatus := func() httpapi.SchedulerStatus {
		return schedulerStatusFor(sched.Status())
	}
	collectionDeps := httpapi.CollectionDeps{Scheduler: sched, Status: schedulerStatus}
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
		log.Printf("listening on https://%s", cfg.HTTPAddr)
		if err := server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-ctx.Done():
		log.Print("shutting down")
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
		log.Print("no operator accounts exist and no PUMC_BOOTSTRAP_ADMIN_USERNAME/PASSWORD were set — nobody can sign in yet")
		return nil
	}
	if _, err := users.CreateUser(context.Background(), cfg.BootstrapAdminUsername, cfg.BootstrapAdminPassword, auth.RoleOperator); err != nil {
		return err
	}
	log.Printf("created initial operator account %q", cfg.BootstrapAdminUsername)
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
