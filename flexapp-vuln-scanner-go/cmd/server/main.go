// Command server runs the FlexApp Vulnerability Scanner's headless HTTP
// API + embedded Angular frontend. It has no database and no auth --
// this is a local, single-user tool, launched and supervised by
// cmd/tray, listening on loopback only by default.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // embed the IANA zone database so time.LoadLocation works without OS-provided zoneinfo (notably on Windows)

	"flexapp-vuln-scanner/internal/config"
	"flexapp-vuln-scanner/internal/dotenv"
	"flexapp-vuln-scanner/internal/httpapi"
	"flexapp-vuln-scanner/internal/logging"
	"flexapp-vuln-scanner/internal/version"
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

	slog.Info(fmt.Sprintf("flexapp-vuln-scanner %s starting (environment=%s, log_level=%s, log_file=%s)", version.Version, cfg.Environment, cfg.LogLevel, cfg.LogFile))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	router, err := httpapi.NewRouter()
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: router}
	serveErr := make(chan error, 1)
	go func() {
		slog.Info(fmt.Sprintf("listening on http://%s", cfg.HTTPAddr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
