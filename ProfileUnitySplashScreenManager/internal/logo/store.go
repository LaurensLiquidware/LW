// Package logo implements the ProfileUnity splash screen logo store: applying a
// logo, archiving the one it replaces, and restoring from history.
//
// Fixed by ProfileUnity itself, not by us (see the KB, article 12914471137293):
// the target filename is always client-custom-logo-300x86.<ext>, and the target
// folder is the ProfileUnity Client.NET directory.
//
// History and the manifest live outside Client.NET deliberately, so they survive
// a ProfileUnity client reinstall or upgrade.
package logo

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// LogoBaseName is the filename stem ProfileUnity looks for. Not our choice.
const LogoBaseName = "client-custom-logo-300x86"

// ErrSourceIsLiveLogo is returned when the file being applied is already the live
// logo. Archiving deletes the live file before the copy runs, so proceeding would
// leave the machine with no splash logo at all.
var ErrSourceIsLiveLogo = errors.New("that file is already the live splash logo")

// Store owns the target folder and the history directory.
type Store struct {
	// TargetDir is the ProfileUnity Client.NET folder that receives the logo.
	TargetDir string
	// DataDir holds the manifest and the archived history files.
	DataDir string
}

// HistoryDir is where archived logo files are kept.
func (s *Store) HistoryDir() string { return filepath.Join(s.DataDir, "History") }

// ManifestPath is the history index.
func (s *Store) ManifestPath() string { return filepath.Join(s.DataDir, "manifest.json") }

// currentMetaPath records what the live logo was originally called.
func (s *Store) currentMetaPath() string { return filepath.Join(s.DataDir, "current.json") }

// CurrentMeta describes the live logo.
type CurrentMeta struct {
	OriginalName   string    `json:"originalName"`
	StoredFileName string    `json:"storedFileName"`
	DateSet        time.Time `json:"dateSet"`
}

// legacyCurrentMeta is the PowerShell shape of current.json.
type legacyCurrentMeta struct {
	OriginalName   string `json:"OriginalName"`
	StoredFileName string `json:"StoredFileName"`
	DateSet        string `json:"DateSet"`
}

// UnmarshalJSON accepts both the current and the PowerShell shape.
func (m *CurrentMeta) UnmarshalJSON(b []byte) error {
	type current CurrentMeta
	var c current
	if err := json.Unmarshal(b, &c); err == nil && c.OriginalName != "" {
		*m = CurrentMeta(c)
		return nil
	}
	var l legacyCurrentMeta
	if err := json.Unmarshal(b, &l); err != nil {
		return err
	}
	m.OriginalName = l.OriginalName
	m.StoredFileName = l.StoredFileName
	m.DateSet = parseLegacyTime(l.DateSet)
	return nil
}

// EnsureDirs creates the data and history directories.
func (s *Store) EnsureDirs() error {
	for _, d := range []string{s.DataDir, s.HistoryDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("cannot create %s: %w", d, err)
		}
	}
	return nil
}

// LiveLogo is a client-custom-logo-300x86.* file present in the target folder.
type LiveLogo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Ext      string    `json:"ext"`
	Modified time.Time `json:"modified"`
	Size     int64     `json:"size"`
}

// LiveLogos returns every client-custom-logo-300x86.* file in the target folder,
// newest first.
//
// There is normally exactly one. More than one means an earlier archive-then-copy
// was interrupted, or a file was dropped in by hand -- and it matters, because
// ProfileUnity may read a different one than this tool is managing. Callers
// surface that rather than silently taking the first.
func (s *Store) LiveLogos() ([]LiveLogo, error) {
	entries, err := os.ReadDir(s.TargetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", s.TargetDir, err)
	}

	var found []LiveLogo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if !strings.EqualFold(strings.TrimSuffix(name, ext), LogoBaseName) {
			continue
		}
		if !IsAllowedExtension(ext) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		found = append(found, LiveLogo{
			Name:     name,
			Path:     filepath.Join(s.TargetDir, name),
			Ext:      strings.ToLower(ext),
			Modified: info.ModTime(),
			Size:     info.Size(),
		})
	}

	sort.Slice(found, func(i, j int) bool { return found[i].Modified.After(found[j].Modified) })
	return found, nil
}

// Manifest reads the history index. A missing, empty or unparseable manifest
// yields an empty history rather than an error, so the UI still loads.
func (s *Store) Manifest() ([]Entry, error) {
	b, err := os.ReadFile(s.ManifestPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("cannot read the history manifest: %w", err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return []Entry{}, nil
	}

	var entries []Entry
	if err := json.Unmarshal(b, &entries); err != nil {
		// A single object rather than an array: what PowerShell's ConvertTo-Json
		// wrote for a one-entry manifest before 0.2.0.
		var one Entry
		if err2 := json.Unmarshal(b, &one); err2 == nil {
			return []Entry{one}, nil
		}
		return []Entry{}, nil
	}
	if entries == nil {
		entries = []Entry{}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].DateArchived.After(entries[j].DateArchived)
	})
	return entries, nil
}

// saveManifest writes the history index atomically.
//
// The slice is normalised to non-nil so an empty history marshals as "[]" rather
// than "null". The PowerShell version had the equivalent bug in the other
// direction: piping an empty collection to ConvertTo-Json wrote nothing at all,
// leaving the previous contents on disk, so deleting the last history entry left
// a phantom row whose backing file was already gone.
func (s *Store) saveManifest(entries []Entry) error {
	if entries == nil {
		entries = []Entry{}
	}
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot encode the history manifest: %w", err)
	}
	return writeFileAtomic(s.ManifestPath(), append(b, '\n'))
}

// CurrentMeta reads the live logo's metadata, or nil if there is none.
func (s *Store) CurrentMeta() *CurrentMeta {
	b, err := os.ReadFile(s.currentMetaPath())
	if err != nil {
		return nil
	}
	var m CurrentMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return &m
}

func (s *Store) saveCurrentMeta(m CurrentMeta) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.currentMetaPath(), append(b, '\n'))
}

// safeNamePattern strips anything that is not a letter, digit, underscore or
// hyphen from an archived filename. Unicode letters are kept, so a logo named
// 会社ロゴ.png keeps a recognisable archived name.
var safeNamePattern = regexp.MustCompile(`[^\p{L}\p{N}_-]+`)

func sanitizeName(name string) string {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	s := safeNamePattern.ReplaceAllString(stem, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "logo"
	}
	if len(s) > 60 {
		s = trimToRunes(s, 60)
	}
	return s
}

// trimToRunes cuts a string to at most n runes, never mid-rune.
func trimToRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func randomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ArchiveLive moves every live logo file into history and records it in the
// manifest, then removes it from the target folder. Returns the entries added.
//
// Every stray is archived, not just the newest, so a file left behind by an
// interrupted run is preserved rather than deleted unrecorded.
func (s *Store) ArchiveLive() ([]Entry, error) {
	live, err := s.LiveLogos()
	if err != nil {
		return nil, err
	}
	if len(live) == 0 {
		return nil, nil
	}
	if err := s.EnsureDirs(); err != nil {
		return nil, err
	}

	meta := s.CurrentMeta()
	manifest, err := s.Manifest()
	if err != nil {
		return nil, err
	}

	var added []Entry
	now := time.Now()
	for i, l := range live {
		originalName := "unknown-original" + l.Ext
		// Only the newest file can be the one current.json describes.
		if i == 0 && meta != nil && meta.OriginalName != "" {
			originalName = meta.OriginalName
		}

		suffix, err := randomToken(4)
		if err != nil {
			return nil, err
		}
		stored := fmt.Sprintf("%s__%s__%s%s",
			now.Format("20060102-150405"), sanitizeName(originalName), suffix, l.Ext)

		if err := copyFile(l.Path, filepath.Join(s.HistoryDir(), stored)); err != nil {
			return nil, fmt.Errorf("cannot archive %s: %w", l.Name, err)
		}

		id, err := randomToken(16)
		if err != nil {
			return nil, err
		}
		entry := Entry{
			ID:           id,
			StoredFile:   stored,
			OriginalName: originalName,
			Extension:    l.Ext,
			DateArchived: now,
		}
		manifest = append(manifest, entry)
		added = append(added, entry)

		if err := os.Remove(l.Path); err != nil {
			return nil, fmt.Errorf("archived %s but could not remove it from the target folder: %w", l.Name, err)
		}
	}

	if err := s.saveManifest(manifest); err != nil {
		return nil, err
	}
	_ = os.Remove(s.currentMetaPath())
	return added, nil
}

// Apply makes sourcePath the live splash logo, archiving whatever it replaces.
// It returns the path written.
func (s *Store) Apply(sourcePath string) (string, error) {
	if _, err := os.Stat(s.TargetDir); err != nil {
		return "", fmt.Errorf("target directory not found: %s. Is the ProfileUnity client installed here?", s.TargetDir)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("source file not found: %s", sourcePath)
	}
	if info.IsDir() {
		return "", fmt.Errorf("source is a directory, not a file: %s", sourcePath)
	}

	ext := strings.ToLower(filepath.Ext(sourcePath))
	if !IsAllowedExtension(ext) {
		return "", fmt.Errorf("unsupported file type %q. Allowed: %s", ext, strings.Join(AllowedExtensions, ", "))
	}
	// Content check, not just the extension.
	if _, err := Inspect(sourcePath); err != nil {
		return "", err
	}

	// Archiving deletes the live logo before the copy runs, so applying the live
	// logo to itself would destroy it.
	live, err := s.LiveLogos()
	if err != nil {
		return "", err
	}
	for _, l := range live {
		same, err := sameFile(l.Path, sourcePath)
		if err == nil && same {
			return "", ErrSourceIsLiveLogo
		}
	}

	if _, err := s.ArchiveLive(); err != nil {
		return "", err
	}

	target := filepath.Join(s.TargetDir, LogoBaseName+NormalizeExtension(ext))
	if err := copyFile(sourcePath, target); err != nil {
		return "", fmt.Errorf("cannot write the logo to %s: %w", target, err)
	}

	if err := s.saveCurrentMeta(CurrentMeta{
		OriginalName:   filepath.Base(sourcePath),
		StoredFileName: filepath.Base(target),
		DateSet:        time.Now(),
	}); err != nil {
		return "", err
	}
	return target, nil
}

// Restore makes the history entry with the given id live again, archiving
// whatever it replaces.
func (s *Store) Restore(id string) (string, error) {
	manifest, err := s.Manifest()
	if err != nil {
		return "", err
	}
	var entry *Entry
	for i := range manifest {
		if manifest[i].ID == id {
			entry = &manifest[i]
			break
		}
	}
	if entry == nil {
		return "", fmt.Errorf("no history entry with id %q", id)
	}

	src := filepath.Join(s.HistoryDir(), entry.StoredFile)
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("history file is missing on disk: %s", entry.StoredFile)
	}
	if _, err := os.Stat(s.TargetDir); err != nil {
		return "", fmt.Errorf("target directory not found: %s", s.TargetDir)
	}

	if _, err := s.ArchiveLive(); err != nil {
		return "", err
	}

	target := filepath.Join(s.TargetDir, LogoBaseName+NormalizeExtension(entry.Extension))
	if err := copyFile(src, target); err != nil {
		return "", fmt.Errorf("cannot restore to %s: %w", target, err)
	}
	if err := s.saveCurrentMeta(CurrentMeta{
		OriginalName:   entry.OriginalName,
		StoredFileName: filepath.Base(target),
		DateSet:        time.Now(),
	}); err != nil {
		return "", err
	}
	return target, nil
}

// DeleteHistory removes a history entry and its backing file.
func (s *Store) DeleteHistory(id string) error {
	manifest, err := s.Manifest()
	if err != nil {
		return err
	}
	kept := make([]Entry, 0, len(manifest))
	var removed *Entry
	for i := range manifest {
		if manifest[i].ID == id {
			removed = &manifest[i]
			continue
		}
		kept = append(kept, manifest[i])
	}
	if removed == nil {
		return fmt.Errorf("no history entry with id %q", id)
	}
	if err := s.saveManifest(kept); err != nil {
		return err
	}
	if removed.StoredFile != "" {
		_ = os.Remove(filepath.Join(s.HistoryDir(), removed.StoredFile))
	}
	return nil
}

// HistoryFilePath returns the on-disk path of a history entry's file, for preview.
func (s *Store) HistoryFilePath(id string) (string, error) {
	manifest, err := s.Manifest()
	if err != nil {
		return "", err
	}
	for _, e := range manifest {
		if e.ID == id {
			p := filepath.Join(s.HistoryDir(), e.StoredFile)
			if _, err := os.Stat(p); err != nil {
				return "", fmt.Errorf("history file is missing on disk: %s", e.StoredFile)
			}
			return p, nil
		}
	}
	return "", fmt.Errorf("no history entry with id %q", id)
}

// --- file helpers -----------------------------------------------------------

// writeFileAtomic writes via a temporary file in the same directory and renames,
// so a crash mid-write cannot truncate the manifest.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Windows will not rename onto an existing file.
	_ = os.Remove(path)
	return os.Rename(tmpName, path)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// sameFile reports whether two paths refer to the same file on disk, comparing
// identity rather than the path text, so a different spelling of the same file is
// still caught.
func sameFile(a, b string) (bool, error) {
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ai, bi), nil
}
