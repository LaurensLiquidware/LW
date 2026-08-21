// Package cpemap loads config/cpe-mappings.yaml and looks up manual
// vendor/product -> CPE overrides for a Stage 1 identity.
//
// CPE resolution from PE metadata is lossy and will produce both false
// positives and misses, so this is a small, editable override table
// rather than an attempt to be clever about automatic normalization. A
// match found here is confidence "mapped-cpe"; anything not found here
// falls back to automatic heuristic normalization elsewhere (see the
// normalize package), which is confidence "heuristic" instead -- a
// strictly lower bar of trust.
package cpemap

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"flexapp-vuln-scanner/internal/inventory"
)

// Entry is one cpe-mappings.yaml override.
type Entry struct {
	Match Match `yaml:"match"`
	CPE   CPE   `yaml:"cpe"`
}

// Match is an entry's optional identity-matching criteria. A nil field
// means "don't filter on this".
type Match struct {
	Method  *string `yaml:"method"`
	Vendor  *string `yaml:"vendor"`
	Product *string `yaml:"product"`
}

// CPE is an entry's override: the CPE vendor/product to use instead of
// guessing, plus an optional version transform.
type CPE struct {
	Vendor         string `yaml:"vendor"`
	Product        string `yaml:"product"`
	VersionPattern string `yaml:"versionPattern"`
	VersionGroup   int    `yaml:"versionGroup"`
}

type file struct {
	Mappings []Entry `yaml:"mappings"`
}

// Mappings holds the loaded override table.
type Mappings struct {
	entries []Entry
}

// New builds a Mappings directly from entries, without reading a file --
// useful for tests and for constructing an empty table.
func New(entries []Entry) *Mappings {
	return &Mappings{entries: entries}
}

// Load reads path (or returns an empty table if it doesn't exist).
func Load(path string) (*Mappings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Mappings{}, nil
		}
		return nil, err
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &Mappings{entries: f.Mappings}, nil
}

func (m *Mappings) findEntry(identity *inventory.Identity) *Entry {
	if identity == nil {
		return nil
	}
	vendor := strings.ToLower(deref(identity.Vendor))
	product := strings.ToLower(deref(identity.Product))

	for i := range m.entries {
		entry := &m.entries[i]
		// method is optional in a mapping entry: requiring an exact
		// method match caused a real "zlib" Win32 version resource
		// (method pe-version-resource) to miss a mapping written only
		// for the string-signature path, even though "zlib" is
		// unambiguous regardless of which method found it -- only scope
		// by method when the entry genuinely needs it.
		if entry.Match.Method != nil && *entry.Match.Method != identity.Method {
			continue
		}
		if entry.Match.Vendor != nil && strings.ToLower(*entry.Match.Vendor) != vendor {
			continue
		}
		if entry.Match.Product != nil && strings.ToLower(*entry.Match.Product) != product {
			continue
		}
		if entry.CPE.Vendor != "" && entry.CPE.Product != "" {
			return entry
		}
	}
	return nil
}

// Find returns (cpeVendor, cpeProduct, ok) for the first matching
// override, or ok=false if nothing in the table matches this identity.
func (m *Mappings) Find(identity *inventory.Identity) (vendor, product string, ok bool) {
	entry := m.findEntry(identity)
	if entry == nil {
		return "", "", false
	}
	return entry.CPE.Vendor, entry.CPE.Product, true
}

// VersionTransform returns (regexPattern, captureGroup, ok) for the
// matching entry's optional versionPattern/versionGroup.
//
// Exists because a Stage 1 identity's raw version string doesn't always
// match NVD's own version format for that product: FFmpeg reports its
// own git-tag-style version ("n7.1.1") where NVD's dictionary uses plain
// "7.1.1"; Qt's Win32 FILEVERSION resource is 4-part ("6.8.3.0") where
// NVD's dictionary is 3-part ("6.8.3").
func (m *Mappings) VersionTransform(identity *inventory.Identity) (pattern string, group int, ok bool) {
	entry := m.findEntry(identity)
	if entry == nil || entry.CPE.VersionPattern == "" {
		return "", 0, false
	}
	group = entry.CPE.VersionGroup
	if group == 0 {
		group = 1
	}
	return entry.CPE.VersionPattern, group, true
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
