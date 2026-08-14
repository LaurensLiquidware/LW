package profileunity

import (
	"bytes"
	"encoding/json"
)

// flexString unmarshals a JSON string, number, boolean, or null into a Go
// string, holding whatever text was there. The §3 API contract documents
// every /licenseinfo field as a string, but cross-version behavior is
// unverified (project brief §4) — a future console sending a real JSON
// number or boolean for a field must not crash decoding. Unmarshaling into
// a plain `string` field would do exactly that.
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
