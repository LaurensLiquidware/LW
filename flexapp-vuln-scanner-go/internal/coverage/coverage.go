// Package coverage computes resolution coverage, per PLAN.md's exact
// definition.
//
// Deliberately independent of OSV/NVD matching -- the headline coverage
// percentage is entirely about identity resolution (did Stage 1 figure
// out what a file is?), not vulnerability matching (did we find a CVE
// for it?). This means a coverage report can always be produced from an
// inventory JSON alone, with no network access required.
//
// Definitions (verbatim intent from PLAN.md):
//
//	denominator = every file with excluded: false (candidate components)
//	numerator   = of those, every file with identity != null (resolved)
//	unresolved  = denominator - numerator
//	excluded files are reported but outside both numerator and denominator
package coverage

import (
	"flexapp-vuln-scanner/internal/inventory"
)

// UnresolvedFile is one candidate component Stage 1 could not resolve.
type UnresolvedFile struct {
	RelativePath  string  `json:"relativePath"`
	ComponentType string  `json:"componentType"`
	ReadError     *string `json:"readError"`
}

// Coverage is the computed resolution-coverage result.
type Coverage struct {
	TotalFilesScanned    int              `json:"totalFilesScanned"`
	ExcludedCount        int              `json:"excludedCount"`
	ExcludedByReason     map[string]int   `json:"excludedByReason"`
	CandidateComponents  int              `json:"candidateComponents"`
	ResolvedComponents   int              `json:"resolvedComponents"`
	ResolvedByMethod     map[string]int   `json:"resolvedByMethod"`
	UnresolvedComponents int              `json:"unresolvedComponents"`
	UnresolvedFiles      []UnresolvedFile `json:"unresolvedFiles"`
	CoveragePercent      *float64         `json:"coveragePercent"`
}

// Compute computes the coverage result for an inventory.
func Compute(inv *inventory.Inventory) Coverage {
	excludedByReason := map[string]int{}
	excludedCount := 0
	for _, f := range inv.Files {
		if !f.Excluded {
			continue
		}
		excludedCount++
		reason := "unknown"
		if f.ExclusionReason != nil && *f.ExclusionReason != "" {
			reason = *f.ExclusionReason
		}
		excludedByReason[reason]++
	}

	candidates := inv.NonExcludedFiles()
	resolvedByMethod := map[string]int{}
	var unresolvedFiles []UnresolvedFile
	numerator := 0
	for _, f := range candidates {
		if f.Identity != nil {
			numerator++
			resolvedByMethod[f.Identity.Method]++
			continue
		}
		unresolvedFiles = append(unresolvedFiles, UnresolvedFile{
			RelativePath:  f.RelativePath,
			ComponentType: f.ComponentType,
			ReadError:     f.ReadError,
		})
	}

	denominator := len(candidates)
	var pct *float64
	if denominator > 0 {
		p := float64(numerator) / float64(denominator) * 100
		pct = &p
	}

	return Coverage{
		TotalFilesScanned:    len(inv.Files),
		ExcludedCount:        excludedCount,
		ExcludedByReason:     excludedByReason,
		CandidateComponents:  denominator,
		ResolvedComponents:   numerator,
		ResolvedByMethod:     resolvedByMethod,
		UnresolvedComponents: len(unresolvedFiles),
		UnresolvedFiles:      unresolvedFiles,
		CoveragePercent:      pct,
	}
}
