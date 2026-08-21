package resolve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/inventory"
	"flexapp-vuln-scanner/internal/nvd"
	"flexapp-vuln-scanner/internal/osv"
)

const fixturePath = "../inventory/testdata/sample.inventory.json"

func openSSLMappings() *cpemap.Mappings {
	method := "string-signature"
	product := "OpenSSL"
	return cpemap.New([]cpemap.Entry{
		{Match: cpemap.Match{Method: &method, Product: &product}, CPE: cpemap.CPE{Vendor: "openssl", Product: "openssl"}},
	})
}

// TestResolve_BuildsPurlsAndMatches mirrors
// ../../../flexapp-vuln-scanner/stage2-resolve/tests/test_cli.py's
// test_resolve_vuln_matches_builds_purls_and_matches, against the same
// fixture and mapping, for output parity with the Python implementation.
func TestResolve_BuildsPurlsAndMatches(t *testing.T) {
	inv, err := inventory.Load(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	osvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/querybatch":
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{"vulns": []map[string]any{{"id": "GHSA-aaaa"}}}},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"id": "GHSA-aaaa", "summary": "Something bad", "severity": []any{},
				"database_specific": map[string]any{"severity": "HIGH"},
			})
		}
	}))
	defer osvSrv.Close()

	nvdSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"vulnerabilities": []any{
				map[string]any{"cve": map[string]any{
					"id":           "CVE-2023-0001",
					"descriptions": []any{map[string]any{"lang": "en", "value": "OpenSSL issue"}},
					"metrics": map[string]any{"cvssMetricV31": []any{
						map[string]any{"cvssData": map[string]any{"baseScore": 9.8, "baseSeverity": "CRITICAL"}},
					}},
				}},
			},
		})
	}))
	defer nvdSrv.Close()

	cacheDir := t.TempDir()
	osvClient, err := osv.New(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	osvClient.BaseURL = osvSrv.URL
	nvdClient, err := nvd.New(cacheDir, "")
	if err != nil {
		t.Fatal(err)
	}
	nvdClient.BaseURL = nvdSrv.URL

	result, err := ResolveWithClients(inv, openSSLMappings(), osvClient, nvdClient, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 3 non-excluded files in the fixture (kernel32.dll is excluded).
	if len(result.Components) != 3 {
		t.Fatalf("len(Components) = %d, want 3", len(result.Components))
	}
	byPath := map[string]Component{}
	for _, c := range result.Components {
		byPath[c.RelativePath] = c
	}

	jar := byPath[`Program Files\App\outer-app.jar`]
	if jar.Purl != "pkg:maven/com.acme/outer-app@9.9.9" {
		t.Errorf("jar.Purl = %q", jar.Purl)
	}
	if jar.CPE != "" {
		t.Errorf("jar.CPE = %q, want empty", jar.CPE)
	}
	if jar.Confidence != "exact-purl" {
		t.Errorf("jar.Confidence = %q", jar.Confidence)
	}
	if len(jar.Vulnerabilities) != 1 || jar.Vulnerabilities[0].ID != "GHSA-aaaa" || jar.Vulnerabilities[0].Source != "osv" {
		t.Fatalf("jar.Vulnerabilities = %+v", jar.Vulnerabilities)
	}
	if jar.Vulnerabilities[0].SeverityLevel == nil || *jar.Vulnerabilities[0].SeverityLevel != "HIGH" {
		t.Errorf("jar SeverityLevel = %v", jar.Vulnerabilities[0].SeverityLevel)
	}

	openssl := byPath[`Program Files\App\lib\libcrypto-1_1.dll`]
	if openssl.Purl != "" {
		t.Errorf("openssl.Purl = %q, want empty", openssl.Purl)
	}
	if openssl.CPE != "cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*" {
		t.Errorf("openssl.CPE = %q", openssl.CPE)
	}
	if openssl.Confidence != "mapped-cpe" {
		t.Errorf("openssl.Confidence = %q", openssl.Confidence)
	}
	if len(openssl.Vulnerabilities) != 1 || openssl.Vulnerabilities[0].ID != "CVE-2023-0001" || openssl.Vulnerabilities[0].Source != "nvd" {
		t.Fatalf("openssl.Vulnerabilities = %+v", openssl.Vulnerabilities)
	}

	unresolved := byPath[`Program Files\App\unresolved.bin`]
	if unresolved.Purl != "" || unresolved.CPE != "" || unresolved.Confidence != "" {
		t.Errorf("unresolved = %+v, want all empty", unresolved)
	}
}
