//go:build windows

package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/lxn/walk"
)

// startPickerServer hosts the native file/folder picker the New Scan
// screen's Browse buttons call: GET /pick-file and GET /pick-folder each
// show a real Win32 dialog (via lxn/walk, this app's existing GUI
// dependency) and return the chosen path as JSON. This has to live here,
// not in cmd/server, because showing a Win32 dialog requires the same
// GUI thread that owns a.mw -- the headless server has no such thread.
// Listening on loopback only (config.DefaultPickerAddr / FVS_PICKER_ADDR)
// and CORS-allowing any origin is safe: this only ever answers a picked
// local file path back to whatever is running on the same machine, never
// file contents.
//
// A failure to bind (most likely: another instance of this launcher
// already running) is reported once via a warning dialog and otherwise
// ignored -- the picker is a convenience feature, not something worth
// blocking startup over. The frontend degrades gracefully (hides its
// Browse buttons) whenever this endpoint isn't reachable.
func (a *app) startPickerServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/pick-file", a.handlePickFile)
	mux.HandleFunc("/pick-folder", a.handlePickFolder)

	srv := &http.Server{Addr: addr, Handler: withPickerCORS(mux)}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Warn("picker server failed to start", "addr", addr, "error", err)
		}
	}()
	return srv
}

// withPickerCORS lets the Angular app (served from the main server's own
// origin, a different port than the picker) call these endpoints via
// fetch/XHR from the browser -- without this, the browser's
// same-origin policy blocks the response before JS ever sees it.
func withPickerCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type pickResponse struct {
	Path string `json:"path"`
}

// runOnUIThread schedules fn on the GUI thread via walk's Synchronize
// (required for any Win32 dialog call) and blocks until it completes, so
// the calling HTTP handler goroutine can read back whatever fn set.
func (a *app) runOnUIThread(fn func()) {
	done := make(chan struct{})
	a.mw.Synchronize(func() {
		fn()
		close(done)
	})
	<-done
}

func (a *app) handlePickFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	title := r.URL.Query().Get("title")
	if title == "" {
		title = "Select a File"
	}
	filter := r.URL.Query().Get("filter")

	var (
		accepted bool
		path     string
		dlgErr   error
	)
	a.runOnUIThread(func() {
		dlg := &walk.FileDialog{Title: title, Filter: filter}
		accepted, dlgErr = dlg.ShowOpen(a.mw)
		path = dlg.FilePath
	})

	if dlgErr != nil {
		http.Error(w, dlgErr.Error(), http.StatusInternalServerError)
		return
	}
	if !accepted {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writePickerJSON(w, pickResponse{Path: path})
}

func (a *app) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	title := r.URL.Query().Get("title")
	if title == "" {
		title = "Select a Folder"
	}

	var (
		accepted bool
		path     string
		dlgErr   error
	)
	a.runOnUIThread(func() {
		dlg := &walk.FileDialog{Title: title}
		accepted, dlgErr = dlg.ShowBrowseFolder(a.mw)
		path = dlg.FilePath
	})

	if dlgErr != nil {
		http.Error(w, dlgErr.Error(), http.StatusInternalServerError)
		return
	}
	if !accepted {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writePickerJSON(w, pickResponse{Path: path})
}

func writePickerJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(v)
}
