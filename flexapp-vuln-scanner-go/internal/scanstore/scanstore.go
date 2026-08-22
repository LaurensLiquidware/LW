// Package scanstore persists the dashboard's scan list across server
// restarts. The in-memory job registry (internal/httpapi/jobs.go) is
// memory-only and resets every time the process restarts -- fine for a
// page you reload while the process stays up, wrong for a tray-launched
// app someone quits and reopens.
//
// This stores just enough to render the dashboard row without
// re-reading every report file: id, package path, output dir, status,
// timestamp, and (once done) the package name, coverage percent,
// severity counts, and inventory path. Opening the full results view
// still re-reads the real files via pipeline.LoadExistingResult -- this
// store is a shortcut for the list, not a second source of truth.
//
// Ported from ../../../flexapp-vuln-scanner/desktop/recent_scans_store.py.
package scanstore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry is one persisted scan-list row.
type Entry struct {
	ID              string         `json:"id"`
	Kind            string         `json:"kind"` // "scan" (fresh Stage 1+2) or "refresh" (Stage 2 only)
	PackagePath     string         `json:"packagePath"`
	OutputDir       string         `json:"outputDir"`
	Status          string         `json:"status"`
	CreatedAt       string         `json:"createdAt"`
	Error           string         `json:"error,omitempty"`
	PackageName     string         `json:"packageName,omitempty"`
	CoveragePercent *float64       `json:"coveragePercent,omitempty"`
	SeverityCounts  map[string]int `json:"severityCounts,omitempty"`
	InventoryPath   string         `json:"inventoryPath,omitempty"`
}

// Store is a flat JSON array of entries, newest first, at Path. Every
// mutating call rewrites the whole file -- this list is expected to
// stay small (dozens to low hundreds of entries for a tool run
// interactively), so there's no real cost to keeping it simple.
type Store struct {
	Path string
	mu   sync.Mutex
}

// New creates a Store backed by path. The file is created on first
// write; reading before that returns an empty list.
func New(path string) *Store {
	return &Store{Path: path}
}

func (s *Store) read() ([]Entry, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		// A corrupt file is treated the same as an empty one -- this
		// store is a shortcut list, not the source of truth, so it's
		// safe to just start fresh rather than fail the whole app.
		return nil, nil
	}
	return entries, nil
}

func (s *Store) write(entries []Entry) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, data, 0o644)
}

// All returns every persisted entry, newest first.
func (s *Store) All() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.read()
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// Add creates and persists a new entry, inserted at the front.
func (s *Store) Add(id, packagePath, outputDir, kind string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := Entry{
		ID:          id,
		Kind:        kind,
		PackagePath: packagePath,
		OutputDir:   outputDir,
		Status:      "queued",
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	entries, err := s.read()
	if err != nil {
		return Entry{}, err
	}
	entries = append([]Entry{entry}, entries...)
	if err := s.write(entries); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Update applies mutate to the entry with the given id, if present, and
// persists the result. A no-op if no entry with that id exists.
func (s *Store) Update(id string, mutate func(*Entry)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.read()
	if err != nil {
		return err
	}
	for i := range entries {
		if entries[i].ID == id {
			mutate(&entries[i])
			break
		}
	}
	return s.write(entries)
}

// Remove deletes the entry with the given id, if present.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.read()
	if err != nil {
		return err
	}
	out := entries[:0]
	for _, e := range entries {
		if e.ID != id {
			out = append(out, e)
		}
	}
	return s.write(out)
}

// NewID generates a short random hex id, matching the job id format
// internal/httpapi/jobs.go already uses.
func NewID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
