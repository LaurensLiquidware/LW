package licenseserver

import (
	"bytes"
	"encoding/json"
)

// flexString unmarshals a JSON string, number, boolean, or null into a Go
// string, holding whatever text was there. LICENSE_PUSH_INTEGRATION_SPEC.md
// documents every text field on LicenseInfo/LicenseInfoItem as a string,
// but that spec is explicitly derived from decompilation and flagged as
// needing per-build verification -- a real server has been observed
// sending a JSON number for a field the spec called a string (e.g.
// ProductType), which unmarshaling into a plain `string` field would
// crash on. Mirrors internal/profileunity's flexString (duplicated
// rather than exported across packages for one consumer -- the type is
// unexported there too).
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*f = ""
		return nil
	}
	if len(data) >= 2 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	// Number, boolean, or anything else JSON-scalar: keep its literal text.
	*f = flexString(data)
	return nil
}

func (f flexString) String() string {
	return string(f)
}
