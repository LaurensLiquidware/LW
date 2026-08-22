// Command flexapp-vuln-scanner-cli runs a single scan (or Stage 2-only
// refresh) from the terminal, with no HTTP server and no Angular UI --
// for scripted/scheduled use (a CI pipeline, a cron job, an operator
// who just wants one package scanned without opening a browser).
//
// It calls exactly the same internal/pipeline functions the HTTP API's
// ScanDeps does, so behavior can never drift between the two front ends.
//
// On Windows, running a scan (-package) shells out to Stage 1, which
// mounts the VHDX being scanned (Mount-DiskImage, see
// stage1-extract/Mount-ClassicFlexApp.ps1) -- an operation that requires
// Administrator. Unlike cmd/tray and cmd/server, this binary deliberately
// carries no requireAdministrator manifest: a UAC elevation prompt has no
// way to display (and nothing to answer it) in the unattended contexts
// this exists for. Run it from an already-elevated context instead -- a
// Scheduled Task configured to "Run with highest privileges", or an
// already-elevated shell -- or stick to -refresh (Stage 2 only, no
// mounting) if the inventory already exists.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata"

	"flexapp-vuln-scanner/internal/config"
	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/pipeline"
	"flexapp-vuln-scanner/internal/version"
)

// stdoutSink implements pipeline.ProgressSink by printing directly to
// stdout/stderr -- the CLI's only UI.
type stdoutSink struct {
	status string
}

func (s *stdoutSink) SetStatus(status string) {
	s.status = status
	fmt.Printf("==> %s\n", status)
}

func (s *stdoutSink) AppendLog(line string) {
	fmt.Println(line)
}

func (s *stdoutSink) SetProgress(phase string, done, total int) {
	fmt.Printf("    [%s] %d/%d\n", phase, done, total)
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-version", "-v":
			fmt.Println(version.Version)
			return
		}
	}

	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("flexapp-vuln-scanner-cli", flag.ContinueOnError)
	packagePath := fs.String("package", "", "path to the FlexApp package to scan (VHDX or FlexApp One exe)")
	refreshInventory := fs.String("refresh", "", "path to an existing *.inventory.json to re-run Stage 2 (OSV/NVD matching) against, skipping Stage 1")
	outputDir := fs.String("output", "", "directory to write the inventory/reports to (required)")
	nvdAPIKey := fs.String("nvd-api-key", os.Getenv("FVS_NVD_API_KEY"), "NVD 2.0 API key (optional; raises the rate limit from 5 to 50 req/30s)")
	stage1Script := fs.String("stage1-script", "", "path to the Stage 1 PowerShell script (defaults to FVS_STAGE1_SCRIPT or ./stage1-extract/Invoke-FlexAppInventory.ps1)")
	cacheDir := fs.String("cache-dir", "", "OSV/NVD response cache directory (defaults to FVS_CACHE_DIR or ./cache)")
	cpeMappingsPath := fs.String("cpe-mappings", "", "path to cpe-mappings.yaml (defaults to FVS_CPE_MAPPINGS_PATH or ./config/cpe-mappings.yaml)")
	skipDefenderScan := fs.Bool("skip-defender-scan", false, "skip Stage 1's Windows Defender scan of the mounted package (defaults to FVS_SKIP_DEFENDER_SCAN, or false)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *outputDir == "" {
		return fmt.Errorf("-output is required")
	}
	if *packagePath == "" && *refreshInventory == "" {
		return fmt.Errorf("either -package (fresh scan) or -refresh (Stage 2 only) is required")
	}
	if *packagePath != "" && *refreshInventory != "" {
		return fmt.Errorf("-package and -refresh are mutually exclusive")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if *stage1Script == "" {
		*stage1Script = cfg.StageOneScript
	}
	if *cacheDir == "" {
		*cacheDir = cfg.CacheDir
	}
	if *cpeMappingsPath == "" {
		*cpeMappingsPath = cfg.CPEMappingsPath
	}
	// Either source asking to skip is enough to skip -- there's no clean
	// way to tell "the flag was explicitly passed as false" apart from
	// "the flag was never passed" with the standard flag package, so
	// this ORs them rather than letting an unset flag silently override
	// FVS_SKIP_DEFENDER_SCAN=true back to false.
	effectiveSkipDefenderScan := *skipDefenderScan || cfg.SkipDefenderScan

	mappings, err := cpemap.Load(*cpeMappingsPath)
	if err != nil {
		return fmt.Errorf("load CPE mappings: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sink := &stdoutSink{}
	inventoryPath := *refreshInventory
	if *packagePath != "" {
		fmt.Printf("Scanning %s -> %s\n", *packagePath, *outputDir)
		inventoryPath, err = pipeline.RunStage1(ctx, sink, *stage1Script, *packagePath, *outputDir, effectiveSkipDefenderScan)
		if err != nil {
			return fmt.Errorf("stage 1: %w", err)
		}
	} else {
		fmt.Printf("Refreshing vulnerability matches for %s -> %s\n", inventoryPath, *outputDir)
	}

	result, err := pipeline.RunStage2(ctx, sink, inventoryPath, *outputDir, *cacheDir, *nvdAPIKey, mappings)
	if err != nil {
		return fmt.Errorf("stage 2: %w", err)
	}

	fmt.Println()
	fmt.Printf("Package:  %s\n", result.PackageName)
	if result.Coverage.CoveragePercent != nil {
		fmt.Printf("Coverage: %.1f%% (%d/%d resolved)\n", *result.Coverage.CoveragePercent, result.Coverage.ResolvedComponents, result.Coverage.CandidateComponents)
	} else {
		fmt.Println("Coverage: N/A (no candidate components found)")
	}
	fmt.Printf("Findings: %dC / %dH / %dM / %dL\n",
		result.SeverityCounts["CRITICAL"], result.SeverityCounts["HIGH"], result.SeverityCounts["MEDIUM"], result.SeverityCounts["LOW"])
	if result.MalwareScan != nil {
		switch result.MalwareScan.Status {
		case "clean":
			fmt.Println("Malware:  clean (Windows Defender)")
		case "threats-found":
			fmt.Printf("Malware:  %d threat(s) found by Windows Defender:\n", len(result.MalwareScan.Threats))
			for _, t := range result.MalwareScan.Threats {
				fmt.Printf("  - %s\n", t.ThreatName)
			}
		case "unavailable":
			fmt.Println("Malware:  Windows Defender not available on this machine -- not scanned")
		default:
			fmt.Println("Malware:  scan could not be completed/confirmed -- see the log above")
		}
	}
	fmt.Println()
	fmt.Println("Reports written:")
	fmt.Printf("  SBOM:     %s\n", result.Files.SBOM)
	fmt.Printf("  Coverage: %s\n", result.Files.CoverageReport)
	fmt.Printf("  Findings: %s\n", result.Files.Findings)
	fmt.Printf("  PDF:      %s\n", result.Files.PDF)
	if result.Files.FindingsCSV != "" {
		fmt.Printf("  CSV:      %s\n", result.Files.FindingsCSV)
	}

	return nil
}
