// Package sbom builds a CycloneDX 1.6 JSON SBOM from a Stage 1
// inventory -- the per-scan finding SBOM, not to be confused with this
// tool's own dependency SBOM (bom.cdx.json, internal/legal).
//
// Deliberately independent of OSV/NVD matching (and therefore of
// network access) -- purl/CPE are recomputed here directly, the same
// way vulnerability matching does, so an SBOM can always be produced
// from an inventory JSON alone.
//
// No license data is included -- Stage 1 never captures license
// information for any component, and CycloneDX allows omitting the
// licenses field entirely.
package sbom

import (
	"crypto/rand"
	"fmt"
	"time"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/inventory"
	"flexapp-vuln-scanner/internal/normalize"
)

// Hash is a CycloneDX component hash entry.
type Hash struct {
	Alg     string `json:"alg"`
	Content string `json:"content"`
}

// Property is a CycloneDX component property (name/value pair).
type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Component is one CycloneDX SBOM component entry.
type Component struct {
	Type       string     `json:"type"`
	Name       string     `json:"name"`
	Version    string     `json:"version"`
	BomRef     string     `json:"bom-ref"`
	Properties []Property `json:"properties"`
	Purl       string     `json:"purl,omitempty"`
	CPE        string     `json:"cpe,omitempty"`
	Hashes     []Hash     `json:"hashes,omitempty"`
}

// Metadata is the CycloneDX document's metadata block.
type Metadata struct {
	Timestamp string            `json:"timestamp"`
	Component MetadataComponent `json:"component"`
}

// MetadataComponent describes the application the SBOM is for.
type MetadataComponent struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Document is a CycloneDX 1.6 JSON SBOM.
type Document struct {
	BomFormat    string      `json:"bomFormat"`
	SpecVersion  string      `json:"specVersion"`
	SerialNumber string      `json:"serialNumber"`
	Version      int         `json:"version"`
	Metadata     Metadata    `json:"metadata"`
	Components   []Component `json:"components"`
}

func dedupKey(purl, cpe string, identity *inventory.Identity) string {
	if purl != "" {
		return "purl:" + purl
	}
	if cpe != "" {
		return "cpe:" + cpe
	}
	// Only reached for resolved-but-neither-purl-nor-cpe identities
	// (currently just jar-manifest) -- dedupe by the raw identity triple.
	product := ""
	version := ""
	if identity.Product != nil {
		product = *identity.Product
	}
	if identity.Version != nil {
		version = *identity.Version
	}
	return fmt.Sprintf("raw:%s:%s:%s", identity.Method, product, version)
}

func randomUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Build builds a CycloneDX 1.6 SBOM document from an inventory.
func Build(inv *inventory.Inventory, mappings *cpemap.Mappings) Document {
	appName := inv.Package.SourcePath
	appVersion := "0.0.0.0"
	if inv.Package.FlexAppXML != nil {
		if inv.Package.FlexAppXML.DisplayName != nil && *inv.Package.FlexAppXML.DisplayName != "" {
			appName = *inv.Package.FlexAppXML.DisplayName
		}
		if inv.Package.FlexAppXML.VersionMajorMinorBuildRevision != nil && *inv.Package.FlexAppXML.VersionMajorMinorBuildRevision != "" {
			appVersion = *inv.Package.FlexAppXML.VersionMajorMinorBuildRevision
		}
	}
	if appName == "" {
		appName = "unknown-package"
	}

	seenOrder := []string{}
	seen := map[string]Component{}

	for _, f := range inv.NonExcludedFiles() {
		identity := f.Identity
		if identity == nil {
			continue
		}
		// A resolved identity with no product name has nothing to put
		// in CycloneDX's required, string-typed name field -- skip it
		// here rather than emitting an invalid component.
		if identity.Product == nil || *identity.Product == "" {
			continue
		}

		purl := normalize.BuildPurl(identity)
		var cpe, cpeConfidence string
		if purl == "" {
			cpe, cpeConfidence = normalize.BuildCPECandidate(identity, mappings)
		}
		confidence := cpeConfidence
		if purl != "" {
			confidence = normalize.ConfidenceExactPurl
		}
		if confidence == "" {
			confidence = "unresolved"
		}

		key := dedupKey(purl, cpe, identity)
		if _, ok := seen[key]; ok {
			continue
		}

		version := ""
		if identity.Version != nil {
			version = *identity.Version
		}

		component := Component{
			Type:    "library",
			Name:    *identity.Product,
			Version: version,
			BomRef:  key,
			Properties: []Property{
				{Name: "flexapp-vuln:resolutionMethod", Value: identity.Method},
				{Name: "flexapp-vuln:confidence", Value: confidence},
			},
			Purl: purl,
			CPE:  cpe,
		}
		if f.SHA256 != nil && *f.SHA256 != "" {
			component.Hashes = []Hash{{Alg: "SHA-256", Content: *f.SHA256}}
		}

		seen[key] = component
		seenOrder = append(seenOrder, key)
	}

	components := make([]Component, 0, len(seenOrder))
	for _, key := range seenOrder {
		components = append(components, seen[key])
	}

	return Document{
		BomFormat:    "CycloneDX",
		SpecVersion:  "1.6",
		SerialNumber: "urn:uuid:" + randomUUID(),
		Version:      1,
		Metadata: Metadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Component: MetadataComponent{
				Type:    "application",
				Name:    appName,
				Version: appVersion,
			},
		},
		Components: components,
	}
}
