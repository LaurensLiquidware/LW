package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigHandler_ReportsPickerAddr(t *testing.T) {
	handler := ConfigHandler("127.0.0.1:8745")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		PickerAddr string `json:"pickerAddr"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.PickerAddr != "127.0.0.1:8745" {
		t.Errorf("pickerAddr = %q, want 127.0.0.1:8745", body.PickerAddr)
	}
}

func TestConfigHandler_EmptyPickerAddrReportedAsEmpty(t *testing.T) {
	handler := ConfigHandler("")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var body struct {
		PickerAddr string `json:"pickerAddr"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.PickerAddr != "" {
		t.Errorf("pickerAddr = %q, want empty", body.PickerAddr)
	}
}
