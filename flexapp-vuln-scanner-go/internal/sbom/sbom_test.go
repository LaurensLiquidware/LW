package sbom

import (
	"strings"
	"testing"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/inventory"
)

const fixturePath = "../inventory/testdata/sample.inventory.json"

func testMappings() *cpemap.Mappings {
	method := "string-signature"
	product := "OpenSSL"
	return cpemap.New([]cpemap.Entry{
		{
			Match: cpemap.Match{Method: &method, Product: &product},
			CPE:   cpemap.CPE{Vendor: "openssl", Product: "openssl"},
		},
	})
}

func TestBuild_IsValidCycloneDX16Shape(t *testing.T) {
	inv, err := inventory.Load(fixturePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	doc := Build(inv, testMappings())

	if doc.BomFormat != "CycloneDX" {
		t.Errorf("BomFormat = %q", doc.BomFormat)
	}
	if doc.SpecVersion != "1.6" {
		t.Errorf("SpecVersion = %q", doc.SpecVersion)
	}
	if !strings.HasPrefix(doc.SerialNumber, "urn:uuid:") {
		t.Errorf("SerialNumber = %q", doc.SerialNumber)
	}
	if doc.Metadata.Timestamp == "" {
		t.Error("Metadata.Timestamp should not be empty")
	}
}

func TestBuild_IncludesOnlyResolvedNonExcludedComponents(t *testing.T) {
	inv, err := inventory.Load(fixturePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	doc := Build(inv, testMappings())

	// Fixture has 2 resolved components (jar + string-signature); the
	// excluded kernel32.dll and the unresolved.bin never appear.
	if len(doc.Components) != 2 {
		t.Fatalf("len(Components) = %d, want 2", len(doc.Components))
	}
	names := map[string]bool{}
	for _, c := range doc.Components {
		names[c.Name] = true
	}
	if !names["outer-app"] || !names["OpenSSL"] {
		t.Errorf("names = %v, want outer-app and OpenSSL", names)
	}
}

func TestBuild_JarComponentHasPurlAndHash(t *testing.T) {
	inv, err := inventory.Load(fixturePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	doc := Build(inv, testMappings())

	var jar *Component
	for i := range doc.Components {
		if doc.Components[i].Name == "outer-app" {
			jar = &doc.Components[i]
		}
	}
	if jar == nil {
		t.Fatal("outer-app component not found")
	}
	if jar.Purl != "pkg:maven/com.acme/outer-app@9.9.9" {
		t.Errorf("Purl = %q", jar.Purl)
	}
	if jar.CPE != "" {
		t.Errorf("CPE = %q, want empty", jar.CPE)
	}
	if len(jar.Hashes) != 1 || jar.Hashes[0].Alg != "SHA-256" {
		t.Fatalf("Hashes = %+v", jar.Hashes)
	}
	if jar.Hashes[0].Content != "bc70a2ea1dea659dd82d351ab4f0a9ef9d387ffd3b84491cb4d60cd8cc9bea36" {
		t.Errorf("Hashes[0].Content = %q", jar.Hashes[0].Content)
	}
}

func TestBuild_NativeComponentHasCPENotPurl(t *testing.T) {
	inv, err := inventory.Load(fixturePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	doc := Build(inv, testMappings())

	var native *Component
	for i := range doc.Components {
		if doc.Components[i].Name == "OpenSSL" {
			native = &doc.Components[i]
		}
	}
	if native == nil {
		t.Fatal("OpenSSL component not found")
	}
	if native.Purl != "" {
		t.Errorf("Purl = %q, want empty", native.Purl)
	}
	if native.CPE != "cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*" {
		t.Errorf("CPE = %q", native.CPE)
	}
}

func TestBuild_DedupesIdenticalComponents(t *testing.T) {
	product := "x"
	version := "1.0"
	raw := map[string]any{"groupId": "g", "artifactId": "x", "version": "1.0"}
	inv := &inventory.Inventory{
		Files: []inventory.File{
			{RelativePath: "a/lib.jar", Excluded: false, ComponentType: "jar",
				Identity: &inventory.Identity{Method: "jar-pom-properties", Product: &product, Version: &version, Raw: raw}},
			{RelativePath: "b/lib-copy.jar", Excluded: false, ComponentType: "jar",
				Identity: &inventory.Identity{Method: "jar-pom-properties", Product: &product, Version: &version, Raw: raw}},
		},
	}
	doc := Build(inv, cpemap.New(nil))
	if len(doc.Components) != 1 {
		t.Errorf("len(Components) = %d, want 1", len(doc.Components))
	}
}

func TestBuild_EmptyInventoryHasEmptyComponents(t *testing.T) {
	inv := &inventory.Inventory{}
	doc := Build(inv, cpemap.New(nil))
	if len(doc.Components) != 0 {
		t.Errorf("len(Components) = %d, want 0", len(doc.Components))
	}
}
