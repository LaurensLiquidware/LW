package cpemap

import (
	"testing"

	"flexapp-vuln-scanner/internal/inventory"
)

// The test cases here mirror
// ../../../flexapp-vuln-scanner/stage2-resolve/tests/test_cpe_mappings.py
// exactly, for output parity with the Python implementation.

func s(v string) *string { return &v }

func ident(method string, vendor, product, version *string) *inventory.Identity {
	return &inventory.Identity{Method: method, Vendor: vendor, Product: product, Version: version}
}

func TestFind_ExactMatch(t *testing.T) {
	method := "string-signature"
	product := "OpenSSL"
	m := New([]Entry{{Match: Match{Method: &method, Product: &product}, CPE: CPE{Vendor: "openssl", Product: "openssl"}}})
	id := ident("string-signature", nil, s("OpenSSL"), s("1.1.1w"))
	vendor, prod, ok := m.Find(id)
	if !ok || vendor != "openssl" || prod != "openssl" {
		t.Errorf("got (%q, %q, %v)", vendor, prod, ok)
	}
}

func TestFind_CaseInsensitive(t *testing.T) {
	method := "string-signature"
	product := "openssl"
	m := New([]Entry{{Match: Match{Method: &method, Product: &product}, CPE: CPE{Vendor: "openssl", Product: "openssl"}}})
	id := ident("string-signature", nil, s("OpenSSL"), s("1.1.1w"))
	vendor, prod, ok := m.Find(id)
	if !ok || vendor != "openssl" || prod != "openssl" {
		t.Errorf("got (%q, %q, %v)", vendor, prod, ok)
	}
}

func TestFind_NoMatchReturnsFalse(t *testing.T) {
	method := "string-signature"
	product := "OpenSSL"
	m := New([]Entry{{Match: Match{Method: &method, Product: &product}, CPE: CPE{Vendor: "openssl", Product: "openssl"}}})
	id := ident("string-signature", nil, s("zlib"), s("1.3"))
	if _, _, ok := m.Find(id); ok {
		t.Error("expected no match")
	}
}

func TestFind_MethodMismatch(t *testing.T) {
	method := "electron-embedded"
	product := "OpenSSL"
	m := New([]Entry{{Match: Match{Method: &method, Product: &product}, CPE: CPE{Vendor: "openssl", Product: "openssl"}}})
	id := ident("string-signature", nil, s("OpenSSL"), s("1.1.1w"))
	if _, _, ok := m.Find(id); ok {
		t.Error("expected no match due to method mismatch")
	}
}

func TestFind_MatchesRegardlessOfMethodWhenUnscoped(t *testing.T) {
	product := "zlib"
	m := New([]Entry{{Match: Match{Product: &product}, CPE: CPE{Vendor: "zlib", Product: "zlib"}}})
	for _, method := range []string{"string-signature", "pe-version-resource", "dotnet-manifest"} {
		id := ident(method, nil, s("zlib"), s("1.3.1"))
		vendor, prod, ok := m.Find(id)
		if !ok || vendor != "zlib" || prod != "zlib" {
			t.Errorf("method %s: got (%q, %q, %v)", method, vendor, prod, ok)
		}
	}
}

func TestFind_VendorOnlyMatch(t *testing.T) {
	method := "pe-version-resource"
	vendor := "Google Inc."
	m := New([]Entry{{Match: Match{Method: &method, Vendor: &vendor}, CPE: CPE{Vendor: "google", Product: "chrome"}}})
	id := ident("pe-version-resource", s("Google Inc."), s("Google Chrome"), s("120.0"))
	gotVendor, gotProduct, ok := m.Find(id)
	if !ok || gotVendor != "google" || gotProduct != "chrome" {
		t.Errorf("got (%q, %q, %v)", gotVendor, gotProduct, ok)
	}
}

func TestFind_NoneIdentity(t *testing.T) {
	if _, _, ok := New(nil).Find(nil); ok {
		t.Error("expected no match for nil identity")
	}
}

func TestLoad_RealConfigFileParses(t *testing.T) {
	m, err := Load("../../config/cpe-mappings.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.entries) == 0 {
		t.Fatal("expected at least one mapping entry")
	}
	id := ident("string-signature", nil, s("OpenSSL"), s("1.1.1w"))
	vendor, product, ok := m.Find(id)
	if !ok || vendor != "openssl" || product != "openssl" {
		t.Errorf("got (%q, %q, %v)", vendor, product, ok)
	}
}

func TestFindVersionTransform_ReturnsPatternAndGroup(t *testing.T) {
	product := "FFmpeg"
	m := New([]Entry{{
		Match: Match{Product: &product},
		CPE:   CPE{Vendor: "ffmpeg", Product: "ffmpeg", VersionPattern: `^n?(\d+\.\d+\.\d+)`, VersionGroup: 1},
	}})
	id := ident("pe-version-resource", nil, s("FFmpeg"), s("n7.1.1"))
	pattern, group, ok := m.VersionTransform(id)
	if !ok || pattern != `^n?(\d+\.\d+\.\d+)` || group != 1 {
		t.Errorf("got (%q, %d, %v)", pattern, group, ok)
	}
}

func TestFindVersionTransform_NoneWhenEntryHasNoPattern(t *testing.T) {
	product := "OpenSSL"
	m := New([]Entry{{Match: Match{Product: &product}, CPE: CPE{Vendor: "openssl", Product: "openssl"}}})
	id := ident("string-signature", nil, s("OpenSSL"), s("1.1.1w"))
	if _, _, ok := m.VersionTransform(id); ok {
		t.Error("expected no version transform")
	}
}

func TestFindVersionTransform_NoneWhenNothingMatches(t *testing.T) {
	product := "OpenSSL"
	m := New([]Entry{{Match: Match{Product: &product}, CPE: CPE{Vendor: "openssl", Product: "openssl", VersionPattern: `(\d+)`}}})
	id := ident("string-signature", nil, s("zlib"), s("1.3"))
	if _, _, ok := m.VersionTransform(id); ok {
		t.Error("expected no version transform")
	}
}

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	m, err := Load("/nonexistent/path.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	id := ident("string-signature", nil, s("OpenSSL"), nil)
	if _, _, ok := m.Find(id); ok {
		t.Error("expected no match from empty mappings")
	}
}
