package httpapi

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"time"

	"profileunity-msp-console/internal/auth"
	"profileunity-msp-console/internal/mailer"
	"profileunity-msp-console/internal/reportmail"
	"profileunity-msp-console/internal/reportpdf"
	"profileunity-msp-console/internal/scheduler"
	"profileunity-msp-console/internal/settings"
)

// SettingsDeps bundles what the Settings screen's handlers need.
// Sessions/Scheduler/ReportMail are pushed the new values directly after
// a successful save — that's what makes a change take effect immediately
// rather than only on the process's next restart.
type SettingsDeps struct {
	Store      *settings.Store
	Sessions   *auth.SessionRepo
	Scheduler  *scheduler.Scheduler
	ReportMail *reportmail.Scheduler
	TLSCert    tlscertHolder
}

// tlscertHolder is the minimal interface httpapi needs from
// tlscert.Holder — declared here rather than importing the concrete type
// so tests can supply a fake without touching real certificates.
type tlscertHolder interface {
	Set(certPEM, keyPEM []byte) error
}

type settingsDTO struct {
	SMTPHost        string `json:"smtpHost"`
	SMTPPort        int    `json:"smtpPort"`
	SMTPUsername    string `json:"smtpUsername"`
	SMTPPasswordSet bool   `json:"smtpPasswordSet"`
	SMTPFrom        string `json:"smtpFrom"`
	SMTPSecurity    string `json:"smtpSecurity"`

	ReportRecipients []string `json:"reportRecipients"`
	ReportEmailDay   int      `json:"reportEmailDay"`

	CollectionIntervalSeconds      int    `json:"collectionIntervalSeconds"`
	CollectionTimezone             string `json:"collectionTimezone"`
	CollectionConcurrency          int    `json:"collectionConcurrency"`
	CollectionTenantTimeoutSeconds int    `json:"collectionTenantTimeoutSeconds"`

	SessionIdleTimeoutSeconds     int `json:"sessionIdleTimeoutSeconds"`
	SessionAbsoluteTimeoutSeconds int `json:"sessionAbsoluteTimeoutSeconds"`

	// TLS cert info is read-only here; it's changed only via
	// UploadTLSCertHandler, never via UpdateSettingsHandler.
	TLSCertSubject    string `json:"tlsCertSubject,omitempty"`
	TLSCertExpiresUTC string `json:"tlsCertExpiresUtc,omitempty"`
	TLSCertSelfSigned bool   `json:"tlsCertSelfSigned"`
	TLSCertConfigured bool   `json:"tlsCertConfigured"`

	// CompanyName is saved as part of the main Settings form. The logo
	// image itself is read-only here (see CompanyLogoConfigured) --
	// it's changed only via UploadLogoHandler/ClearLogoHandler, the same
	// split TLS cert already uses.
	CompanyName           string `json:"companyName"`
	CompanyLogoConfigured bool   `json:"companyLogoConfigured"`
}

func toSettingsDTO(s settings.Settings) settingsDTO {
	dto := settingsDTO{
		SMTPHost:                       s.SMTPHost,
		SMTPPort:                       s.SMTPPort,
		SMTPUsername:                   s.SMTPUsername,
		SMTPPasswordSet:                s.SMTPPassword != "",
		SMTPFrom:                       s.SMTPFrom,
		SMTPSecurity:                   s.SMTPSecurity,
		ReportRecipients:               s.ReportRecipients,
		ReportEmailDay:                 s.ReportEmailDay,
		CollectionIntervalSeconds:      int(s.CollectionInterval / time.Second),
		CollectionTimezone:             s.CollectionTimezone,
		CollectionConcurrency:          s.CollectionConcurrency,
		CollectionTenantTimeoutSeconds: int(s.CollectionTenantTimeout / time.Second),
		SessionIdleTimeoutSeconds:      int(s.SessionIdleTimeout / time.Second),
		SessionAbsoluteTimeoutSeconds:  int(s.SessionAbsoluteTimeout / time.Second),
		CompanyName:                    s.CompanyName,
		CompanyLogoConfigured:          len(s.CompanyLogoImage) > 0,
	}
	if dto.ReportRecipients == nil {
		dto.ReportRecipients = []string{}
	}
	if s.TLSCertPEM != "" {
		dto.TLSCertConfigured = true
		if block, _ := pem.Decode([]byte(s.TLSCertPEM)); block != nil {
			if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
				dto.TLSCertSubject = cert.Subject.CommonName
				if dto.TLSCertSubject == "" && len(cert.Subject.Organization) > 0 {
					dto.TLSCertSubject = cert.Subject.Organization[0]
				}
				dto.TLSCertExpiresUTC = cert.NotAfter.UTC().Format(time.RFC3339)
				dto.TLSCertSelfSigned = cert.Issuer.CommonName == cert.Subject.CommonName && string(cert.RawIssuer) == string(cert.RawSubject)
			}
		}
	}
	return dto
}

// settingsWriteRequest is the PUT /api/settings request body.
// SMTPPassword uses the same three-way pointer semantics as a tenant's
// password (see tenants.go): omitted (nil) leaves the stored password
// untouched; present-and-empty clears it; present-and-non-empty sets it.
type settingsWriteRequest struct {
	SMTPHost     string  `json:"smtpHost"`
	SMTPPort     int     `json:"smtpPort"`
	SMTPUsername string  `json:"smtpUsername"`
	SMTPPassword *string `json:"smtpPassword"`
	SMTPFrom     string  `json:"smtpFrom"`
	SMTPSecurity string  `json:"smtpSecurity"`

	ReportRecipients []string `json:"reportRecipients"`
	ReportEmailDay   int      `json:"reportEmailDay"`

	CollectionIntervalSeconds      int    `json:"collectionIntervalSeconds"`
	CollectionTimezone             string `json:"collectionTimezone"`
	CollectionConcurrency          int    `json:"collectionConcurrency"`
	CollectionTenantTimeoutSeconds int    `json:"collectionTenantTimeoutSeconds"`

	SessionIdleTimeoutSeconds     int `json:"sessionIdleTimeoutSeconds"`
	SessionAbsoluteTimeoutSeconds int `json:"sessionAbsoluteTimeoutSeconds"`

	CompanyName string `json:"companyName"`
}

// GetSettingsHandler serves the current runtime settings.
func GetSettingsHandler(deps SettingsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current, ok, err := deps.Store.Load(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "settings not initialized", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, toSettingsDTO(current))
	}
}

// UpdateSettingsHandler validates and persists a new settings payload,
// then pushes the new values into every live component that needs them
// (the collection scheduler, the report-mail scheduler, session
// timeouts) so the change is in effect immediately — no restart.
func UpdateSettingsHandler(deps SettingsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req settingsWriteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		current, ok, err := deps.Store.Load(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "settings not initialized", http.StatusInternalServerError)
			return
		}

		password := current.SMTPPassword
		if req.SMTPPassword != nil {
			password = *req.SMTPPassword
		}

		updated := settings.Settings{
			SMTPHost:                req.SMTPHost,
			SMTPPort:                req.SMTPPort,
			SMTPUsername:            req.SMTPUsername,
			SMTPPassword:            password,
			SMTPFrom:                req.SMTPFrom,
			SMTPSecurity:            req.SMTPSecurity,
			ReportRecipients:        req.ReportRecipients,
			ReportEmailDay:          req.ReportEmailDay,
			CollectionInterval:      time.Duration(req.CollectionIntervalSeconds) * time.Second,
			CollectionTimezone:      req.CollectionTimezone,
			CollectionConcurrency:   req.CollectionConcurrency,
			CollectionTenantTimeout: time.Duration(req.CollectionTenantTimeoutSeconds) * time.Second,
			SessionIdleTimeout:      time.Duration(req.SessionIdleTimeoutSeconds) * time.Second,
			SessionAbsoluteTimeout:  time.Duration(req.SessionAbsoluteTimeoutSeconds) * time.Second,
			TLSCertPEM:              current.TLSCertPEM,
			TLSKeyPEM:               current.TLSKeyPEM,
			CompanyName:             req.CompanyName,
			CompanyLogoImage:        current.CompanyLogoImage,
			CompanyLogoImageType:    current.CompanyLogoImageType,
		}

		if err := updated.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := deps.Store.Update(r.Context(), updated); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		applyLive(deps, updated)
		writeJSON(w, http.StatusOK, toSettingsDTO(updated))
	}
}

// applyLive pushes settings that were just persisted into every running
// component that reads them, so a save takes effect immediately.
func applyLive(deps SettingsDeps, s settings.Settings) {
	loc, err := s.Location()
	if err != nil {
		// Validate already checked this; it can't actually fail here,
		// but a stale/invalid location must never be pushed live.
		return
	}
	deps.Scheduler.SetTunables(s.CollectionInterval, loc, s.CollectionConcurrency, s.CollectionTenantTimeout)
	deps.Sessions.SetTimeouts(s.SessionIdleTimeout, s.SessionAbsoluteTimeout)
	deps.ReportMail.SetConfig(mailer.Config{
		Host:     s.SMTPHost,
		Port:     s.SMTPPort,
		Username: s.SMTPUsername,
		Password: s.SMTPPassword,
		From:     s.SMTPFrom,
		Security: s.SMTPSecurity,
	}, s.ReportRecipients, s.ReportEmailDay, loc, brandingFrom(s))
}

// brandingFrom builds the reportpdf.Branding a Settings value implies --
// shared by applyLive and cmd/server/main.go's boot-time wiring so the two
// never drift.
func brandingFrom(s settings.Settings) reportpdf.Branding {
	return reportpdf.Branding{
		CompanyName:   s.CompanyName,
		LogoImage:     s.CompanyLogoImage,
		LogoImageType: s.CompanyLogoImageType,
	}
}

// logoUploadRequest is the POST /api/settings/logo request body: a
// base64-encoded PNG or JPEG image, read from a local file by the browser
// (FileReader.readAsDataURL, with the "data:image/...;base64," prefix
// stripped) before submitting.
type logoUploadRequest struct {
	ImageBase64 string `json:"imageBase64"`
}

// maxLogoImageBytes caps the stored logo -- enforced before decoding, so
// an oversized upload is rejected cheaply rather than after a wasted
// image.DecodeConfig call.
const maxLogoImageBytes = 2 * 1024 * 1024

// UploadLogoHandler validates a new company logo image and, only if
// it's a well-formed PNG/JPEG under the size cap, stores it so every PDF
// report rendered from now on carries it alongside Liquidware's own
// branding (see reportpdf.Branding).
func UploadLogoHandler(deps SettingsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req logoUploadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.ImageBase64 == "" {
			http.Error(w, "imageBase64 is required", http.StatusBadRequest)
			return
		}

		imageBytes, err := base64.StdEncoding.DecodeString(req.ImageBase64)
		if err != nil {
			http.Error(w, "imageBase64 is not valid base64", http.StatusBadRequest)
			return
		}
		if len(imageBytes) > maxLogoImageBytes {
			http.Error(w, fmt.Sprintf("logo must be %d bytes or smaller", maxLogoImageBytes), http.StatusBadRequest)
			return
		}

		format, err := sniffImageFormat(imageBytes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		current, _, err := deps.Store.Load(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := deps.Store.UpdateBranding(r.Context(), current.CompanyName, imageBytes, format); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		updated, _, err := deps.Store.Load(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		applyLive(deps, updated)
		writeJSON(w, http.StatusOK, toSettingsDTO(updated))
	}
}

// ClearLogoHandler removes a previously uploaded logo, leaving the
// company name and every other setting untouched.
func ClearLogoHandler(deps SettingsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current, _, err := deps.Store.Load(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := deps.Store.UpdateBranding(r.Context(), current.CompanyName, nil, ""); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		updated, _, err := deps.Store.Load(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		applyLive(deps, updated)
		writeJSON(w, http.StatusOK, toSettingsDTO(updated))
	}
}

// GetLogoHandler serves the stored logo's raw image bytes, for the
// Settings screen's <img> preview. 404 if none is stored.
func GetLogoHandler(deps SettingsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current, _, err := deps.Store.Load(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if len(current.CompanyLogoImage) == 0 {
			http.Error(w, "no logo configured", http.StatusNotFound)
			return
		}
		contentType := "image/png"
		if current.CompanyLogoImageType == "jpg" {
			contentType = "image/jpeg"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(current.CompanyLogoImage)
	}
}

// sniffImageFormat decodes just enough of imageBytes to confirm it's a
// PNG or JPEG (the two formats fpdf.ImageOptions.ImageType supports that
// this app accepts for a logo) and returns fpdf's own type string for it.
// Any other format, or malformed image data, is rejected.
func sniffImageFormat(imageBytes []byte) (string, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(imageBytes))
	if err != nil {
		return "", fmt.Errorf("not a readable image: %w", err)
	}
	switch format {
	case "png":
		return "png", nil
	case "jpeg":
		return "jpg", nil
	default:
		return "", fmt.Errorf("logo must be a PNG or JPEG image (got %q)", format)
	}
}

// SendReportNowHandler triggers an immediate send of last calendar
// month's portfolio report email, bypassing the scheduled send day --
// the Settings screen's "Send Now" button. Both of SendNow's failure
// modes (SMTP not configured, no recipients) are configuration problems
// the operator needs to fix on this same screen, so both surface as 400
// with SendNow's own descriptive message.
func SendReportNowHandler(deps SettingsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		year, month, err := deps.ReportMail.SendNow(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"year": year, "month": month})
	}
}

// tlsCertUploadRequest is the POST /api/settings/tls-cert request body:
// a PEM-encoded certificate and matching private key, pasted or read
// from a local file by the browser before submitting.
type tlsCertUploadRequest struct {
	CertPEM string `json:"certPem"`
	KeyPEM  string `json:"keyPem"`
}

// UploadTLSCertHandler validates a new certificate/key pair and, only if
// they match, hot-swaps them into the running HTTPS listener and
// persists them so a restart keeps using the same certificate.
func UploadTLSCertHandler(deps SettingsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tlsCertUploadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.CertPEM == "" || req.KeyPEM == "" {
			http.Error(w, "certPem and keyPem are both required", http.StatusBadRequest)
			return
		}

		if err := deps.TLSCert.Set([]byte(req.CertPEM), []byte(req.KeyPEM)); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := deps.Store.UpdateTLSCert(r.Context(), req.CertPEM, req.KeyPEM); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		current, _, err := deps.Store.Load(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, toSettingsDTO(current))
	}
}

// testEmailRequest is the POST /api/settings/test-email request body —
// deliberately its own SMTP field set (not the saved settings) so an
// operator can test a configuration before saving it, the same way
// tenant Test Connection tests unsaved form values.
type testEmailRequest struct {
	SMTPHost     string  `json:"smtpHost"`
	SMTPPort     int     `json:"smtpPort"`
	SMTPUsername string  `json:"smtpUsername"`
	SMTPPassword *string `json:"smtpPassword"` // nil = use the currently stored password
	SMTPFrom     string  `json:"smtpFrom"`
	SMTPSecurity string  `json:"smtpSecurity"`
	To           string  `json:"to"`
}

// TestEmailHandler sends a one-off test message so an operator can
// confirm SMTP settings work before saving them.
func TestEmailHandler(deps SettingsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req testEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.To == "" {
			http.Error(w, "to is required", http.StatusBadRequest)
			return
		}

		password := ""
		if req.SMTPPassword != nil {
			password = *req.SMTPPassword
		} else if current, ok, err := deps.Store.Load(r.Context()); err == nil && ok {
			password = current.SMTPPassword
		}

		cfg := mailer.Config{
			Host:     req.SMTPHost,
			Port:     req.SMTPPort,
			Username: req.SMTPUsername,
			Password: password,
			From:     req.SMTPFrom,
			Security: req.SMTPSecurity,
		}
		err := mailer.Send(cfg, []string{req.To},
			"ProfileUnity MSP Licensing Console — test email",
			"This is a test message from the ProfileUnity MSP Licensing Console Settings screen. If you received this, the SMTP settings you're testing work.\n",
			nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
