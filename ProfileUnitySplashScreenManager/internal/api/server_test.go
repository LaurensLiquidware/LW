package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/liquidware/profileunity-splashscreen-manager/internal/logo"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/platform"
)

// fixture is a running server with a temporary store, exercised over real HTTP.
type fixture struct {
	t      *testing.T
	srv    *Server
	ts     *httptest.Server
	store  *logo.Store
	origin string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	store := &logo.Store{
		TargetDir: filepath.Join(root, "ClientNET"),
		DataDir:   filepath.Join(root, "Data"),
	}
	if err := os.MkdirAll(store.TargetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	ui := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>ui</title>")},
		"main.js":    &fstest.MapFile{Data: []byte("console.log(1)")},
	}

	licensePath := filepath.Join(root, "Spark_License.pdf")
	if err := os.WriteFile(licensePath, []byte("%PDF-1.4 fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := New(store, platform.New(), ui, Docs{
		LicensePDF: licensePath,
		SBOM:       filepath.Join(root, "bom.cdx.json"), // deliberately absent
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Cleanup)

	f := &fixture{t: t, srv: srv, store: store}
	f.ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.srv.Handler(f.origin).ServeHTTP(w, r)
	}))
	t.Cleanup(f.ts.Close)
	f.origin = f.ts.URL
	return f
}

// do performs a request with the API token attached.
func (f *fixture) do(method, path string, body any) *http.Response {
	f.t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, f.ts.URL+path, r)
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set(TokenHeader, f.srv.Token())
	resp, err := f.ts.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp
}

func (f *fixture) decode(resp *http.Response, into any) {
	f.t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		f.t.Fatalf("decoding response: %v", err)
	}
}

func (f *fixture) errorBody(resp *http.Response) string {
	f.t.Helper()
	defer resp.Body.Close()
	var e struct{ Error string }
	_ = json.NewDecoder(resp.Body).Decode(&e)
	return e.Error
}

// writePNG makes a real PNG on disk.
func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 0, G: 0x61, B: 0xA0, A: 0xFF})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- authentication and origin -----------------------------------------------

func TestAPIRequiresToken(t *testing.T) {
	f := newFixture(t)
	resp, err := f.ts.Client().Get(f.ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status without a token = %d, want 401", resp.StatusCode)
	}
}

func TestAPIRejectsWrongToken(t *testing.T) {
	f := newFixture(t)
	req, _ := http.NewRequest(http.MethodGet, f.ts.URL+"/api/state", nil)
	req.Header.Set(TokenHeader, strings.Repeat("0", 64))
	resp, err := f.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status with a wrong token = %d, want 401", resp.StatusCode)
	}
}

// TestAPIRejectsForeignOrigin: a page in the user's browser must not be able to
// drive the tool even if it discovers the port.
func TestAPIRejectsForeignOrigin(t *testing.T) {
	f := newFixture(t)
	for _, h := range []string{"Origin", "Referer"} {
		req, _ := http.NewRequest(http.MethodGet, f.ts.URL+"/api/state", nil)
		req.Header.Set(TokenHeader, f.srv.Token())
		req.Header.Set(h, "https://evil.example")
		resp, err := f.ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", h, resp.StatusCode)
		}
	}
}

func TestAPIAcceptsOwnOrigin(t *testing.T) {
	f := newFixture(t)
	req, _ := http.NewRequest(http.MethodGet, f.ts.URL+"/api/state", nil)
	req.Header.Set(TokenHeader, f.srv.Token())
	req.Header.Set("Origin", f.origin)
	resp, err := f.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status from our own origin = %d, want 200", resp.StatusCode)
	}
}

func TestNoCORSHeaders(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodGet, "/api/state", nil)
	defer resp.Body.Close()
	if v := resp.Header.Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("CORS header present: %q", v)
	}
}

// --- UI serving --------------------------------------------------------------

func TestUIServesIndexAndSetsCSP(t *testing.T) {
	f := newFixture(t)
	resp, err := f.ts.Client().Get(f.ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	csp := resp.Header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'none'", "script-src 'self'", "connect-src 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q missing %q", csp, want)
		}
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff header missing")
	}
}

// TestUIRefusesTraversal: the UI is served from an embedded FS, and a traversal
// attempt must not escape it.
func TestUIRefusesTraversal(t *testing.T) {
	f := newFixture(t)
	for _, p := range []string{
		"/../main.go",
		"/../../etc/passwd",
		"/..%2f..%2fetc%2fpasswd",
		"/%2e%2e/%2e%2e/etc/passwd",
	} {
		resp, err := f.ts.Client().Get(f.ts.URL + p)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if strings.Contains(string(body), "root:") || strings.Contains(string(body), "package main") {
			t.Errorf("%s leaked content outside the embedded UI", p)
		}
	}
}

// --- about -------------------------------------------------------------------

func TestAboutReportsDocumentPresence(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodGet, "/api/about", nil)
	var about AboutResponse
	f.decode(resp, &about)

	if about.Version == "" {
		t.Error("version is empty")
	}
	if !about.LicensePresent {
		t.Error("license PDF exists in the fixture but is reported absent")
	}
	if about.SBOMPresent {
		t.Error("SBOM is absent in the fixture but is reported present")
	}
	if !about.SearchEnabled || about.SearchHost == "" {
		t.Errorf("search should be enabled with a host, got enabled=%v host=%q",
			about.SearchEnabled, about.SearchHost)
	}
}

// --- state -------------------------------------------------------------------

func TestStateEmptyThenApplied(t *testing.T) {
	f := newFixture(t)

	resp := f.do(http.MethodGet, "/api/state", nil)
	var st StateResponse
	f.decode(resp, &st)
	if !st.TargetDirExists {
		t.Error("target dir should exist in the fixture")
	}
	if len(st.Live) != 0 {
		t.Errorf("live logos = %d, want 0", len(st.Live))
	}
	if st.History == nil {
		t.Error("history should marshal as an empty array, not null")
	}

	src := filepath.Join(t.TempDir(), "brand.png")
	writePNG(t, src, 300, 86)
	if _, err := f.srv.setPending(src, "browse", false); err != nil {
		t.Fatal(err)
	}

	resp = f.do(http.MethodPost, "/api/apply", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("apply failed: %s", f.errorBody(resp))
	}
	resp.Body.Close()

	resp = f.do(http.MethodGet, "/api/state", nil)
	f.decode(resp, &st)
	if len(st.Live) != 1 {
		t.Fatalf("live logos after apply = %d, want 1", len(st.Live))
	}
	if st.Live[0].Info == nil || st.Live[0].Info.Width != 300 {
		t.Errorf("live logo dimensions not reported: %+v", st.Live[0])
	}
	if st.Pending != nil {
		t.Error("pending should be cleared after apply")
	}
	if st.Current == nil || st.Current.OriginalName != "brand.png" {
		t.Errorf("current metadata = %+v, want brand.png", st.Current)
	}
}

func TestStateWarnsAboutStrayLogos(t *testing.T) {
	f := newFixture(t)
	for _, ext := range []string{".png", ".jpg"} {
		writePNG(t, filepath.Join(f.store.TargetDir, logo.LogoBaseName+ext), 300, 86)
	}
	resp := f.do(http.MethodGet, "/api/state", nil)
	var st StateResponse
	f.decode(resp, &st)
	if len(st.Warnings) == 0 {
		t.Fatal("expected a warning about multiple logo files")
	}
	if !strings.Contains(strings.Join(st.Warnings, " "), "2 logo files") {
		t.Errorf("warnings = %v, want one naming the count", st.Warnings)
	}
}

// --- apply -------------------------------------------------------------------

func TestApplyWithoutPendingIsRejected(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodPost, "/api/apply", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if msg := f.errorBody(resp); !strings.Contains(msg, "Nothing is being previewed") {
		t.Errorf("message = %q, want it to explain what to do", msg)
	}
}

// TestPendingRejectsNonImage: the client cannot get a non-image into Client.NET.
func TestPendingRejectsNonImage(t *testing.T) {
	f := newFixture(t)
	p := filepath.Join(t.TempDir(), "fake.png")
	if err := os.WriteFile(p, []byte("definitely not a png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.srv.setPending(p, "browse", false); err == nil {
		t.Fatal("setPending accepted a non-image")
	}
}

func TestPendingRejectsDisallowedExtension(t *testing.T) {
	f := newFixture(t)
	p := filepath.Join(t.TempDir(), "logo.webp")
	writePNG(t, p, 300, 86)
	if _, err := f.srv.setPending(p, "browse", false); err == nil {
		t.Fatal("setPending accepted .webp")
	}
}

// TestApplyIsNotAnArbitraryFileCopy is the reason apply takes no path: the client
// names nothing, so the API can never be used to copy an arbitrary file into
// Program Files.
func TestApplyIsNotAnArbitraryFileCopy(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodPost, "/api/apply", map[string]string{
		"path": "/etc/passwd", "source": "browse",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 -- a client-supplied path must be ignored", resp.StatusCode)
	}
	if live, _ := f.store.LiveLogos(); len(live) != 0 {
		t.Fatalf("a client-named path reached the target folder: %+v", live)
	}
}

func TestApplyRefusesTheLiveLogoAsItsOwnSource(t *testing.T) {
	f := newFixture(t)
	src := filepath.Join(t.TempDir(), "first.png")
	writePNG(t, src, 300, 86)
	if _, err := f.srv.setPending(src, "browse", false); err != nil {
		t.Fatal(err)
	}
	resp := f.do(http.MethodPost, "/api/apply", nil)
	resp.Body.Close()

	live, _ := f.store.LiveLogos()
	if len(live) != 1 {
		t.Fatalf("live = %d, want 1", len(live))
	}
	if _, err := f.srv.setPending(live[0].Path, "browse", false); err != nil {
		t.Fatal(err)
	}
	resp = f.do(http.MethodPost, "/api/apply", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if msg := f.errorBody(resp); !strings.Contains(msg, "already the live splash logo") {
		t.Errorf("message = %q", msg)
	}
	if live, _ = f.store.LiveLogos(); len(live) != 1 {
		t.Error("the live logo did not survive the refusal")
	}
}

// --- restore and history -----------------------------------------------------

func TestRestoreAndDeleteHistory(t *testing.T) {
	f := newFixture(t)
	for _, name := range []string{"one.png", "two.png"} {
		src := filepath.Join(t.TempDir(), name)
		writePNG(t, src, 300, 86)
		if _, err := f.srv.setPending(src, "browse", false); err != nil {
			t.Fatal(err)
		}
		resp := f.do(http.MethodPost, "/api/apply", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("apply %s: %s", name, f.errorBody(resp))
		}
		resp.Body.Close()
	}

	resp := f.do(http.MethodGet, "/api/state", nil)
	var st StateResponse
	f.decode(resp, &st)
	if len(st.History) != 1 {
		t.Fatalf("history = %d, want 1", len(st.History))
	}
	id := st.History[0].ID

	resp = f.do(http.MethodPost, "/api/restore", map[string]string{"id": id})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("restore: %s", f.errorBody(resp))
	}
	resp.Body.Close()

	resp = f.do(http.MethodGet, "/api/state", nil)
	f.decode(resp, &st)
	if st.Current == nil || st.Current.OriginalName != "one.png" {
		t.Errorf("current after restore = %+v, want one.png", st.Current)
	}

	// Delete every entry, including the last, and confirm none come back.
	for {
		resp = f.do(http.MethodGet, "/api/state", nil)
		f.decode(resp, &st)
		if len(st.History) == 0 {
			break
		}
		resp = f.do(http.MethodPost, "/api/history/delete", map[string]string{"id": st.History[0].ID})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("delete: %s", f.errorBody(resp))
		}
		resp.Body.Close()
	}
	resp = f.do(http.MethodGet, "/api/state", nil)
	f.decode(resp, &st)
	if len(st.History) != 0 {
		t.Fatalf("history came back after deleting everything: %d entries", len(st.History))
	}
}

func TestRestoreUnknownID(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodPost, "/api/restore", map[string]string{"id": "nope"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestRestoreWithoutIDIsRejected(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodPost, "/api/restore", map[string]string{"id": "   "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- image serving -----------------------------------------------------------

func TestImageServesPendingAndLive(t *testing.T) {
	f := newFixture(t)
	src := filepath.Join(t.TempDir(), "img.png")
	writePNG(t, src, 300, 86)
	if _, err := f.srv.setPending(src, "browse", false); err != nil {
		t.Fatal(err)
	}

	resp := f.do(http.MethodGet, "/api/image?kind=pending", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pending image status = %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if _, err := png.Decode(bytes.NewReader(b)); err != nil {
		t.Errorf("pending image is not a decodable PNG: %v", err)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "png") {
		t.Errorf("content type = %q, want png", ct)
	}

	resp = f.do(http.MethodPost, "/api/apply", nil)
	resp.Body.Close()

	resp = f.do(http.MethodGet, "/api/image?kind=live", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("live image status = %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestImageRejectsUnknownKind(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodGet, "/api/image?kind=../../etc/passwd", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestImageRequiresToken(t *testing.T) {
	f := newFixture(t)
	resp, err := f.ts.Client().Get(f.ts.URL + "/api/image?kind=live")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 -- image bytes must be token-guarded too", resp.StatusCode)
	}
}

// --- search and documents ----------------------------------------------------

func TestSearchRequiresQuery(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodPost, "/api/search", map[string]string{"query": "  "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if msg := f.errorBody(resp); !strings.Contains(msg, "Enter a search term") {
		t.Errorf("message = %q", msg)
	}
}

// TestSearchCanBeDisabled covers the air-gapped configuration.
func TestSearchCanBeDisabled(t *testing.T) {
	f := newFixture(t)
	old := SearchURLTemplate
	SearchURLTemplate = ""
	defer func() { SearchURLTemplate = old }()

	resp := f.do(http.MethodPost, "/api/search", map[string]string{"query": "liquidware logo"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when search is disabled", resp.StatusCode)
	}
	resp.Body.Close()

	resp = f.do(http.MethodGet, "/api/about", nil)
	var about AboutResponse
	f.decode(resp, &about)
	if about.SearchEnabled {
		t.Error("about still reports search as enabled")
	}
}

// TestOpenDocOnlyAcceptsKnownNames: the client names a document, never a path.
func TestOpenDocOnlyAcceptsKnownNames(t *testing.T) {
	f := newFixture(t)
	for _, which := range []string{"", "passwd", "../../etc/passwd", "C:\\Windows\\System32\\cmd.exe"} {
		resp := f.do(http.MethodPost, "/api/open-doc", map[string]string{"which": which})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("which=%q: status = %d, want 400", which, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestOpenDocReportsMissingFile(t *testing.T) {
	f := newFixture(t)
	// The SBOM is deliberately absent from the fixture.
	resp := f.do(http.MethodPost, "/api/open-doc", map[string]string{"which": "sbom"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if msg := f.errorBody(resp); !strings.Contains(msg, "Not found next to this tool") {
		t.Errorf("message = %q", msg)
	}
}

// --- method enforcement ------------------------------------------------------

func TestMutatingRoutesRequirePost(t *testing.T) {
	f := newFixture(t)
	for _, p := range []string{"/api/apply", "/api/restore", "/api/history/delete",
		"/api/search", "/api/preview-splash", "/api/open-doc", "/api/browse", "/api/clipboard"} {
		resp := f.do(http.MethodGet, p, nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: status = %d, want 405", p, resp.StatusCode)
		}
	}
}

// --- stub platform behaviour -------------------------------------------------

// TestWindowsOnlyRoutesFailCleanly: on a development machine the OS-specific
// operations are stubbed, and they must return a clean error rather than panic.
func TestWindowsOnlyRoutesFailCleanly(t *testing.T) {
	f := newFixture(t)
	for _, p := range []string{"/api/browse", "/api/clipboard"} {
		resp := f.do(http.MethodPost, p, nil)
		if resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			t.Errorf("%s unexpectedly succeeded on a stub platform", p)
			continue
		}
		if msg := f.errorBody(resp); msg == "" {
			t.Errorf("%s returned no error message", p)
		}
	}
}

func TestPreviewSplashReportsMissingExe(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodPost, "/api/preview-splash", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if msg := f.errorBody(resp); !strings.Contains(msg, "Preview executable not found") {
		t.Errorf("message = %q", msg)
	}
}

// --- pending lifecycle -------------------------------------------------------

func TestDiscardClearsPendingAndDeletesTemp(t *testing.T) {
	f := newFixture(t)
	tmp := filepath.Join(f.srv.tempDir, "clip.png")
	writePNG(t, tmp, 300, 86)
	if _, err := f.srv.setPending(tmp, "clipboard", true); err != nil {
		t.Fatal(err)
	}

	resp := f.do(http.MethodPost, "/api/discard", nil)
	resp.Body.Close()

	resp = f.do(http.MethodGet, "/api/state", nil)
	var st StateResponse
	f.decode(resp, &st)
	if st.Pending != nil {
		t.Error("pending not cleared")
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("a temporary clipboard file was left behind")
	}
}

func TestReplacingPendingDeletesThePreviousTemp(t *testing.T) {
	f := newFixture(t)
	first := filepath.Join(f.srv.tempDir, "a.png")
	writePNG(t, first, 300, 86)
	if _, err := f.srv.setPending(first, "clipboard", true); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(f.srv.tempDir, "b.png")
	writePNG(t, second, 300, 86)
	if _, err := f.srv.setPending(second, "clipboard", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Error("the superseded temporary file was not removed")
	}
}

func TestOversizedFileRejected(t *testing.T) {
	f := newFixture(t)
	p := filepath.Join(t.TempDir(), "huge.png")
	// A valid PNG header followed by padding past the cap, so size is what trips.
	writePNG(t, p, 8, 8)
	fh, err := os.OpenFile(p, os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := fh.Truncate(maxPendingBytes + 1); err != nil {
		t.Fatal(err)
	}
	fh.Close()

	_, err = f.srv.setPending(p, "browse", false)
	if err == nil {
		t.Fatal("an oversized file was accepted")
	}
	if !strings.Contains(err.Error(), "limit is") {
		t.Errorf("error = %q, want it to state the limit", err)
	}
}

// --- non-Latin filenames over the API ----------------------------------------

func TestNonLatinAndBracketedNamesOverTheAPI(t *testing.T) {
	names := []string{"日本語データ.png", "logo[1].png", "Ångström café.png", "Данные.png", "한국어.png"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			src := filepath.Join(t.TempDir(), name)
			writePNG(t, src, 300, 86)
			if _, err := f.srv.setPending(src, "browse", false); err != nil {
				t.Fatalf("setPending: %v", err)
			}
			resp := f.do(http.MethodPost, "/api/apply", nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("apply: %s", f.errorBody(resp))
			}
			resp.Body.Close()

			resp = f.do(http.MethodGet, "/api/state", nil)
			var st StateResponse
			f.decode(resp, &st)
			if st.Current == nil || st.Current.OriginalName != name {
				t.Errorf("current = %+v, want OriginalName %q", st.Current, name)
			}
			// And it must survive the JSON round trip byte for byte.
			raw, _ := json.Marshal(st.Current)
			if !strings.Contains(string(raw), name) && !strings.Contains(string(raw), jsonEscape(name)) {
				t.Errorf("name %q did not survive JSON encoding: %s", name, raw)
			}
		})
	}
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return strings.Trim(string(b), `"`)
}

func TestStateIsValidJSONWithEmptyCollections(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodGet, "/api/state", nil)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("state is not valid JSON: %v", err)
	}
	for _, key := range []string{"history", "live", "warnings"} {
		if m[key] == nil {
			t.Errorf("%s marshalled as null; it should be an empty array", key)
		}
	}
	_ = fmt.Sprint(m)
}
