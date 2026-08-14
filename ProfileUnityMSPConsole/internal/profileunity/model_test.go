package profileunity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadEnvelopeTag(t *testing.T, filename string) json.RawMessage {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("read fixture %s: %v", filename, err)
	}
	tag, err := decodeEnvelope(body)
	if err != nil {
		t.Fatalf("decode envelope %s: %v", filename, err)
	}
	return tag
}

func decodeLicenseInfoFixture(t *testing.T, filename string) LicenseInfo {
	t.Helper()
	tag := loadEnvelopeTag(t, filename)
	var rows []licenseInfoRaw
	if err := json.Unmarshal(tag, &rows); err != nil {
		t.Fatalf("unmarshal licenseinfo rows from %s: %v", filename, err)
	}
	if len(rows) != 1 {
		t.Fatalf("%s: got %d rows, want 1", filename, len(rows))
	}
	return normalizeLicenseInfo(rows[0])
}

// TestLicenseInfo_ReferenceFixture cross-checks the §3.2 payload verbatim
// against the independently-confirmed database values: TotalLicenses=5,
// UsedLicenses=1. Any parsing path that does not yield 1 of 5 here is wrong.
func TestLicenseInfo_ReferenceFixture(t *testing.T) {
	info := decodeLicenseInfoFixture(t, "licenseinfo_success.json")

	if !info.TotalLicenses.Valid || info.TotalLicenses.Value != 5 {
		t.Errorf("TotalLicenses = %+v, want Valid=true Value=5", info.TotalLicenses)
	}
	if !info.UsedLicenses.Valid || info.UsedLicenses.Value != 1 {
		t.Errorf("UsedLicenses = %+v, want Valid=true Value=1", info.UsedLicenses)
	}
	if info.RegisteredTo != "Liquidware Training EU" {
		t.Errorf("RegisteredTo = %q", info.RegisteredTo)
	}
	if info.LicenseMode != "NamedUser" {
		t.Errorf("LicenseMode = %q", info.LicenseMode)
	}
	if !info.Evaluation {
		t.Error("Evaluation should be true for \"Yes\"")
	}
	if info.IsTrialExpired || info.IsTrial || info.IsProUOnly || info.IsFlexOnly {
		t.Error("all Is* flags should be false for \"false\"")
	}
	if !info.SupportEnds.Valid || info.SupportEnds.ISO != "2026-12-31" {
		t.Errorf("SupportEnds = %+v, want ISO=2026-12-31", info.SupportEnds)
	}
	if !info.ConsoleVersion.Valid ||
		info.ConsoleVersion.ProductBuild != "6.9.5.9678" ||
		info.ConsoleVersion.InternalNumber != "3038806" ||
		info.ConsoleVersion.BuildDate != "2026-07-01" {
		t.Errorf("ConsoleVersion = %+v", info.ConsoleVersion)
	}
}

func TestLicenseInfo_ErrorEnvelopeHTTP200(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "licenseinfo_error_http200.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = decodeEnvelope(body)
	if err == nil {
		t.Fatal("expected an error for Type=error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Message != "Invalid username or password" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestLicenseInfo_MissingFieldsDoNotCrash(t *testing.T) {
	info := decodeLicenseInfoFixture(t, "licenseinfo_missing_fields.json")
	if info.RegisteredTo != "" {
		t.Errorf("RegisteredTo = %q, want empty", info.RegisteredTo)
	}
	if info.SupportEnds.Valid {
		t.Error("SupportEnds should be invalid when the field is missing")
	}
	if !info.TotalLicenses.Valid || info.TotalLicenses.Value != 5 {
		t.Errorf("TotalLicenses = %+v", info.TotalLicenses)
	}
}

func TestLicenseInfo_UnexpectedFieldTypesDoNotCrash(t *testing.T) {
	info := decodeLicenseInfoFixture(t, "licenseinfo_unexpected_types.json")
	if !info.TotalLicenses.Valid || info.TotalLicenses.Value != 5 {
		t.Errorf("TotalLicenses = %+v, want Valid=true Value=5 (from a JSON number)", info.TotalLicenses)
	}
	if !info.UsedLicenses.Valid || info.UsedLicenses.Value != 1 {
		t.Errorf("UsedLicenses = %+v, want Valid=true Value=1 (from a JSON number)", info.UsedLicenses)
	}
	if info.IsTrialExpired || info.IsTrial || info.IsProUOnly || info.IsFlexOnly {
		t.Error("Is* flags decoded from JSON booleans should still read as false")
	}
}

func TestLicenseInfo_UnknownFieldAndDifferingConsoleVersion(t *testing.T) {
	info := decodeLicenseInfoFixture(t, "licenseinfo_unknown_field_diff_version.json")
	if info.ConsoleVersion.Raw != "7.0.0.1234 4000000 2027-01-15" {
		t.Errorf("ConsoleVersion.Raw = %q", info.ConsoleVersion.Raw)
	}
	if info.RegisteredTo != "Future Console Customer" {
		t.Errorf("RegisteredTo = %q", info.RegisteredTo)
	}
}

func TestLicenseInfo_ConcurrentMode(t *testing.T) {
	info := decodeLicenseInfoFixture(t, "licenseinfo_concurrent.json")
	if info.LicenseMode != "Concurrent" {
		t.Errorf("LicenseMode = %q, want Concurrent", info.LicenseMode)
	}
}

func TestLicenseInfo_UsedExceedsTotal(t *testing.T) {
	info := decodeLicenseInfoFixture(t, "licenseinfo_used_exceeds_total.json")
	if info.UsedLicenses.Value <= info.TotalLicenses.Value {
		t.Errorf("expected UsedLicenses (%d) > TotalLicenses (%d)", info.UsedLicenses.Value, info.TotalLicenses.Value)
	}
}

func TestLicenseInfo_SupportEndsAmbiguousDateIsUSInterpretation(t *testing.T) {
	info := decodeLicenseInfoFixture(t, "licenseinfo_ambiguous_date.json")
	// "03/04/2026" must be March 4th (US), never 4 March (EU).
	if info.SupportEnds.ISO != "2026-03-04" {
		t.Errorf("SupportEnds.ISO = %q, want 2026-03-04 (US M/D/YYYY interpretation)", info.SupportEnds.ISO)
	}
}

func TestLicenseInfo_NonLatinRegisteredToSurvivesRoundTrip(t *testing.T) {
	info := decodeLicenseInfoFixture(t, "licenseinfo_nonlatin_registeredto.json")
	want := "日本語データ 简体中文 한국어 Данные Ångström café naïve"
	if info.RegisteredTo != want {
		t.Errorf("RegisteredTo = %q, want %q", info.RegisteredTo, want)
	}
}

func TestParseUSDate(t *testing.T) {
	cases := []struct {
		raw     string
		wantISO string
		valid   bool
	}{
		{"12/31/2026", "2026-12-31", true},
		{"03/04/2026", "2026-03-04", true}, // ambiguous vs. EU reading; US wins
		{"1/15/2020", "2020-01-15", true},  // already-past date
		{"", "", false},
		{"not-a-date", "", false},
		{"31/12/2026", "", false}, // an EU-only date must not silently parse
	}
	for _, c := range cases {
		got := parseUSDate(c.raw)
		if got.Valid != c.valid || got.ISO != c.wantISO {
			t.Errorf("parseUSDate(%q) = %+v, want ISO=%q valid=%v", c.raw, got, c.wantISO, c.valid)
		}
	}
}

func TestParseYesNo(t *testing.T) {
	if !parseYesNo("Yes") || !parseYesNo("yes") {
		t.Error("Yes should parse true")
	}
	if parseYesNo("No") || parseYesNo("") || parseYesNo("true") {
		t.Error("No/empty/true should parse false for the Yes/No field")
	}
}

func TestParseBoolString(t *testing.T) {
	if !parseBoolString("true") || !parseBoolString("True") {
		t.Error("true should parse true")
	}
	if parseBoolString("false") || parseBoolString("") || parseBoolString("Yes") {
		t.Error("false/empty/Yes should parse false for the true/false field")
	}
}

func TestParseConsoleVersion(t *testing.T) {
	cv := parseConsoleVersion("6.9.5.9678 3038806 2026-07-01")
	if !cv.Valid || cv.ProductBuild != "6.9.5.9678" || cv.InternalNumber != "3038806" || cv.BuildDate != "2026-07-01" {
		t.Errorf("parseConsoleVersion = %+v", cv)
	}
	cv2 := parseConsoleVersion("garbage")
	if cv2.Valid {
		t.Errorf("expected invalid ConsoleVersion for a single-token string, got %+v", cv2)
	}
	if cv2.Raw != "garbage" {
		t.Errorf("Raw should always be kept: got %q", cv2.Raw)
	}
}

func TestServerLicensing_Fixture(t *testing.T) {
	tag := loadEnvelopeTag(t, "server_licensing_success.json")
	var raw serverLicensingRaw
	if err := json.Unmarshal(tag, &raw); err != nil {
		t.Fatal(err)
	}
	sl := normalizeServerLicensing(raw)
	if !sl.MaxUsers.Valid || sl.MaxUsers.Value != 5 {
		t.Errorf("MaxUsers = %+v", sl.MaxUsers)
	}
	if !sl.UsedLicensed.Valid || sl.UsedLicensed.Value != 1 {
		t.Errorf("UsedLicensed = %+v", sl.UsedLicensed)
	}
	if sl.Organization != "Liquidware Training EU" {
		t.Errorf("Organization = %q", sl.Organization)
	}
}

func TestLicenseServer_Fixture(t *testing.T) {
	tag := loadEnvelopeTag(t, "licenseserver_rows.json")
	var rows rowsTag[licenseServerRowRaw]
	if err := json.Unmarshal(tag, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows.Rows))
	}
	ls := normalizeLicenseServer(rows.Rows[0])
	if ls.ServerAddress != "10.0.0.5" {
		t.Errorf("ServerAddress = %q", ls.ServerAddress)
	}
	if !ls.LastKnownRunningUTCValid {
		t.Error("LastKnownRunningUTCValid should be true")
	}
	wantMillis := int64(1786690772847)
	if ls.LastKnownRunningUTC.UnixMilli() != wantMillis {
		t.Errorf("LastKnownRunningUTC millis = %d, want %d", ls.LastKnownRunningUTC.UnixMilli(), wantMillis)
	}
	if ls.Disabled {
		t.Error("Disabled should be false")
	}
}

func TestAspNetDate(t *testing.T) {
	var d aspNetDate
	if err := json.Unmarshal([]byte(`"/Date(1786690772847)/"`), &d); err != nil {
		t.Fatal(err)
	}
	if !d.Valid || d.Time.UnixMilli() != 1786690772847 {
		t.Errorf("got %+v", d)
	}

	var empty aspNetDate
	if err := json.Unmarshal([]byte(`""`), &empty); err != nil {
		t.Fatal(err)
	}
	if empty.Valid {
		t.Error("empty string should decode to Valid=false")
	}

	var bad aspNetDate
	if err := json.Unmarshal([]byte(`"not-a-date"`), &bad); err == nil {
		t.Error("expected an error for an unrecognized date format")
	}

	var withOffset aspNetDate
	if err := json.Unmarshal([]byte(`"/Date(1786690772847+0200)/"`), &withOffset); err != nil {
		t.Fatal(err)
	}
	if !withOffset.Valid || withOffset.Time.UnixMilli() != 1786690772847 {
		t.Errorf("got %+v", withOffset)
	}
}
