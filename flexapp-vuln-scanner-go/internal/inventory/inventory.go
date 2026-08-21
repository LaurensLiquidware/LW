// Package inventory loads and represents a Stage 1 inventory JSON --
// the contract between Stage 1 (PowerShell extraction/inventory) and
// Stage 2 (this Go module's resolution/matching), per
// ../../../flexapp-vuln-scanner/schemas/inventory.schema.json.
package inventory

import (
	"encoding/json"
	"fmt"
	"os"
)

// Identity is a Stage 1 resolved component identity, or nil when Stage 1
// could not resolve one for a given file.
type Identity struct {
	Method  string         `json:"method"`
	Vendor  *string        `json:"vendor,omitempty"`
	Product *string        `json:"product,omitempty"`
	Version *string        `json:"version,omitempty"`
	Raw     map[string]any `json:"raw,omitempty"`
}

// File is one entry in the inventory's "files" array.
type File struct {
	RelativePath    string    `json:"relativePath"`
	SizeBytes       *int64    `json:"sizeBytes"`
	SHA256          *string   `json:"sha256,omitempty"`
	Excluded        bool      `json:"excluded"`
	ExclusionReason *string   `json:"exclusionReason"`
	ComponentType   string    `json:"componentType"`
	Identity        *Identity `json:"identity"`
	ReadError       *string   `json:"readError"`
}

// FlexAppXML is the optional package metadata Stage 1 extracts from a
// FlexApp package's sidecar/embedded XML.
type FlexAppXML struct {
	UUID                           *string  `json:"uuid,omitempty"`
	DisplayName                    *string  `json:"displayName,omitempty"`
	PackageType                    *string  `json:"packageType,omitempty"`
	SizeInGB                       *float64 `json:"sizeInGb,omitempty"`
	ActualSizeInBytes              *int64   `json:"actualSizeInBytes,omitempty"`
	DateCreated                    *string  `json:"dateCreated,omitempty"`
	DateModified                   *string  `json:"dateModified,omitempty"`
	HistoryRaw                     []string `json:"historyRaw,omitempty"`
	VersionMajorMinorBuildRevision *string  `json:"versionMajorMinorBuildRevision,omitempty"`
	ShortcutTargets                []string `json:"shortcutTargets,omitempty"`
	InstallerIDs                   []string `json:"installerIds,omitempty"`
}

// Package is the inventory's "package" object.
type Package struct {
	SourcePath      string      `json:"sourcePath"`
	PackageType     string      `json:"packageType"`
	FlexAppXML      *FlexAppXML `json:"flexAppXml,omitempty"`
	ScanStartedUTC  string      `json:"scanStartedUtc"`
	ScanFinishedUTC string      `json:"scanFinishedUtc"`
	ToolVersion     string      `json:"toolVersion"`
}

// Inventory is the full decoded Stage 1 output.
type Inventory struct {
	SchemaVersion string  `json:"schemaVersion"`
	Package       Package `json:"package"`
	Files         []File  `json:"files"`
}

// Load reads and decodes a Stage 1 inventory JSON file. Unlike the Python
// implementation this does not validate against the JSON Schema document
// directly -- Go's strict struct decoding (DisallowUnknownFields) plays
// the equivalent role of catching a malformed or out-of-contract file.
func Load(path string) (*Inventory, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open inventory: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var inv Inventory
	if err := dec.Decode(&inv); err != nil {
		return nil, fmt.Errorf("decode inventory %s: %w", path, err)
	}
	return &inv, nil
}

// NonExcludedFiles returns the candidate-component set: every file with
// excluded == false. This is the coverage denominator per PLAN.md.
func (inv *Inventory) NonExcludedFiles() []File {
	out := make([]File, 0, len(inv.Files))
	for _, f := range inv.Files {
		if !f.Excluded {
			out = append(out, f)
		}
	}
	return out
}

// DisplayName returns the package's human-facing name: the FlexApp XML's
// displayName if present, otherwise the source path's file stem.
func (inv *Inventory) DisplayName() string {
	if inv.Package.FlexAppXML != nil && inv.Package.FlexAppXML.DisplayName != nil && *inv.Package.FlexAppXML.DisplayName != "" {
		return *inv.Package.FlexAppXML.DisplayName
	}
	return stem(inv.Package.SourcePath)
}

func stem(path string) string {
	if path == "" {
		return "unknown-package"
	}
	base := path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			base = path[i+1:]
			break
		}
	}
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '.' {
			return base[:i]
		}
	}
	return base
}
