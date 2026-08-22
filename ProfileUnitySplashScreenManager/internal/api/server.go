// Package api serves the JSON API the Angular UI talks to, plus the embedded UI
// itself, on a loopback-only listener.
//
// Security posture, since an HTTP server on a machine that runs elevated is worth
// being deliberate about:
//
//   - The listener binds 127.0.0.1 on an ephemeral port. Nothing is reachable
//     from the network.
//   - Every /api/ request must present a per-run token, generated at startup and
//     handed to the page by the WebView's init script. Another local process
//     cannot guess it, and it never appears in a URL.
//   - Requests carrying a cross-origin Origin or Referer are refused, so a web
//     page in the user's browser cannot drive the tool even if it learns the port.
//   - No CORS headers are ever sent.
//   - The client cannot name a file to apply. Browse and clipboard import record
//     a pending file server-side and the client applies "the pending file", so the
//     API is never a "copy any path into Program Files" primitive.
package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liquidware/profileunity-splashscreen-manager/internal/logo"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/platform"
	"github.com/liquidware/profileunity-splashscreen-manager/internal/version"
)

// TokenHeader carries the per-run API token.
const TokenHeader = "X-PSM-Token"

// maxPendingBytes caps what will be accepted as a candidate logo. A splash logo
// is 300x86; anything above this is a mistake, and refusing early keeps a huge
// file from being read into memory for preview.
const maxPendingBytes = 32 << 20 // 32 MiB

// SearchURLTemplate builds the image-search URL. %s is the URL-encoded query.
// Configurable so an air-gapped or policy-restricted site can point it elsewhere,
// or disable it entirely with an empty string.
var SearchURLTemplate = "https://www.google.com/search?tbm=isch&q=%s"

// Docs are the compliance documents that ship beside the executable.
type Docs struct {
	LicensePDF string
	SBOM       string
	Notices    string
}

// Pending is a file that has been previewed but not applied.
type Pending struct {
	Path   string         `json:"path"`
	Name   string         `json:"name"`
	Info   logo.ImageInfo `json:"info"`
	Source string         `json:"source"` // "browse" or "clipboard"
	Temp   bool           `json:"-"`      // delete on replacement/exit
}

// Server owns the HTTP surface.
type Server struct {
	store *logo.Store
	plat  platform.Platform
	ui    fs.FS
	docs  Docs
	token string

	tempDir string

	mu      sync.Mutex
	pending *Pending
	temps   []string
}

// New builds a Server. ui is the embedded Angular build output.
func New(store *logo.Store, plat platform.Platform, ui fs.FS, docs Docs) (*Server, error) {
	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		return nil, fmt.Errorf("cannot generate the API token: %w", err)
	}
	tempDir, err := os.MkdirTemp("", "psm-preview-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a temporary directory: %w", err)
	}
	return &Server{
		store:   store,
		plat:    plat,
		ui:      ui,
		docs:    docs,
		token:   hex.EncodeToString(tok),
		tempDir: tempDir,
	}, nil
}

// Token is the per-run API token.
func (s *Server) Token() string { return s.token }

// Cleanup removes temporary preview files. Called on shutdown.
func (s *Server) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.temps {
		_ = os.Remove(p)
	}
	s.temps = nil
	_ = os.RemoveAll(s.tempDir)
}

// Listen opens the loopback listener.
func (s *Server) Listen() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

// Handler builds the router.
func (s *Server) Handler(origin string) http.Handler {
	mux := http.NewServeMux()

	api := func(path string, h func(http.ResponseWriter, *http.Request) (any, error)) {
		mux.Handle(path, s.guard(origin, jsonHandler(h)))
	}

	api("/api/about", s.handleAbout)
	api("/api/state", s.handleState)
	api("/api/browse", s.handleBrowse)
	api("/api/clipboard", s.handleClipboard)
	api("/api/apply", s.handleApply)
	api("/api/restore", s.handleRestore)
	api("/api/history/delete", s.handleDeleteHistory)
	api("/api/search", s.handleSearch)
	api("/api/preview-splash", s.handlePreviewSplash)
	api("/api/open-doc", s.handleOpenDoc)
	api("/api/discard", s.handleDiscard)

	// Image bytes for the preview panes. Guarded like any other API route; the UI
	// fetches it with the token header and turns the result into a blob URL, so no
	// token ever appears in a URL.
	mux.Handle("/api/image", s.guard(origin, http.HandlerFunc(s.handleImage)))

	mux.Handle("/", s.uiHandler())
	return mux
}

// guard enforces the token and rejects cross-origin callers.
func (s *Server) guard(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A page in the user's browser must not be able to drive the tool, even if
		// it discovers the port. Same-origin requests from our own WebView send
		// either no Origin or exactly ours.
		if o := r.Header.Get("Origin"); o != "" && o != origin {
			http.Error(w, "cross-origin requests are refused", http.StatusForbidden)
			return
		}
		if ref := r.Header.Get("Referer"); ref != "" && !strings.HasPrefix(ref, origin+"/") && ref != origin {
			http.Error(w, "cross-origin requests are refused", http.StatusForbidden)
			return
		}

		got := r.Header.Get(TokenHeader)
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			http.Error(w, "missing or invalid API token", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// apiError carries an HTTP status alongside a message.
type apiError struct {
	status int
	msg    string
}

func (e apiError) Error() string { return e.msg }

func badRequest(format string, args ...any) error {
	return apiError{http.StatusBadRequest, fmt.Sprintf(format, args...)}
}

// jsonHandler adapts a handler that returns a value or an error.
func jsonHandler(h func(http.ResponseWriter, *http.Request) (any, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, err := h(w, r)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err != nil {
			status := http.StatusInternalServerError
			var ae apiError
			if errors.As(err, &ae) && ae.status != 0 {
				status = ae.status
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if v == nil {
			v = map[string]bool{"ok": true}
		}
		_ = json.NewEncoder(w).Encode(v)
	})
}

// uiHandler serves the embedded Angular build, falling back to index.html so
// client-side routing works.
func (s *Server) uiHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The UI is embedded in the binary and never fetches anything external, so
		// a restrictive CSP costs nothing and closes off script injection.
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' blob: data:; font-src 'self'; connect-src 'self'; "+
				"base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")

		name := strings.TrimPrefix(path(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		b, err := fs.ReadFile(s.ui, name)
		if err != nil {
			b, err = fs.ReadFile(s.ui, "index.html")
			if err != nil {
				http.Error(w, "the user interface is missing from this build", http.StatusInternalServerError)
				return
			}
			name = "index.html"
		}
		if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		if name == "index.html" {
			w.Header().Set("Cache-Control", "no-store")
		}
		_, _ = w.Write(b)
	})
}

// path normalises a request path, refusing traversal.
func path(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	cleaned := filepath.ToSlash(filepath.Clean("/" + strings.TrimPrefix(p, "/")))
	return cleaned
}

// --- handlers ---------------------------------------------------------------

// AboutResponse is what the About dialog shows.
type AboutResponse struct {
	Product        string `json:"product"`
	Version        string `json:"version"`
	Company        string `json:"company"`
	TargetDir      string `json:"targetDir"`
	DataDir        string `json:"dataDir"`
	ExeDir         string `json:"exeDir"`
	Elevated       bool   `json:"elevated"`
	WebViewVersion string `json:"webViewVersion"`
	LicensePresent bool   `json:"licensePresent"`
	SBOMPresent    bool   `json:"sbomPresent"`
	NoticesPresent bool   `json:"noticesPresent"`
	LicensePath    string `json:"licensePath"`
	SBOMPath       string `json:"sbomPath"`
	SearchEnabled  bool   `json:"searchEnabled"`
	SearchHost     string `json:"searchHost"`
}

func (s *Server) handleAbout(_ http.ResponseWriter, _ *http.Request) (any, error) {
	elevated, _ := s.plat.IsElevated()
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	return AboutResponse{
		Product:        version.ProductName,
		Version:        version.AppVersion,
		Company:        version.Company,
		TargetDir:      s.store.TargetDir,
		DataDir:        s.store.DataDir,
		ExeDir:         exeDir,
		Elevated:       elevated,
		WebViewVersion: s.plat.WebViewRuntimeVersion(),
		LicensePresent: fileExists(s.docs.LicensePDF),
		SBOMPresent:    fileExists(s.docs.SBOM),
		NoticesPresent: fileExists(s.docs.Notices),
		LicensePath:    s.docs.LicensePDF,
		SBOMPath:       s.docs.SBOM,
		SearchEnabled:  SearchURLTemplate != "",
		SearchHost:     searchHost(),
	}, nil
}

func searchHost() string {
	if SearchURLTemplate == "" {
		return ""
	}
	u, err := url.Parse(fmt.Sprintf(SearchURLTemplate, "x"))
	if err != nil {
		return ""
	}
	return u.Host
}

// LiveLogoView is a live logo plus its decoded dimensions.
type LiveLogoView struct {
	logo.LiveLogo
	Info      *logo.ImageInfo `json:"info"`
	InfoError string          `json:"infoError,omitempty"`
}

// HistoryView is a history entry as the UI shows it.
type HistoryView struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"originalName"`
	Extension    string    `json:"extension"`
	DateArchived time.Time `json:"dateArchived"`
	FileMissing  bool      `json:"fileMissing"`
}

// StateResponse is the whole UI state in one call.
type StateResponse struct {
	TargetDir       string            `json:"targetDir"`
	TargetDirExists bool              `json:"targetDirExists"`
	Live            []LiveLogoView    `json:"live"`
	Current         *logo.CurrentMeta `json:"current"`
	Pending         *Pending          `json:"pending"`
	History         []HistoryView     `json:"history"`
	PreviewExe      string            `json:"previewExe"`
	PreviewExists   bool              `json:"previewExists"`
	Recommended     [2]int            `json:"recommended"`
	Warnings        []string          `json:"warnings"`
}

func (s *Server) handleState(_ http.ResponseWriter, _ *http.Request) (any, error) {
	resp := StateResponse{
		TargetDir:   s.store.TargetDir,
		Recommended: [2]int{logo.RecommendedWidth, logo.RecommendedHeight},
		Warnings:    []string{},
		History:     []HistoryView{},
		Live:        []LiveLogoView{},
	}
	if fi, err := os.Stat(s.store.TargetDir); err == nil && fi.IsDir() {
		resp.TargetDirExists = true
	}

	live, err := s.store.LiveLogos()
	if err != nil {
		return nil, err
	}
	for _, l := range live {
		v := LiveLogoView{LiveLogo: l}
		if info, err := logo.Inspect(l.Path); err == nil {
			v.Info = &info
		} else {
			v.InfoError = err.Error()
		}
		resp.Live = append(resp.Live, v)
	}
	if len(live) > 1 {
		names := make([]string, 0, len(live))
		for _, l := range live {
			names = append(names, l.Name)
		}
		resp.Warnings = append(resp.Warnings, fmt.Sprintf(
			"%d logo files are present in the target folder (%s). ProfileUnity may not use the one shown here; applying or restoring a logo will archive all of them.",
			len(live), strings.Join(names, ", ")))
	}
	if !resp.TargetDirExists {
		resp.Warnings = append(resp.Warnings,
			"Target directory not found. Is the ProfileUnity client installed here?")
	}
	if elevated, err := s.plat.IsElevated(); err == nil && !elevated {
		resp.Warnings = append(resp.Warnings,
			"Not running elevated. Writing to Program Files will fail; restart the tool as an administrator.")
	}

	resp.Current = s.store.CurrentMeta()

	history, err := s.store.Manifest()
	if err != nil {
		return nil, err
	}
	for _, e := range history {
		_, statErr := os.Stat(filepath.Join(s.store.HistoryDir(), e.StoredFile))
		resp.History = append(resp.History, HistoryView{
			ID:           e.ID,
			OriginalName: e.OriginalName,
			Extension:    e.Extension,
			DateArchived: e.DateArchived,
			FileMissing:  statErr != nil,
		})
	}

	resp.PreviewExe = filepath.Join(s.store.TargetDir, "LwL.ProfileUnity.Client.CtxInit.exe")
	resp.PreviewExists = fileExists(resp.PreviewExe)

	s.mu.Lock()
	resp.Pending = s.pending
	s.mu.Unlock()
	return resp, nil
}

// setPending records a previewed-but-unapplied file, validating it first.
func (s *Server) setPending(path, source string, temp bool) (*Pending, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, badRequest("file not found: %s", path)
	}
	if fi.IsDir() {
		return nil, badRequest("that is a directory, not a file")
	}
	if fi.Size() > maxPendingBytes {
		return nil, badRequest("that file is %d MiB; the limit is %d MiB",
			fi.Size()>>20, maxPendingBytes>>20)
	}
	ext := strings.ToLower(filepath.Ext(path))
	if !logo.IsAllowedExtension(ext) {
		return nil, badRequest("unsupported file type %q. Allowed: %s",
			ext, strings.Join(logo.AllowedExtensions, ", "))
	}
	// Decode before accepting: a file named .png that is not a PNG must not reach
	// Client.NET, where ProfileUnity would render no logo at all.
	info, err := logo.Inspect(path)
	if err != nil {
		return nil, badRequest("%s", err.Error())
	}

	p := &Pending{Path: path, Name: filepath.Base(path), Info: info, Source: source, Temp: temp}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearPendingLocked()
	s.pending = p
	if temp {
		s.temps = append(s.temps, path)
	}
	return p, nil
}

// clearPendingLocked drops the pending file, deleting it if we created it.
func (s *Server) clearPendingLocked() {
	if s.pending != nil && s.pending.Temp {
		_ = os.Remove(s.pending.Path)
	}
	s.pending = nil
}

func (s *Server) handleBrowse(_ http.ResponseWriter, r *http.Request) (any, error) {
	if r.Method != http.MethodPost {
		return nil, apiError{http.StatusMethodNotAllowed, "POST required"}
	}
	path, err := s.plat.OpenFileDialog("Select splash screen logo image", platform.FileFilter{
		Label:    "Image files",
		Patterns: patternsFor(logo.AllowedExtensions),
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return map[string]any{"cancelled": true}, nil
	}
	return s.setPending(path, "browse", false)
}

func patternsFor(exts []string) []string {
	out := make([]string, 0, len(exts))
	for _, e := range exts {
		out = append(out, "*"+e)
	}
	return out
}

func (s *Server) handleClipboard(_ http.ResponseWriter, r *http.Request) (any, error) {
	if r.Method != http.MethodPost {
		return nil, apiError{http.StatusMethodNotAllowed, "POST required"}
	}
	data, err := s.plat.ClipboardImagePNG()
	if err != nil {
		if errors.Is(err, platform.ErrNoClipboardImage) {
			return nil, badRequest("No image found on the clipboard. Right-click an image in your browser and choose Copy image first.")
		}
		return nil, err
	}

	name := fmt.Sprintf("clipboard-%s.png", time.Now().UTC().Format("20060102-150405"))
	tmp := filepath.Join(s.tempDir, name)
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return nil, fmt.Errorf("cannot save the clipboard image: %w", err)
	}
	return s.setPending(tmp, "clipboard", true)
}

func (s *Server) handleDiscard(_ http.ResponseWriter, _ *http.Request) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearPendingLocked()
	return nil, nil
}

func (s *Server) handleApply(_ http.ResponseWriter, r *http.Request) (any, error) {
	if r.Method != http.MethodPost {
		return nil, apiError{http.StatusMethodNotAllowed, "POST required"}
	}
	s.mu.Lock()
	pending := s.pending
	s.mu.Unlock()
	if pending == nil {
		return nil, badRequest("Nothing is being previewed. Browse for a file or import one from the clipboard first.")
	}

	applied, err := s.store.Apply(pending.Path)
	if err != nil {
		if errors.Is(err, logo.ErrSourceIsLiveLogo) {
			return nil, badRequest("That file is already the live splash logo -- nothing to apply.")
		}
		return nil, err
	}

	s.mu.Lock()
	s.clearPendingLocked()
	s.mu.Unlock()
	return map[string]string{"applied": applied}, nil
}

type idRequest struct {
	ID string `json:"id"`
}

func decodeID(r *http.Request) (string, error) {
	var req idRequest
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(&req); err != nil {
		return "", badRequest("could not read the request: %v", err)
	}
	if strings.TrimSpace(req.ID) == "" {
		return "", badRequest("no history entry was selected")
	}
	return req.ID, nil
}

func (s *Server) handleRestore(_ http.ResponseWriter, r *http.Request) (any, error) {
	if r.Method != http.MethodPost {
		return nil, apiError{http.StatusMethodNotAllowed, "POST required"}
	}
	id, err := decodeID(r)
	if err != nil {
		return nil, err
	}
	restored, err := s.store.Restore(id)
	if err != nil {
		return nil, badRequest("%s", err.Error())
	}
	s.mu.Lock()
	s.clearPendingLocked()
	s.mu.Unlock()
	return map[string]string{"restored": restored}, nil
}

func (s *Server) handleDeleteHistory(_ http.ResponseWriter, r *http.Request) (any, error) {
	if r.Method != http.MethodPost {
		return nil, apiError{http.StatusMethodNotAllowed, "POST required"}
	}
	id, err := decodeID(r)
	if err != nil {
		return nil, err
	}
	if err := s.store.DeleteHistory(id); err != nil {
		return nil, badRequest("%s", err.Error())
	}
	return nil, nil
}

type searchRequest struct {
	Query string `json:"query"`
}

func (s *Server) handleSearch(_ http.ResponseWriter, r *http.Request) (any, error) {
	if r.Method != http.MethodPost {
		return nil, apiError{http.StatusMethodNotAllowed, "POST required"}
	}
	if SearchURLTemplate == "" {
		return nil, badRequest("Image search is disabled in this build.")
	}
	var req searchRequest
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(&req); err != nil {
		return nil, badRequest("could not read the request: %v", err)
	}
	q := strings.TrimSpace(req.Query)
	if q == "" {
		return nil, badRequest("Enter a search term first.")
	}
	target := fmt.Sprintf(SearchURLTemplate, url.QueryEscape(q))
	if err := s.plat.OpenInBrowser(target); err != nil {
		return nil, fmt.Errorf("could not open your browser: %w", err)
	}
	return map[string]string{"opened": target}, nil
}

func (s *Server) handlePreviewSplash(_ http.ResponseWriter, r *http.Request) (any, error) {
	if r.Method != http.MethodPost {
		return nil, apiError{http.StatusMethodNotAllowed, "POST required"}
	}
	exe := filepath.Join(s.store.TargetDir, "LwL.ProfileUnity.Client.CtxInit.exe")
	if !fileExists(exe) {
		return nil, badRequest("Preview executable not found: %s", exe)
	}
	if err := s.plat.LaunchDetached(exe); err != nil {
		return nil, fmt.Errorf("could not launch the splash preview: %w", err)
	}
	return map[string]string{"launched": exe}, nil
}

type openDocRequest struct {
	Which string `json:"which"`
}

func (s *Server) handleOpenDoc(_ http.ResponseWriter, r *http.Request) (any, error) {
	if r.Method != http.MethodPost {
		return nil, apiError{http.StatusMethodNotAllowed, "POST required"}
	}
	var req openDocRequest
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(&req); err != nil {
		return nil, badRequest("could not read the request: %v", err)
	}
	// A fixed allow-list: the client names which document, never a path.
	var target string
	switch req.Which {
	case "license":
		target = s.docs.LicensePDF
	case "sbom":
		target = s.docs.SBOM
	case "notices":
		target = s.docs.Notices
	default:
		return nil, badRequest("unknown document %q", req.Which)
	}
	if !fileExists(target) {
		return nil, badRequest("Not found next to this tool: %s", target)
	}
	if err := s.plat.LaunchDetached(target); err != nil {
		// Documents are opened by association, so fall back to the shell.
		if err2 := s.plat.OpenInBrowser(target); err2 != nil {
			return nil, fmt.Errorf("could not open %s: %w", filepath.Base(target), err)
		}
	}
	return map[string]string{"opened": target}, nil
}

// handleImage serves image bytes for the preview panes. The UI fetches this with
// the token header and converts the response to a blob URL, so no token is ever
// placed in a URL and no file path comes from the client.
func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	var path string

	switch kind {
	case "pending":
		s.mu.Lock()
		if s.pending != nil {
			path = s.pending.Path
		}
		s.mu.Unlock()
	case "live":
		live, err := s.store.LiveLogos()
		if err == nil && len(live) > 0 {
			path = live[0].Path
		}
	case "history":
		p, err := s.store.HistoryFilePath(r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		path = p
	default:
		http.Error(w, "unknown image kind", http.StatusBadRequest)
		return
	}

	if path == "" {
		http.Error(w, "no image available", http.StatusNotFound)
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "could not read the image", http.StatusNotFound)
		return
	}
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(b)
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// Serve runs the HTTP server on the given listener until it fails.
func (s *Server) Serve(ln net.Listener, origin string) {
	srv := &http.Server{
		Handler:           s.Handler(origin),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("local server stopped: %v", err)
	}
}
