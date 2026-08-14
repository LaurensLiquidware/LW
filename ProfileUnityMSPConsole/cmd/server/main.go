// Command server runs the ProfileUnity MSP Licensing Console.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"profileunity-msp-console/internal/config"
	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/httpapi"
	"profileunity-msp-console/internal/scheduler"
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go sched.Run(ctx)

	router, err := httpapi.NewRouter(func() httpapi.SchedulerStatus {
		return schedulerStatusFor(sched.Status())
	})
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: router}
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
