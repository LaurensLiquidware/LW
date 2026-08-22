package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigHandler_ReportsPickerAvailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	ConfigHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		PickerAvailable bool `json:"pickerAvailable"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// pickerAvailable is a build-time constant (true only in the Windows
	// build, see picker_windows.go/picker_other.go) -- this just confirms
	// the handler reports whatever this build was compiled with, not a
	// hardcoded true/false.
	if body.PickerAvailable != pickerAvailable {
		t.Errorf("pickerAvailable = %v, want %v", body.PickerAvailable, pickerAvailable)
	}
}
