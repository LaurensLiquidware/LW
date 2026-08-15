package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"profileunity-msp-console/internal/collector"
	"profileunity-msp-console/internal/tenant"
)

// TenantDeps bundles what the tenant handlers need.
type TenantDeps struct {
	Tenants *tenant.Repo
}

// tenantDTO is the tenant shape the frontend sees. It never carries a
// password — HasPassword says whether one is configured, per project
// brief §9 ("never return them through the API once saved, even masked").
type tenantDTO struct {
	ID            string   `json:"id"`
	DisplayName   string   `json:"displayName"`
	Hostname      string   `json:"hostname"`
	Port          int      `json:"port"`
	Username      string   `json:"username"`
	HasPassword   bool     `json:"hasPassword"`
	TLSSkipVerify bool     `json:"tlsSkipVerify"`
	Enabled       bool     `json:"enabled"`
	Tags          []string `json:"tags"`
	Notes         string   `json:"notes"`
	CreatedAtUTC  string   `json:"createdAtUtc"`
	UpdatedAtUTC  string   `json:"updatedAtUtc"`
}

func toDTO(t tenant.Tenant) tenantDTO {
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}
	return tenantDTO{
		ID:            t.ID,
		DisplayName:   t.DisplayName,
		Hostname:      t.Hostname,
		Port:          t.Port,
		Username:      t.Username,
		HasPassword:   t.HasPassword,
		TLSSkipVerify: t.TLSSkipVerify,
		Enabled:       t.Enabled,
		Tags:          tags,
		Notes:         t.Notes,
		CreatedAtUTC:  t.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAtUTC:  t.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// tenantWriteRequest is the create/update request body. Password uses
// the same three-way pointer semantics as tenant.UpdateInput: omitted
// (nil) leaves a stored password untouched on update, or means "no
// password" on create; present-and-empty clears it; present-and-non-empty
// sets it.
type tenantWriteRequest struct {
	DisplayName   string   `json:"displayName"`
	Hostname      string   `json:"hostname"`
	Port          int      `json:"port"`
	Username      string   `json:"username"`
	Password      *string  `json:"password"`
	TLSSkipVerify bool     `json:"tlsSkipVerify"`
	Enabled       bool     `json:"enabled"`
	Tags          []string `json:"tags"`
	Notes         string   `json:"notes"`
}

func ListTenantsHandler(deps TenantDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := deps.Tenants.List(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		dtos := make([]tenantDTO, 0, len(tenants))
		for _, t := range tenants {
			dtos = append(dtos, toDTO(t))
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

func GetTenantHandler(deps TenantDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := deps.Tenants.Get(r.Context(), r.PathValue("id"))
		if errors.Is(err, tenant.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, toDTO(t))
	}
}

func CreateTenantHandler(deps TenantDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tenantWriteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		password := ""
		if req.Password != nil {
			password = *req.Password
		}
		t, err := deps.Tenants.Create(r.Context(), tenant.CreateInput{
			DisplayName: req.DisplayName, Hostname: req.Hostname, Port: req.Port,
			Username: req.Username, Password: password, TLSSkipVerify: req.TLSSkipVerify,
			Enabled: req.Enabled, Tags: req.Tags, Notes: req.Notes,
		})
		if err := writeTenantWriteError(w, err); err != nil {
			return
		}
		writeJSON(w, http.StatusCreated, toDTO(t))
	}
}

func UpdateTenantHandler(deps TenantDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tenantWriteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		t, err := deps.Tenants.Update(r.Context(), r.PathValue("id"), tenant.UpdateInput{
			DisplayName: req.DisplayName, Hostname: req.Hostname, Port: req.Port,
			Username: req.Username, Password: req.Password, TLSSkipVerify: req.TLSSkipVerify,
			Enabled: req.Enabled, Tags: req.Tags, Notes: req.Notes,
		})
		if err := writeTenantWriteError(w, err); err != nil {
			return
		}
		writeJSON(w, http.StatusOK, toDTO(t))
	}
}

func DeleteTenantHandler(deps TenantDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := deps.Tenants.Delete(r.Context(), r.PathValue("id")); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// writeTenantWriteError maps a tenant repo error from Create/Update to an
// HTTP response and reports whether it wrote one (true = caller should
// stop; false = err was nil, proceed).
func writeTenantWriteError(w http.ResponseWriter, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, tenant.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, tenant.ErrCredentialsMismatch):
		http.Error(w, "username and password must be both set or both empty", http.StatusBadRequest)
	case errors.Is(err, tenant.ErrEncryptionKeyRequired):
		http.Error(w, "this server has no credential encryption key configured; a tenant password cannot be stored", http.StatusBadRequest)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
	return err
}

type testConnectionRequest struct {
	Hostname      string  `json:"hostname"`
	Port          int     `json:"port"`
	TLSSkipVerify bool    `json:"tlsSkipVerify"`
	Username      string  `json:"username"`
	Password      *string `json:"password"`
}

type testConnectionResponse struct {
	Outcome string `json:"outcome"`
	Message string `json:"message"`
}

// TestConnectionHandler backs the "Test Connection" button on the tenant
// form — project brief §7.1: reports precisely what happened, not a
// boolean. It never persists anything.
func TestConnectionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req testConnectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		password := ""
		if req.Password != nil {
			password = *req.Password
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		outcome, message := collector.TestConnection(ctx, collector.TestConnectionParams{
			Hostname: req.Hostname, Port: req.Port, TLSSkipVerify: req.TLSSkipVerify,
			Username: req.Username, Password: password,
		})
		writeJSON(w, http.StatusOK, testConnectionResponse{Outcome: string(outcome), Message: message})
	}
}
