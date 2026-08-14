// Package profileunity is the ProfileUnity API client: it talks the §3
// wire contract and returns normalized internal types. Nothing outside
// this package should ever see the raw JSON shapes in types_raw.go.
package profileunity

import (
	"strconv"
	"strings"
	"time"
)

// supportEndsLayout is the *only* format SupportEnds is ever parsed with,
// per §3.2: US M/D/YYYY, explicit format, no locale guessing. Go's
// reference time "1/2/2006" is month/day/year with no leading zeros
// required, which matches values like "3/4/2026" as well as "03/04/2026".
const supportEndsLayout = "1/2/2006"

// isoDateLayout is how every stored/machine-read date looks, per §11.2.
const isoDateLayout = "2006-01-02"

// IntField holds a value the API documents as numeric-but-transported-as-
// a-string. Raw is always kept (so a snapshot can be replayed later even
// if parsing rules change); Valid is false when Raw did not parse cleanly,
// which must never be silently treated as zero by a caller.
type IntField struct {
	Raw   string
	Value int
	Valid bool
}

func parseIntField(raw string) IntField {
	raw = strings.TrimSpace(raw)
	n, err := strconv.Atoi(raw)
	if err != nil {
		return IntField{Raw: raw}
	}
	return IntField{Raw: raw, Value: n, Valid: true}
}

// DateField holds a date the API documents as US M/D/YYYY. ISO is the
// §11.2-mandated storage form and is empty when Raw did not parse.
type DateField struct {
	Raw   string
	ISO   string
	Valid bool
}

// parseUSDate parses raw with the explicit US M/D/YYYY format from §3.2 —
// never with a locale- or culture-dependent parser — and normalizes to
// ISO 8601. "03/04/2026" is always March 4th, never 4 March, regardless of
// the host's locale.
func parseUSDate(raw string) DateField {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DateField{}
	}
	t, err := time.Parse(supportEndsLayout, raw)
	if err != nil {
		return DateField{Raw: raw}
	}
	return DateField{Raw: raw, ISO: t.Format(isoDateLayout), Valid: true}
}

// parseYesNo parses the Evaluation field, which the API spells "Yes"/"No"
// — unlike every other boolean-shaped field in §3.2, which is "true"/"false".
func parseYesNo(raw string) bool {
	return strings.EqualFold(strings.TrimSpace(raw), "yes")
}

// parseBoolString parses the "true"/"false" string booleans used
// elsewhere in §3.2 (IsTrialExpired, IsTrial, IsProUOnly, IsFlexOnly, and
// Disabled on §3.4 rows). Anything else, including a missing field,
// parses as false rather than erroring — these are advisory flags, not
// values a missing-field crash should ever be justified by.
func parseBoolString(raw string) bool {
	return strings.EqualFold(strings.TrimSpace(raw), "true")
}

// ConsoleVersion splits the three space-separated parts documented in
// §3.2: product build, internal build number, and build date. Raw is
// always kept; the split parts are advisory only (Valid is false if the
// string does not have exactly three parts).
type ConsoleVersion struct {
	Raw            string
	ProductBuild   string
	InternalNumber string
	BuildDate      string
	Valid          bool
}

func parseConsoleVersion(raw string) ConsoleVersion {
	raw = strings.TrimSpace(raw)
	cv := ConsoleVersion{Raw: raw}
	parts := strings.Fields(raw)
	if len(parts) == 3 {
		cv.ProductBuild = parts[0]
		cv.InternalNumber = parts[1]
		cv.BuildDate = parts[2]
		cv.Valid = true
	}
	return cv
}

// LicenseInfo is the normalized form of /licenseinfo (§3.2).
type LicenseInfo struct {
	RegisteredTo   string
	LicenseMode    string
	LicenseProduct string
	SupportEnds    DateField
	TotalLicenses  IntField

	// UsedLicenses is exactly what the API returned, parsed to an int.
	// Never recompute this value from any other source — per the project
	// brief, if this disagrees with the customer's own console, the
	// console wins and this tool loses credibility.
	UsedLicenses IntField

	Evaluation     bool
	ConsoleVersion ConsoleVersion
	IsTrialExpired bool
	IsTrial        bool
	IsProUOnly     bool
	IsFlexOnly     bool
}

func normalizeLicenseInfo(raw licenseInfoRaw) LicenseInfo {
	return LicenseInfo{
		RegisteredTo:   raw.RegisteredTo.String(),
		LicenseMode:    raw.LicenseMode.String(),
		LicenseProduct: raw.LicenseProduct.String(),
		SupportEnds:    parseUSDate(raw.SupportEnds.String()),
		TotalLicenses:  parseIntField(raw.TotalLicenses.String()),
		UsedLicenses:   parseIntField(raw.UsedLicenses.String()),
		Evaluation:     parseYesNo(raw.Evaluation.String()),
		ConsoleVersion: parseConsoleVersion(raw.ConsoleVersion.String()),
		IsTrialExpired: parseBoolString(raw.IsTrialExpired.String()),
		IsTrial:        parseBoolString(raw.IsTrial.String()),
		IsProUOnly:     parseBoolString(raw.IsProUOnly.String()),
		IsFlexOnly:     parseBoolString(raw.IsFlexOnly.String()),
	}
}

// ServerLicensing is the normalized form of /api/server/licensing (§3.3).
// Organization, ContactName, ContactEmail, and ContactNumber are customer
// PII (project brief §9) — callers must keep them out of logs and error
// payloads.
type ServerLicensing struct {
	MaxUsers      IntField
	UsedLicensed  IntField
	Organization  string
	ContactName   string
	ContactEmail  string
	ContactNumber string
}

func normalizeServerLicensing(raw serverLicensingRaw) ServerLicensing {
	return ServerLicensing{
		MaxUsers:      parseIntField(raw.MaxUsers.Value.String()),
		UsedLicensed:  parseIntField(raw.UsedLicensed.Value.String()),
		Organization:  raw.Organization.String(),
		ContactName:   raw.ContactName.String(),
		ContactEmail:  raw.ContactEmail.String(),
		ContactNumber: raw.ContactNumber.String(),
	}
}

// LicenseServer is the normalized form of one /api/licenseserver row (§3.4).
type LicenseServer struct {
	ServerAddress string
	Port          string

	// LastKnownRunningUTCValid is false when the field was missing or
	// unparseable — a missing heartbeat must never be confused with a
	// heartbeat of the Unix epoch.
	LastKnownRunningUTC      time.Time
	LastKnownRunningUTCValid bool

	// Preferred field is LastKnownRunningUTC; Local's real-conversion
	// semantics are unconfirmed (project brief §4).
	LastKnownRunningLocal      time.Time
	LastKnownRunningLocalValid bool

	MachineGuid string
	ID          string

	// DateCreated is synthesized by the API from the record's ObjectId
	// timestamp and does not exist in the database — advisory only.
	DateCreated      time.Time
	DateCreatedValid bool

	DateLastModified      time.Time
	DateLastModifiedValid bool

	Disabled       bool
	CreatedBy      string
	LastModifiedBy string
}

func normalizeLicenseServer(raw licenseServerRowRaw) LicenseServer {
	return LicenseServer{
		ServerAddress:              raw.ServerAddress.String(),
		Port:                       raw.Port.String(),
		LastKnownRunningUTC:        raw.LastKnownRunningUTC.Time,
		LastKnownRunningUTCValid:   raw.LastKnownRunningUTC.Valid,
		LastKnownRunningLocal:      raw.LastKnownRunningLocal.Time,
		LastKnownRunningLocalValid: raw.LastKnownRunningLocal.Valid,
		MachineGuid:                raw.MachineGuid.String(),
		ID:                         raw.Id.String(),
		DateCreated:                raw.DateCreated.Time,
		DateCreatedValid:           raw.DateCreated.Valid,
		DateLastModified:           raw.DateLastModified.Time,
		DateLastModifiedValid:      raw.DateLastModified.Valid,
		Disabled:                   parseBoolString(raw.Disabled.String()),
		CreatedBy:                  raw.CreatedBy.String(),
		LastModifiedBy:             raw.LastModifiedBy.String(),
	}
}
