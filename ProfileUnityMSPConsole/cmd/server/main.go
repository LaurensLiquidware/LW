// Command server runs the ProfileUnity MSP Licensing Console.
package main

import (
	"context"
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
	"profileunity-msp-console/internal/httpapi"
	"profileunity-msp-console/internal/scheduler"
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

	tenantRepo := tenant.NewRepo(sqlDB, cfg.CredentialEncryptionKey)
	snapshotRepo := snapshot.NewRepo(sqlDB)
	sched := scheduler.New(tenantRepo, snapshotRepo, cfg.CollectionInterval, cfg.CollectionLocation, cfg.CollectionConcurrency, cfg.CollectionTenantTimeout)

	userRepo := auth.NewUserRepo(sqlDB)
	sessionRepo := auth.NewSessionRepo(sqlDB, cfg.SessionIdleTimeout, cfg.SessionAbsoluteTimeout)
	if err := bootstrapAdmin(userRepo, cfg); err != nil {
		return fmt.Errorf("bootstrap admin user: %w", err)
	}

	hosts := tlsHosts(cfg.HTTPAddr)
	generated, err := tlscert.EnsureSelfSigned(cfg.TLSCertFile, cfg.TLSKeyFile, hosts)
	if err != nil {
		return fmt.Errorf("ensure TLS certificate: %w", err)
	}
	if generated {
		log.Printf("generated a self-signed TLS certificate at %s (hosts: %v) — replace it with a CA-signed certificate for production use", cfg.TLSCertFile, hosts)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go sched.Run(ctx)

	authDeps := httpapi.AuthDeps{Users: userRepo, Sessions: sessionRepo, Secure: true}
	tenantDeps := httpapi.TenantDeps{Tenants: tenantRepo}
	repos := dashboard.Repos{Tenants: tenantRepo, Snapshots: snapshotRepo}
	dashboardDeps := httpapi.DashboardDeps{Repos: repos, Location: cfg.CollectionLocation}
	historyDeps := httpapi.HistoryDeps{Repos: repos}
	router, err := httpapi.NewRouter(func() httpapi.SchedulerStatus {
		return schedulerStatusFor(sched.Status())
	}, authDeps, tenantDeps, dashboardDeps, historyDeps)
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: router}
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("listening on https://%s", cfg.HTTPAddr)
		if err := server.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
