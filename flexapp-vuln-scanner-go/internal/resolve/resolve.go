// Package resolve matches a Stage 1 inventory's resolved identities
// against OSV.dev (purl-expressible components) and NVD (CPE-eligible
// native/OS components), producing a vuln-matches result.
package resolve

import (
	"context"
	"fmt"
	"sort"
	"time"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/inventory"
	"flexapp-vuln-scanner/internal/normalize"
	"flexapp-vuln-scanner/internal/nvd"
	"flexapp-vuln-scanner/internal/osv"
)

// Vulnerability is one matched vulnerability against a component.
type Vulnerability struct {
	ID            string  `json:"id"`
	Summary       *string `json:"summary"`
	Severity      []any   `json:"severity"`
	SeverityLevel *string `json:"severityLevel"`
	Source        string  `json:"source"`
}

// Component is one candidate component's match result.
type Component struct {
	RelativePath    string              `json:"relativePath"`
	Identity        *inventory.Identity `json:"identity"`
	Purl            string              `json:"purl,omitempty"`
	CPE             string              `json:"cpe,omitempty"`
	Confidence      string              `json:"confidence,omitempty"`
	Vulnerabilities []Vulnerability     `json:"vulnerabilities"`
}

// Result is the full vuln-matches.json document.
type Result struct {
	GeneratedUTC string            `json:"generatedUtc"`
	Package      inventory.Package `json:"package"`
	Components   []Component       `json:"components"`
}

// ProgressFunc reports progress during the two sequential per-item
// network loops that dominate Resolve's runtime: phase "osv" (fetching
// each distinct OSV vuln ID's detail) and phase "nvd" (querying each
// candidate CPE, rate-limited).
type ProgressFunc func(phase string, done, total int)

type candidate struct {
	relativePath  string
	identity      *inventory.Identity
	purl          string
	cpe           string
	cpeConfidence string
}

// Resolve matches every non-excluded file's identity against OSV.dev
// (purl-expressible) or NVD (CPE-eligible), returning the combined
// result. cacheDir is shared with the OSV/NVD on-disk response caches.
func Resolve(ctx context.Context, inv *inventory.Inventory, cacheDir string, mappings *cpemap.Mappings, nvdAPIKey string, onProgress ProgressFunc) (*Result, error) {
	osvClient, err := osv.New(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("set up osv client: %w", err)
	}
	nvdClient, err := nvd.New(cacheDir, nvdAPIKey)
	if err != nil {
		return nil, fmt.Errorf("set up nvd client: %w", err)
	}
	return ResolveWithClients(ctx, inv, mappings, osvClient, nvdClient, onProgress)
}

// ResolveWithClients is Resolve with pre-built OSV/NVD clients, as a
// seam for tests that need to point those clients at a mock server
// (production code should call Resolve).
func ResolveWithClients(ctx context.Context, inv *inventory.Inventory, mappings *cpemap.Mappings, osvClient *osv.Client, nvdClient *nvd.Client, onProgress ProgressFunc) (*Result, error) {
	var candidates []candidate
	purlSet := map[string]struct{}{}
	cpeSet := map[string]struct{}{}

	for _, f := range inv.NonExcludedFiles() {
		identity := f.Identity
		purl := normalize.BuildPurl(identity)
		var cpe, cpeConfidence string
		if purl == "" {
			cpe, cpeConfidence = normalize.BuildCPECandidate(identity, mappings)
		}

		if purl != "" {
			purlSet[purl] = struct{}{}
		}
		if cpe != "" {
			cpeSet[cpe] = struct{}{}
		}

		candidates = append(candidates, candidate{
			relativePath:  f.RelativePath,
			identity:      identity,
			purl:          purl,
			cpe:           cpe,
			cpeConfidence: cpeConfidence,
		})
	}

	purls := sortedKeys(purlSet)
	var osvMatches map[string][]map[string]any
	if len(purls) > 0 {
		osvOnProgress := func(done, total int) {
			if onProgress != nil {
				onProgress("osv", done, total)
			}
		}
		var err error
		osvMatches, err = osvClient.Resolve(ctx, purls, osvOnProgress)
		if err != nil {
			return nil, fmt.Errorf("could not reach api.osv.dev: %w", err)
		}
	}

	sortedCPEs := sortedKeys(cpeSet)
	nvdMatches := map[string][]nvd.CVE{}
	for i, cpe23 := range sortedCPEs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		response, err := nvdClient.QueryCPE(ctx, cpe23)
		if err != nil {
			return nil, fmt.Errorf("could not reach services.nvd.nist.gov: %w", err)
		}
		nvdMatches[cpe23] = nvd.ExtractCVEs(response)
		if onProgress != nil {
			onProgress("nvd", i+1, len(sortedCPEs))
		}
	}

	components := make([]Component, 0, len(candidates))
	for _, c := range candidates {
		var confidence string
		var vulns []Vulnerability

		switch {
		case c.purl != "":
			confidence = normalize.ConfidenceExactPurl
			for _, v := range osvMatches[c.purl] {
				vulns = append(vulns, vulnFromOSV(v))
			}
		case c.cpe != "":
			confidence = c.cpeConfidence
			for _, v := range nvdMatches[c.cpe] {
				vulns = append(vulns, vulnFromNVD(v))
			}
		}

		components = append(components, Component{
			RelativePath:    c.relativePath,
			Identity:        c.identity,
			Purl:            c.purl,
			CPE:             c.cpe,
			Confidence:      confidence,
			Vulnerabilities: vulns,
		})
	}

	return &Result{
		GeneratedUTC: time.Now().UTC().Format(time.RFC3339),
		Package:      inv.Package,
		Components:   components,
	}, nil
}

func vulnFromOSV(v map[string]any) Vulnerability {
	id, _ := v["id"].(string)
	var summary *string
	if s, ok := v["summary"].(string); ok {
		summary = &s
	}
	severity, _ := v["severity"].([]any)
	var severityLevel *string
	if dbSpecific, ok := v["database_specific"].(map[string]any); ok {
		// GHSA-sourced OSV entries commonly carry this; many other
		// ecosystems don't -- nil is an honest "unknown", not a guess.
		if s, ok := dbSpecific["severity"].(string); ok {
			severityLevel = &s
		}
	}
	return Vulnerability{ID: id, Summary: summary, Severity: severity, SeverityLevel: severityLevel, Source: "osv"}
}

func vulnFromNVD(v nvd.CVE) Vulnerability {
	severity := make([]any, len(v.Severity))
	for i, s := range v.Severity {
		severity[i] = s
	}
	return Vulnerability{ID: v.ID, Summary: v.Summary, Severity: severity, SeverityLevel: v.SeverityLevel, Source: "nvd"}
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
