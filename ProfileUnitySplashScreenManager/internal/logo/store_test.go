package logo

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newStore builds a Store over a throwaway directory tree.
func newStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	s := &Store{
		TargetDir: filepath.Join(root, "ClientNET"),
		DataDir:   filepath.Join(root, "Data"),
	}
	if err := os.MkdirAll(s.TargetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	return s
}

// writePNG writes a real PNG of the given size.
func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 0, G: 0x61, B: 0xA0, A: 0xFF})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- filenames that broke the PowerShell version ----------------------------

// TestApplyAwkwardFilenames covers the names that broke the PowerShell
// implementation. Go's file APIs do not glob, so bracketed names -- which is what
// browsers call repeat downloads, and this tool's whole workflow is
// search-then-download -- are handled with no special care. Non-Latin names are
// exercised too, per section 1 of the review checklist.
func TestApplyAwkwardFilenames(t *testing.T) {
	names := []string{
		"logo[1].png",
		"logo(2).png",
		"logo*star.png",
		"日本語データ.png",
		"简体中文.png",
		"한국어.png",
		"Данные.png",
		"Ångström café naïve.png",
		"会社ロゴ.png",
		"emoji 🚀 logo.png",
		"  leading and trailing  .png",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			src := filepath.Join(t.TempDir(), name)
			writePNG(t, src, RecommendedWidth, RecommendedHeight)

			applied, err := s.Apply(src)
			if err != nil {
				t.Fatalf("Apply(%q) = %v, want success", name, err)
			}
			if _, err := os.Stat(applied); err != nil {
				t.Fatalf("applied file missing: %v", err)
			}
			if got, want := filepath.Base(applied), LogoBaseName+".png"; got != want {
				t.Errorf("target name = %q, want %q", got, want)
			}
			meta := s.CurrentMeta()
			if meta == nil {
				t.Fatal("current metadata not written")
			}
			if meta.OriginalName != name {
				t.Errorf("OriginalName = %q, want %q", meta.OriginalName, name)
			}
		})
	}
}

// --- extension handling -----------------------------------------------------

func TestNormalizeExtension(t *testing.T) {
	// ProfileUnity recognises .jpg and .tif, never .jpeg or .tiff.
	cases := map[string]string{
		".jpeg": ".jpg", ".JPEG": ".jpg", ".tiff": ".tif", ".TIFF": ".tif",
		".png": ".png", ".PNG": ".png", ".bmp": ".bmp", ".gif": ".gif",
		".jpg": ".jpg", ".tif": ".tif",
	}
	for in, want := range cases {
		if got := NormalizeExtension(in); got != want {
			t.Errorf("NormalizeExtension(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApplyRejectsUnsupportedExtension(t *testing.T) {
	s := newStore(t)
	src := filepath.Join(t.TempDir(), "logo.webp")
	writePNG(t, src, 300, 86)
	if _, err := s.Apply(src); err == nil {
		t.Fatal("Apply accepted .webp, want rejection")
	}
}

// TestApplyRejectsNonImage is the content check: extension alone is not enough.
func TestApplyRejectsNonImage(t *testing.T) {
	s := newStore(t)
	src := filepath.Join(t.TempDir(), "not-really.png")
	if err := os.WriteFile(src, []byte("this is plain text, not a PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(src); err == nil {
		t.Fatal("Apply accepted a non-image named .png, want rejection")
	}
	if live, _ := s.LiveLogos(); len(live) != 0 {
		t.Fatalf("a rejected file reached the target folder: %+v", live)
	}
}

// --- the data-loss cases ----------------------------------------------------

// TestApplyLiveLogoAsItsOwnSource covers the case that destroyed the live logo in
// the PowerShell version: archiving deletes the live file before the copy reads it.
func TestApplyLiveLogoAsItsOwnSource(t *testing.T) {
	s := newStore(t)
	src := filepath.Join(t.TempDir(), "first.png")
	writePNG(t, src, 300, 86)
	applied, err := s.Apply(src)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Apply(applied)
	if err == nil {
		t.Fatal("applying the live logo to itself succeeded, want refusal")
	}
	if !strings.Contains(err.Error(), "already the live splash logo") {
		t.Errorf("error = %q, want it to name the cause", err)
	}
	live, _ := s.LiveLogos()
	if len(live) != 1 {
		t.Fatalf("live logo count = %d, want 1 (it must survive the refusal)", len(live))
	}
}

// TestArchiveAllStrays: more than one live logo means ProfileUnity may read a
// different file than the tool is managing, so every stray is archived.
func TestArchiveAllStrays(t *testing.T) {
	s := newStore(t)
	for _, ext := range []string{".png", ".jpg", ".bmp"} {
		writePNG(t, filepath.Join(s.TargetDir, LogoBaseName+ext), 300, 86)
	}
	live, err := s.LiveLogos()
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 3 {
		t.Fatalf("live logo count = %d, want 3", len(live))
	}

	src := filepath.Join(t.TempDir(), "replacement.png")
	writePNG(t, src, 300, 86)
	if _, err := s.Apply(src); err != nil {
		t.Fatal(err)
	}

	if live, _ = s.LiveLogos(); len(live) != 1 {
		t.Fatalf("live logo count after apply = %d, want 1", len(live))
	}
	m, err := s.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 3 {
		t.Fatalf("archived %d entries, want 3", len(m))
	}
	// No archived file may overwrite another.
	seen := map[string]bool{}
	for _, e := range m {
		if seen[e.StoredFile] {
			t.Errorf("duplicate StoredFile %q -- one archive overwrote another", e.StoredFile)
		}
		seen[e.StoredFile] = true
		if _, err := os.Stat(filepath.Join(s.HistoryDir(), e.StoredFile)); err != nil {
			t.Errorf("archived file missing: %v", err)
		}
	}
}

// TestDeleteLastHistoryEntry is the phantom-row bug: in the PowerShell version an
// empty write left the previous manifest on disk, so the deleted row came back
// while its file was gone.
func TestDeleteLastHistoryEntry(t *testing.T) {
	s := newStore(t)
	src := filepath.Join(t.TempDir(), "one.png")
	writePNG(t, src, 300, 86)
	if _, err := s.Apply(src); err != nil {
		t.Fatal(err)
	}
	src2 := filepath.Join(t.TempDir(), "two.png")
	writePNG(t, src2, 300, 86)
	if _, err := s.Apply(src2); err != nil {
		t.Fatal(err)
	}

	m, _ := s.Manifest()
	if len(m) != 1 {
		t.Fatalf("history = %d, want 1", len(m))
	}
	if err := s.DeleteHistory(m[0].ID); err != nil {
		t.Fatal(err)
	}
	m, _ = s.Manifest()
	if len(m) != 0 {
		t.Fatalf("history after deleting the last entry = %d, want 0", len(m))
	}

	// And it must be an empty JSON array on disk, not null and not stale content.
	b, err := os.ReadFile(s.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(b)); got != "[]" {
		t.Errorf("manifest on disk = %q, want %q", got, "[]")
	}
}

// --- restore ----------------------------------------------------------------

func TestRestoreRoundTrip(t *testing.T) {
	s := newStore(t)
	first := filepath.Join(t.TempDir(), "brand-a.png")
	writePNG(t, first, 300, 86)
	if _, err := s.Apply(first); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(t.TempDir(), "brand-b.jpeg")
	writePNG(t, second, 300, 86)
	if _, err := s.Apply(second); err != nil {
		t.Fatal(err)
	}

	// .jpeg must have landed as .jpg.
	live, _ := s.LiveLogos()
	if len(live) != 1 || live[0].Ext != ".jpg" {
		t.Fatalf("live logo = %+v, want a single .jpg", live)
	}

	m, _ := s.Manifest()
	if len(m) != 1 || m[0].OriginalName != "brand-a.png" {
		t.Fatalf("history = %+v, want brand-a.png", m)
	}

	restored, err := s.Restore(m[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(restored) != LogoBaseName+".png" {
		t.Errorf("restored to %q, want %q", filepath.Base(restored), LogoBaseName+".png")
	}
	if meta := s.CurrentMeta(); meta == nil || meta.OriginalName != "brand-a.png" {
		t.Errorf("current metadata = %+v, want brand-a.png", meta)
	}
	if live, _ = s.LiveLogos(); len(live) != 1 {
		t.Errorf("live logo count = %d, want exactly 1 after restore", len(live))
	}
}

func TestDeleteHistoryRemovesBackingFile(t *testing.T) {
	s := newStore(t)
	a := filepath.Join(t.TempDir(), "a.png")
	writePNG(t, a, 300, 86)
	if _, err := s.Apply(a); err != nil {
		t.Fatal(err)
	}
	b := filepath.Join(t.TempDir(), "b.png")
	writePNG(t, b, 300, 86)
	if _, err := s.Apply(b); err != nil {
		t.Fatal(err)
	}

	m, _ := s.Manifest()
	stored := filepath.Join(s.HistoryDir(), m[0].StoredFile)
	if _, err := os.Stat(stored); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteHistory(m[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stored); !os.IsNotExist(err) {
		t.Errorf("backing file still present after delete")
	}
}

// --- timestamps and the manifest format -------------------------------------

// TestTimestampsAreOffsetAware is section 2's "store UTC with offset" requirement,
// which the PowerShell version deferred. Go's time marshalling satisfies it.
func TestTimestampsAreOffsetAware(t *testing.T) {
	s := newStore(t)
	src := filepath.Join(t.TempDir(), "x.png")
	writePNG(t, src, 300, 86)
	if _, err := s.Apply(src); err != nil {
		t.Fatal(err)
	}
	src2 := filepath.Join(t.TempDir(), "y.png")
	writePNG(t, src2, 300, 86)
	if _, err := s.Apply(src2); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(s.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	var raw []map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	ts, _ := raw[0]["dateArchived"].(string)
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("dateArchived %q is not RFC 3339: %v", ts, err)
	}
	if !strings.ContainsAny(ts, "Z+") && strings.Count(ts, "-") < 3 {
		t.Errorf("dateArchived %q carries no timezone offset", ts)
	}
}

// TestReadsPowerShellManifest: the data directory is unchanged from the
// PowerShell version, so a machine that ran 0.2.0 has a manifest in its shape.
func TestReadsPowerShellManifest(t *testing.T) {
	s := newStore(t)
	legacy := `[
      {"Id":"aaa","StoredFile":"20260101-080000__old.png","OriginalName":"日本語データ.png","Extension":".png","DateArchived":"2026-01-01 08:00:00"},
      {"Id":"bbb","StoredFile":"20260601-093000__mid.png","OriginalName":"legacy-dot-separator.png","Extension":".png","DateArchived":"2026-06-01 09.30.00"},
      {"Id":"ccc","StoredFile":"20260822-143900__new.png","OriginalName":"newest.png","Extension":".png","DateArchived":"2026-08-22 14:39:00"},
      {"Id":"ddd","StoredFile":"garbage.png","OriginalName":"garbage.png","Extension":".png","DateArchived":"not a date at all"}
    ]`
	if err := os.WriteFile(s.ManifestPath(), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := s.Manifest()
	if err != nil {
		t.Fatalf("reading a PowerShell-era manifest failed: %v", err)
	}
	if len(m) != 4 {
		t.Fatalf("read %d entries, want 4", len(m))
	}
	if m[0].OriginalName != "newest.png" {
		t.Errorf("newest first: got %q", m[0].OriginalName)
	}
	if m[len(m)-1].OriginalName != "garbage.png" {
		t.Errorf("unparseable timestamp should sort last, got %q", m[len(m)-1].OriginalName)
	}
	// The CJK name must survive.
	var foundCJK bool
	for _, e := range m {
		if e.OriginalName == "日本語データ.png" {
			foundCJK = true
		}
	}
	if !foundCJK {
		t.Error("CJK original name lost when reading the legacy manifest")
	}
}

// TestReadsPowerShellSingleObjectManifest covers pre-0.2.0 PowerShell, which
// wrote a bare object rather than an array for a one-entry manifest.
func TestReadsPowerShellSingleObjectManifest(t *testing.T) {
	s := newStore(t)
	one := `{"Id":"solo","StoredFile":"s.png","OriginalName":"solo.png","Extension":".png","DateArchived":"2026-05-05 10:00:00"}`
	if err := os.WriteFile(s.ManifestPath(), []byte(one), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := s.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[0].OriginalName != "solo.png" {
		t.Fatalf("got %+v, want one entry named solo.png", m)
	}
}

func TestCorruptManifestDoesNotBlockStartup(t *testing.T) {
	s := newStore(t)
	if err := os.WriteFile(s.ManifestPath(), []byte("{{{ not json at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := s.Manifest()
	if err != nil {
		t.Fatalf("a corrupt manifest returned an error, want empty history: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("got %d entries from a corrupt manifest, want 0", len(m))
	}
}

// --- dimensions -------------------------------------------------------------

func TestInspectDimensions(t *testing.T) {
	dir := t.TempDir()
	exact := filepath.Join(dir, "exact.png")
	writePNG(t, exact, RecommendedWidth, RecommendedHeight)
	info, err := Inspect(exact)
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 300 || info.Height != 86 {
		t.Errorf("got %dx%d, want 300x86", info.Width, info.Height)
	}
	if !info.MatchesRecommended() {
		t.Error("300x86 should match the recommended size")
	}
	if info.Format != "png" {
		t.Errorf("format = %q, want png", info.Format)
	}

	off := filepath.Join(dir, "off.png")
	writePNG(t, off, 512, 128)
	info, err = Inspect(off)
	if err != nil {
		t.Fatal(err)
	}
	if info.MatchesRecommended() {
		t.Error("512x128 should not match the recommended size")
	}
}

func TestInspectRejectsGarbage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "garbage.png")
	if err := os.WriteFile(p, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(p); err == nil {
		t.Fatal("Inspect accepted garbage, want an error")
	}
}

// --- sanitising -------------------------------------------------------------

func TestSanitizeNameKeepsUnicodeLetters(t *testing.T) {
	cases := map[string]string{
		"会社ロゴ.png":      "会社ロゴ",
		"logo[1].png":   "logo_1",
		"Ångström.png":  "Ångström",
		"a/b\\c.png":    "a_b_c",
		"....png":       "logo",
		"Данные.png":    "Данные",
		"emoji 🚀 x.png": "emoji_x",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeNameTrimsWithoutSplittingRunes(t *testing.T) {
	long := strings.Repeat("日", 200) + ".png"
	got := sanitizeName(long)
	if r := []rune(got); len(r) != 60 {
		t.Fatalf("trimmed to %d runes, want 60", len(r))
	}
	if !utf8Valid(got) {
		t.Error("trimming split a multi-byte rune")
	}
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

// --- target folder missing --------------------------------------------------

func TestApplyWithMissingTargetDir(t *testing.T) {
	root := t.TempDir()
	s := &Store{TargetDir: filepath.Join(root, "nope"), DataDir: filepath.Join(root, "Data")}
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "x.png")
	writePNG(t, src, 300, 86)
	_, err := s.Apply(src)
	if err == nil {
		t.Fatal("Apply succeeded with a missing target directory")
	}
	if !strings.Contains(err.Error(), "target directory not found") {
		t.Errorf("error = %q, want it to name the missing target directory", err)
	}
	live, err := s.LiveLogos()
	if err != nil {
		t.Errorf("LiveLogos on a missing target dir should not error: %v", err)
	}
	if len(live) != 0 {
		t.Errorf("got %d live logos from a missing directory", len(live))
	}
}
