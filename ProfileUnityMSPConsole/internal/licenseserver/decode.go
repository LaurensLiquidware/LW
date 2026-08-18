package licenseserver

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// Fields is what DecodeLicenseFields extracts from a signed license code
// for local display/validation before a push -- never used to
// reconstruct or verify the license itself (only the target server's own
// RSA signature check does that).
type Fields struct {
	Organization string
	ContactName  string
	ContactEmail string
	ValidUntil   string
	LicenseType  string
	MaxUsers     int
	IsMachine    bool
	IsConcurrent bool
}

// DecodeLicenseFields base64-decodes base64License and parses it as the
// <license>...</license> XML document described in
// LICENSE_PUSH_INTEGRATION_SPEC.md §2.2, for local preview only -- it
// makes no network call and cannot confirm the license is genuinely
// signed (only the target License Server's own RSA verification does
// that). Fields are matched case-insensitively since the exact casing has
// varied across the source material this was derived from.
func DecodeLicenseFields(base64License string) (Fields, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(base64License))
	if err != nil {
		return Fields{}, fmt.Errorf("licenseserver: not valid base64: %w", err)
	}

	raw := map[string]string{}
	dec := xml.NewDecoder(bytes.NewReader(decoded))
	var cur string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			cur = strings.ToLower(t.Name.Local)
		case xml.CharData:
			if v := strings.TrimSpace(string(t)); cur != "" && v != "" {
				raw[cur] = v
			}
		case xml.EndElement:
			cur = ""
		}
	}
	if len(raw) == 0 {
		return Fields{}, fmt.Errorf("licenseserver: decoded but does not look like a license XML document")
	}

	f := Fields{
		Organization: raw["organization"],
		ContactName:  raw["contactname"],
		ContactEmail: raw["contactemail"],
		ValidUntil:   raw["validuntil"],
		LicenseType:  raw["licensetype"],
		IsMachine:    strings.EqualFold(raw["licensecomputer"], "yes"),
		IsConcurrent: strings.EqualFold(raw["licenseconcurrent"], "yes"),
	}
	if n, err := strconv.Atoi(raw["maxusers"]); err == nil {
		f.MaxUsers = n
	}
	return f, nil
}
