// Command server runs the ProfileUnity MSP Licensing Console.
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"profileunity-msp-console/internal/config"
	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/httpapi"
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

	router, err := httpapi.NewRouter()
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	log.Printf("listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, router); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
