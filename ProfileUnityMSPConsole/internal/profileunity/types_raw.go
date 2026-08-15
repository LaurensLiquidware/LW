package profileunity

// Raw transport types. These mirror the §3 wire format exactly, field
// spelling included — nothing past this file should ever see these types
// or their JSON tags. Every value that the API documents as a string
// (which per §3.2 is every field on licenseInfoRaw) uses flexString so an
// unexpected number/boolean on a future console version doesn't crash
// decoding.

// licenseInfoRaw is the single element of the /licenseinfo Tag array (§3.2).
type licenseInfoRaw struct {
	RegisteredTo   flexString `json:"RegisteredTo"`
	LicenseMode    flexString `json:"LicenseMode"`
	LicenseProduct flexString `json:"LicenseProduct"`
	SupportEnds    flexString `json:"SupportEnds"`
	TotalLicenses  flexString `json:"TotalLicenses"`
	UsedLicenses   flexString `json:"UsedLicenses"`
	Evaluation     flexString `json:"Evaluation"`
	ConsoleVersion flexString `json:"ConsoleVersion"`
	IsTrialExpired flexString `json:"IsTrialExpired"`
	IsTrial        flexString `json:"IsTrial"`
	IsProUOnly     flexString `json:"IsProUOnly"`
	IsFlexOnly     flexString `json:"IsFlexOnly"`
}

// serverLicensingField is the {Name, Value} shape used throughout
// /api/server/licensing (§3.3). Name is a display label and may localize
// — callers must never key off it, only off the enclosing JSON field name.
type serverLicensingField struct {
	Name  flexString `json:"Name"`
	Value flexString `json:"Value"`
}

// serverLicensingRaw is the /api/server/licensing Tag object (§3.3). Note
// the field is spelled "UsedLicensed" here but "UsedLicenses" on
// licenseInfoRaw — the brief calls this out explicitly; each transport
// type keeps its endpoint's own spelling, normalized only in model.go.
type serverLicensingRaw struct {
	MaxUsers      serverLicensingField `json:"MaxUsers"`
	UsedLicensed  serverLicensingField `json:"UsedLicensed"`
	Organization  flexString           `json:"Organization"`
	ContactName   flexString           `json:"ContactName"`
	ContactEmail  flexString           `json:"ContactEmail"`
	ContactNumber flexString           `json:"ContactNumber"`
}

// rowsTag is the generic paged-list Tag shape from §3.1:
// { "Rows": [...], "TotalPages": n, "CurrentPage": n, "TotalRecords": n }.
type rowsTag[T any] struct {
	Rows         []T        `json:"Rows"`
	TotalPages   flexString `json:"TotalPages"`
	CurrentPage  flexString `json:"CurrentPage"`
	TotalRecords flexString `json:"TotalRecords"`
}

// licenseServerRowRaw is one row of /api/licenseserver's Tag.Rows (§3.4).
type licenseServerRowRaw struct {
	ServerAddress         flexString `json:"ServerAddress"`
	Port                  flexString `json:"Port"`
	LastKnownRunningUTC   aspNetDate `json:"LastKnownRunningUTC"`
	LastKnownRunningLocal aspNetDate `json:"LastKnownRunningLocal"`
	MachineGuid           flexString `json:"MachineGuid"`
	Id                    flexString `json:"Id"`
	DateCreated           aspNetDate `json:"DateCreated"`
	DateLastModified      aspNetDate `json:"DateLastModified"`
	Disabled              flexString `json:"Disabled"`
	CreatedBy             flexString `json:"CreatedBy"`
	LastModifiedBy        flexString `json:"LastModifiedBy"`
}
