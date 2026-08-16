// Command gendemodb builds demo.db: a standalone, pre-seeded sqlite
// database containing ten fictional MSP tenants and up to --months of
// daily license history, for dropping next to a fresh install's real
// database (see README.md's "Demo data" section and
// cmd/server/main.go's openDatabase). It is not shipped in the repo --
// see the release process for when it's actually generated.
//
// Schema is built by running the real migrations (internal/db.Open), not
// hand-written DDL, and every row is written through the same repo
// methods (tenant.Repo.Create, snapshot.Repo.Upsert) production code
// uses, so this can never drift from what the app itself expects.
//
// Determinism: --seed controls every piece of *usage* randomness (the
// daily jitter baseUtilization's noise applies) -- two runs with the same
// seed produce the same tenant roster, the same day-by-day statuses, and
// the same UsedLicenses/TotalLicenses figures. Row IDs (tenant.Repo.Create
// and snapshot.Repo.Upsert both mint a fresh UUID via crypto/rand) and the
// encrypted credential blob (a fresh random AES-GCM nonce every call) are
// NOT reproducible byte-for-byte across runs, and must not be made so --
// both are security-relevant randomness this tool deliberately does not
// touch. The golden test (gendemodb_test.go) checksums only the
// deterministic fields.
package main

import (
	"context"
	crand "crypto/rand"
	"flag"
	"fmt"
	"log"
	mathrand "math/rand/v2"
	"os"
	"time"

	pumccrypto "profileunity-msp-console/internal/crypto"
	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("gendemodb: %v", err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("gendemodb", flag.ContinueOnError)
	out := fs.String("out", "./demo.db", "path to write the generated demo database to")
	seed := fs.Uint64("seed", defaultSeed, "seed controlling all usage-figure randomness (jitter); fixed by default for reproducible output")
	tenantsFlag := fs.Int("tenants", len(roster), fmt.Sprintf("number of roster tenants to include, in table order (max %d -- the roster is a fixed set of named storylines, not randomly generated entries)", len(roster)))
	months := fs.Int("months", 6, "months of daily history to generate, ending on --end-date")
	endDateFlag := fs.String("end-date", "", "YYYY-MM-DD; history is generated relative to this date, defaulting to today. Regenerate the checked-in release artifact per release so it's always \"the last N months\" -- see README.md")
	if err := fs.Parse(args); err != nil {
		return err
	}

	endDate := time.Now().UTC()
	if *endDateFlag != "" {
		parsed, err := time.Parse("2006-01-02", *endDateFlag)
		if err != nil {
			return fmt.Errorf("--end-date: %w", err)
		}
		endDate = parsed
	}
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)

	tenantCount := *tenantsFlag
	if tenantCount > len(roster) {
		return fmt.Errorf("--tenants must be at most %d (the roster is a fixed set of named storylines)", len(roster))
	}
	if tenantCount < 1 {
		return fmt.Errorf("--tenants must be at least 1")
	}
	if *months < 1 {
		return fmt.Errorf("--months must be at least 1")
	}

	if _, err := os.Stat(*out); err == nil {
		return fmt.Errorf("%s already exists -- remove it first (this tool never overwrites an existing file, to avoid silently destroying a previous generation someone may still be using)", *out)
	}

	sqlDB, err := db.Open("sqlite", *out)
	if err != nil {
		return fmt.Errorf("create %s: %w", *out, err)
	}
	defer sqlDB.Close()

	// A random, in-memory-only key: the encrypted credential blobs this
	// tool writes are never decrypted while demo mode's safety rails hold
	// (the collector/Test-Connection/Collect-Now are all disabled -- see
	// internal/httpapi.DisallowInDemoMode), so which key encrypted them is
	// irrelevant, and persisting one that will never again be needed would
	// just be a stray file to explain. internal/tenant.Repo.GetCredentials
	// already degrades gracefully (returns an error, never panics) on a
	// key mismatch regardless -- see internal/tenant/repo.go's Decrypt
	// call site.
	key := make([]byte, pumccrypto.KeySize)
	if _, err := crand.Read(key); err != nil {
		return fmt.Errorf("generate credential encryption key: %w", err)
	}

	tenantRepo := tenant.NewRepo(sqlDB, key)
	snapshotRepo := snapshot.NewRepo(sqlDB)

	if err := generate(context.Background(), tenantRepo, snapshotRepo, tenantCount, *months, endDate, *seed); err != nil {
		return err
	}

	// A single self-contained file is the whole point of a drop-in
	// sidecar -- checkpoint WAL back into the main file and truncate it so
	// no -wal/-shm sidecars ship alongside demo.db (see internal/db.Open's
	// doc comment on why WAL mode is on in the first place).
	if _, err := sqlDB.ExecContext(context.Background(), `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("checkpoint WAL: %w", err)
	}

	log.Printf("gendemodb: wrote %s (%d tenants, %d months, ending %s)", *out, tenantCount, *months, endDate.Format("2006-01-02"))
	return nil
}

// generate writes tenantCount roster entries and their full history
// (months of daily snapshots, ending on endDate) through tenantRepo and
// snapshotRepo -- the real repo methods, not hand-written SQL. Factored
// out of run() so the golden test (gendemodb_test.go) can exercise it
// directly against a temp database without going through flag parsing or
// file-existence checks.
func generate(ctx context.Context, tenantRepo *tenant.Repo, snapshotRepo *snapshot.Repo, tenantCount, months int, endDate time.Time, seed uint64) error {
	totalDays := months * 30
	windowStart := endDate.AddDate(0, 0, -(totalDays - 1))
	rng := mathrand.New(mathrand.NewPCG(seed, seed))

	for i := 0; i < tenantCount; i++ {
		spec := roster[i]
		t, err := tenantRepo.Create(ctx, tenant.CreateInput{
			DisplayName:   spec.displayName,
			Hostname:      spec.hostname,
			Port:          spec.port,
			Username:      "demo-svc",
			Password:      "Demo-Password-Not-Real!",
			TLSSkipVerify: true,
			Enabled:       true,
			Notes:         spec.notes,
		})
		if err != nil {
			return fmt.Errorf("create tenant %q: %w", spec.displayName, err)
		}

		for dayIndex := 0; dayIndex < totalDays; dayIndex++ {
			day := windowStart.AddDate(0, 0, dayIndex)
			plan := planDay(i, dayIndex, totalDays)
			if plan.absent {
				continue
			}
			s := buildSnapshot(i, dayIndex, totalDays, day, t.ID, plan, endDate, rng)
			if _, err := snapshotRepo.Upsert(ctx, s); err != nil {
				return fmt.Errorf("write snapshot for %q on %s: %w", spec.displayName, s.CollectionDate, err)
			}
		}
		log.Printf("gendemodb: wrote %q (%s)", spec.displayName, spec.hostname)
	}
	return nil
}

// defaultSeed is fixed so `gendemodb` with no --seed reproduces the same
// usage figures every time, per the task brief's "fixed default seed"
// requirement.
const defaultSeed = 20260101
