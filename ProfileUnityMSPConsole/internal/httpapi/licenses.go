package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"profileunity-msp-console/internal/auth"
	"profileunity-msp-console/internal/licensepush"
	"profileunity-msp-console/internal/licenseserver"
	"profileunity-msp-console/internal/tenant"
)

// LicenseDeps bundles what the Licenses screen's handlers need.
type LicenseDeps struct {
	Tenants *tenant.Repo
	Pushes  *licensepush.Repo
	Users   *auth.UserRepo

	// DemoMode is true when running against a demo.db sidecar file --
	// see DisallowInDemoMode, which uses this to block Checkup/Push, the
	// two handlers here that make an outbound call to a real host.
	DemoMode bool
}

type licenseServerDTO struct {
	Hostname      string `json:"hostname"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	HasPassword   bool   `json:"hasPassword"`
	TLSSkipVerify bool   `json:"tlsSkipVerify"`
}

func toLicenseServerDTO(t tenant.Tenant) licenseServerDTO {
	return licenseServerDTO{
		Hostname:      t.LicenseServerHostname,
		Port:          t.LicenseServerPort,
		Username:      t.LicenseServerUsername,
		HasPassword:   t.LicenseServerHasPassword,
		TLSSkipVerify: t.LicenseServerTLSSkipVerify,
	}
}

// GetLicenseServerHandler returns the tenant's current License Server
// connection (never the password).
func GetLicenseServerHandler(deps LicenseDeps) http.HandlerFunc {
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
		writeJSON(w, http.StatusOK, toLicenseServerDTO(t))
	}
}

type licenseServerWriteRequest struct {
	Hostname      string  `json:"hostname"`
	Port          int     `json:"port"`
	Username      string  `json:"username"`
	Password      *string `json:"password"`
	TLSSkipVerify bool    `json:"tlsSkipVerify"`
}

// UpdateLicenseServerHandler saves the tenant's License Server
// connection. Password uses the same three-way pointer semantics as the
// tenant's own credential update (nil keeps it, "" clears it, a
// non-empty string replaces it).
func UpdateLicenseServerHandler(deps LicenseDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req licenseServerWriteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		id := r.PathValue("id")
		err := deps.Tenants.UpdateLicenseServer(r.Context(), id, req.Hostname, req.Port, req.Username, req.Password, req.TLSSkipVerify)
		if err := writeTenantWriteError(w, err); err != nil {
			return
		}
		t, err := deps.Tenants.Get(r.Context(), id)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, toLicenseServerDTO(t))
	}
}

type licenseServerCheckupResponse struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
}

// CheckupLicenseServerHandler confirms reachability of the tenant's
// License Server before a push is attempted. Blocked in demo mode (see
// LicenseDeps.DemoMode / DisallowInDemoMode at the route registration) --
// this makes a real outbound call.
func CheckupLicenseServerHandler(deps LicenseDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client, err := licenseServerClientForTenant(r.Context(), deps, r.PathValue("id"))
		if err != nil {
			writeLicenseServerClientError(w, err)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		ok, message, err := client.Checkup(ctx)
		if err != nil {
			writeJSON(w, http.StatusOK, licenseServerCheckupResponse{Ok: false, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, licenseServerCheckupResponse{Ok: ok, Message: message})
	}
}

type decodedLicenseDTO struct {
	Organization string `json:"organization"`
	ContactName  string `json:"contactName"`
	ContactEmail string `json:"contactEmail"`
	ValidUntil   string `json:"validUntil"`
	LicenseType  string `json:"licenseType"`
	MaxUsers     int    `json:"maxUsers"`
	IsMachine    bool   `json:"isMachine"`
	IsConcurrent bool   `json:"isConcurrent"`
}

func toDecodedLicenseDTO(f licenseserver.Fields) decodedLicenseDTO {
	return decodedLicenseDTO{
		Organization: f.Organization,
		ContactName:  f.ContactName,
		ContactEmail: f.ContactEmail,
		ValidUntil:   f.ValidUntil,
		LicenseType:  f.LicenseType,
		MaxUsers:     f.MaxUsers,
		IsMachine:    f.IsMachine,
		IsConcurrent: f.IsConcurrent,
	}
}

type licensePreviewRequest struct {
	LicenseBase64 string `json:"licenseBase64"`
}

// PreviewLicenseHandler decodes a license code locally, for the
// operator's review before pushing. It never makes a network call and is
// available in demo mode -- nothing here touches a real server.
func PreviewLicenseHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req licensePreviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		fields, err := licenseserver.DecodeLicenseFields(req.LicenseBase64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, toDecodedLicenseDTO(fields))
	}
}

type licensePushRequest struct {
	LicenseBase64 string `json:"licenseBase64"`
	Confirm       bool   `json:"confirm"`
}

type licensePushResponse struct {
	Outcome string            `json:"outcome"`
	Message string            `json:"message"`
	Fields  decodedLicenseDTO `json:"fields"`
}

// PushLicenseHandler installs a license on the tenant's License Server --
// a destructive replace (see internal/licenseserver's package doc).
// Requires an explicit confirm:true, and always records the attempt in
// the push history regardless of outcome. Blocked in demo mode -- this
// makes a real outbound call.
func PushLicenseHandler(deps LicenseDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req licensePushRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !req.Confirm {
			http.Error(w, "confirm must be true: pushing replaces the server's current license and purges its seat assignments", http.StatusBadRequest)
			return
		}

		tenantID := r.PathValue("id")
		client, err := licenseServerClientForTenant(r.Context(), deps, tenantID)
		if err != nil {
			writeLicenseServerClientError(w, err)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		result, err := licensepush.Push(ctx, client, req.LicenseBase64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		operator := operatorUsername(r.Context(), deps.Users)
		var maxUsers *int
		if result.Fields.MaxUsers != 0 {
			v := result.Fields.MaxUsers
			maxUsers = &v
		}
		_, recErr := deps.Pushes.Create(r.Context(), licensepush.Record{
			TenantID:          tenantID,
			OperatorUsername:  operator,
			Outcome:           result.Outcome,
			ErrorMessage:      errorMessageFor(result),
			LicenseCodeBase64: req.LicenseBase64,
			Organization:      result.Fields.Organization,
			ContactName:       result.Fields.ContactName,
			ContactEmail:      result.Fields.ContactEmail,
			ValidUntil:        result.Fields.ValidUntil,
			LicenseType:       result.Fields.LicenseType,
			MaxUsers:          maxUsers,
			IsMachine:         result.Fields.IsMachine,
			IsConcurrent:      result.Fields.IsConcurrent,
		})
		if recErr != nil {
			// The push itself already happened against the real server;
			// failing to write the audit row must not be reported as if
			// the push failed. Surface the push result, not this.
			writeJSON(w, http.StatusOK, licensePushResponse{Outcome: string(result.Outcome), Message: result.Message + " (warning: failed to record this push in history)", Fields: toDecodedLicenseDTO(result.Fields)})
			return
		}
		writeJSON(w, http.StatusOK, licensePushResponse{Outcome: string(result.Outcome), Message: result.Message, Fields: toDecodedLicenseDTO(result.Fields)})
	}
}

func errorMessageFor(result licensepush.Result) string {
	if result.Outcome == licensepush.OutcomeSuccess {
		return ""
	}
	return result.Message
}

type licensePushRecordDTO struct {
	ID                string `json:"id"`
	PushedAtUTC       string `json:"pushedAtUtc"`
	OperatorUsername  string `json:"operatorUsername"`
	Outcome           string `json:"outcome"`
	ErrorMessage      string `json:"errorMessage"`
	LicenseCodeBase64 string `json:"licenseCodeBase64"`
	Organization      string `json:"organization"`
	ContactName       string `json:"contactName"`
	ContactEmail      string `json:"contactEmail"`
	ValidUntil        string `json:"validUntil"`
	LicenseType       string `json:"licenseType"`
	MaxUsers          *int   `json:"maxUsers"`
	IsMachine         bool   `json:"isMachine"`
	IsConcurrent      bool   `json:"isConcurrent"`
}

func toLicensePushRecordDTO(rec licensepush.Record) licensePushRecordDTO {
	return licensePushRecordDTO{
		ID:                rec.ID,
		PushedAtUTC:       rec.PushedAtUTC.Format(time.RFC3339),
		OperatorUsername:  rec.OperatorUsername,
		Outcome:           string(rec.Outcome),
		ErrorMessage:      rec.ErrorMessage,
		LicenseCodeBase64: rec.LicenseCodeBase64,
		Organization:      rec.Organization,
		ContactName:       rec.ContactName,
		ContactEmail:      rec.ContactEmail,
		ValidUntil:        rec.ValidUntil,
		LicenseType:       rec.LicenseType,
		MaxUsers:          rec.MaxUsers,
		IsMachine:         rec.IsMachine,
		IsConcurrent:      rec.IsConcurrent,
	}
}

// LicenseHistoryHandler returns the tenant's push history, newest first.
func LicenseHistoryHandler(deps LicenseDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := deps.Pushes.ListForTenant(r.Context(), r.PathValue("id"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		dtos := make([]licensePushRecordDTO, 0, len(list))
		for _, rec := range list {
			dtos = append(dtos, toLicensePushRecordDTO(rec))
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

// licenseServerClientForTenant resolves tenantID's License Server
// connection and credential and builds a client for it, or a descriptive
// error if nothing is configured yet.
func licenseServerClientForTenant(ctx context.Context, deps LicenseDeps, tenantID string) (*licenseserver.Client, error) {
	t, err := deps.Tenants.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if t.LicenseServerHostname == "" {
		return nil, errLicenseServerNotConfigured
	}
	creds, err := deps.Tenants.GetLicenseServerCredentials(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if creds == nil {
		return nil, errLicenseServerNotConfigured
	}

	baseURL := fmt.Sprintf("https://%s:%d", t.LicenseServerHostname, t.LicenseServerPort)
	var opts []licenseserver.Option
	if t.LicenseServerTLSSkipVerify {
		opts = append(opts, licenseserver.WithInsecureSkipVerify(true))
	}
	return licenseserver.New(baseURL, creds.Username, creds.Password, opts...)
}

var errLicenseServerNotConfigured = errors.New("this tenant has no License Server connection configured yet")

func writeLicenseServerClientError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tenant.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, errLicenseServerNotConfigured):
		http.Error(w, errLicenseServerNotConfigured.Error(), http.StatusBadRequest)
	case errors.Is(err, tenant.ErrEncryptionKeyRequired):
		http.Error(w, "this server has no credential encryption key configured", http.StatusBadRequest)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// operatorUsername resolves the current session's username for the audit
// record. Falls back to "unknown" rather than failing the push outright
// if the lookup itself has a problem -- the push already happened against
// the real server by the time this runs.
func operatorUsername(ctx context.Context, users *auth.UserRepo) string {
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		return "unknown"
	}
	u, err := users.GetByID(ctx, userID)
	if err != nil {
		return "unknown"
	}
	return u.Username
}
