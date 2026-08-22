package logo

import (
	"encoding/json"
	"fmt"
	"time"
)

// Entry is one archived logo in the history manifest.
//
// Timestamps are time.Time and marshal as RFC 3339 with an offset, which is what
// section 2 of the Sparks Tool Project Review Checklist asks for ("store UTC with
// offset; convert for display"). The PowerShell implementation stored local
// wall-clock strings with no offset and deferred this; Go gets it right for free.
type Entry struct {
	ID           string    `json:"id"`
	StoredFile   string    `json:"storedFile"`
	OriginalName string    `json:"originalName"`
	Extension    string    `json:"extension"`
	DateArchived time.Time `json:"dateArchived"`
}

// legacyEntry is the shape written by the PowerShell implementation (0.2.0 and
// earlier): PascalCase keys and a locale-formatted timestamp with no offset.
// Machines that ran the PowerShell tool have manifests in this shape, and the
// data directory is unchanged, so those entries are still read rather than
// silently dropped.
type legacyEntry struct {
	Id           string `json:"Id"`
	StoredFile   string `json:"StoredFile"`
	OriginalName string `json:"OriginalName"`
	Extension    string `json:"Extension"`
	DateArchived string `json:"DateArchived"`
}

// legacyTimeLayouts are tried in order for a PowerShell-era timestamp. The first
// is what 0.2.0 wrote under the invariant culture. The second and third cover
// manifests written by pre-0.2.0 builds on a machine whose locale used a
// different time separator -- the bug 0.2.0 fixed, whose data can still be on
// disk.
var legacyTimeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15.04.05",
	time.RFC3339,
}

// UnmarshalJSON accepts both the current shape and the PowerShell one.
func (e *Entry) UnmarshalJSON(b []byte) error {
	type current Entry
	var c current
	if err := json.Unmarshal(b, &c); err == nil && c.ID != "" {
		*e = Entry(c)
		return nil
	}

	var l legacyEntry
	if err := json.Unmarshal(b, &l); err != nil {
		return fmt.Errorf("history entry is not in a recognised format: %w", err)
	}
	if l.Id == "" && l.StoredFile == "" {
		return fmt.Errorf("history entry has neither an id nor a stored filename")
	}

	e.ID = l.Id
	e.StoredFile = l.StoredFile
	e.OriginalName = l.OriginalName
	e.Extension = l.Extension
	e.DateArchived = parseLegacyTime(l.DateArchived)
	return nil
}

// parseLegacyTime returns the zero time for anything unparseable, so one corrupt
// row sorts to the bottom instead of failing the whole history load. The
// PowerShell version's original defect was that an unparseable timestamp threw
// inside the grid's sort and took the entire list down.
func parseLegacyTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range legacyTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}
