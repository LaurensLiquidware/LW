//go:build !windows

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPickFileHandler_NotImplementedOnNonWindows(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/pick-file", nil)
	rec := httptest.NewRecorder()
	PickFileHandler(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}

func TestPickFolderHandler_NotImplementedOnNonWindows(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/pick-folder", nil)
	rec := httptest.NewRecorder()
	PickFolderHandler(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}
