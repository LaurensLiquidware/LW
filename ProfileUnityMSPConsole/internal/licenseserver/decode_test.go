package licenseserver

import "testing"

const testLicenseBase64 = "PGxpY2Vuc2U+CiAgPG9yZ2FuaXphdGlvbj5CYWtodWlzIFJldGFpbCBHcm91cDwvb3JnYW5pemF0aW9uPgogIDxjb250YWN0TmFtZT5FcmlrIEJha2h1aXM8L2NvbnRhY3ROYW1lPgogIDxjb250YWN0RW1haWw+ZS5iYWtodWlzQGV4YW1wbGUuY29tPC9jb250YWN0RW1haWw+CiAgPHZhbGlkVW50aWw+MTIvMzEvMjAyNzwvdmFsaWRVbnRpbD4KICA8bGljZW5zZVR5cGU+UGVycGV0dWFsPC9saWNlbnNlVHlwZT4KICA8bWF4VXNlcnM+NDUwPC9tYXhVc2Vycz4KICA8bGljZW5zZUNvbXB1dGVyPm5vPC9saWNlbnNlQ29tcHV0ZXI+CiAgPGxpY2Vuc2VDb25jdXJyZW50PnllczwvbGljZW5zZUNvbmN1cnJlbnQ+CiAgPHNpZ25hdHVyZT5BQkMxMjM9PTwvc2lnbmF0dXJlPgo8L2xpY2Vuc2U+"

func TestDecodeLicenseFields_ValidLicense(t *testing.T) {
	f, err := DecodeLicenseFields(testLicenseBase64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Organization != "Bakhuis Retail Group" {
		t.Errorf("Organization = %q, want %q", f.Organization, "Bakhuis Retail Group")
	}
	if f.ContactEmail != "e.bakhuis@example.com" {
		t.Errorf("ContactEmail = %q", f.ContactEmail)
	}
	if f.ValidUntil != "12/31/2027" {
		t.Errorf("ValidUntil = %q", f.ValidUntil)
	}
	if f.LicenseType != "Perpetual" {
		t.Errorf("LicenseType = %q", f.LicenseType)
	}
	if f.MaxUsers != 450 {
		t.Errorf("MaxUsers = %d, want 450", f.MaxUsers)
	}
	if f.IsMachine {
		t.Error("IsMachine = true, want false")
	}
	if !f.IsConcurrent {
		t.Error("IsConcurrent = false, want true")
	}
}

func TestDecodeLicenseFields_InvalidBase64(t *testing.T) {
	if _, err := DecodeLicenseFields("not-valid-base64!!!"); err == nil {
		t.Fatal("expected an error for invalid base64")
	}
}

func TestDecodeLicenseFields_ValidBase64NotLicenseXML(t *testing.T) {
	// "hello world" base64-encoded -- decodes fine but isn't a license doc.
	if _, err := DecodeLicenseFields("aGVsbG8gd29ybGQ="); err == nil {
		t.Fatal("expected an error for non-license content")
	}
}

func TestDecodeLicenseFields_TrimsWhitespace(t *testing.T) {
	if _, err := DecodeLicenseFields("  " + testLicenseBase64 + "\n"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
