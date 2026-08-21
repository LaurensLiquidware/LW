package normalize

import (
	"testing"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/inventory"
)

// The test cases here mirror
// ../../../flexapp-vuln-scanner/stage2-resolve/tests/test_normalize.py
// exactly, for output parity with the Python implementation.

func ident(method string, vendor, product, version *string, raw map[string]any) *inventory.Identity {
	return &inventory.Identity{Method: method, Vendor: vendor, Product: product, Version: version, Raw: raw}
}

func s(v string) *string { return &v }

func TestBuildPurl_MavenPomProperties(t *testing.T) {
	id := ident("jar-pom-properties", s("com.acme"), s("outer-app"), s("9.9.9"),
		map[string]any{"groupId": "com.acme", "artifactId": "outer-app", "version": "9.9.9"})
	if got := BuildPurl(id); got != "pkg:maven/com.acme/outer-app@9.9.9" {
		t.Errorf("BuildPurl = %q", got)
	}
}

func TestBuildPurl_NpmUnscoped(t *testing.T) {
	id := ident("node-package-json", nil, s("lodash"), s("4.17.21"), nil)
	if got := BuildPurl(id); got != "pkg:npm/lodash@4.17.21" {
		t.Errorf("BuildPurl = %q", got)
	}
}

func TestBuildPurl_NpmScoped(t *testing.T) {
	id := ident("node-package-json", nil, s("@angular/core"), s("17.0.0"), nil)
	if got := BuildPurl(id); got != "pkg:npm/%40angular/core@17.0.0" {
		t.Errorf("BuildPurl = %q", got)
	}
}

func TestBuildPurl_PypiNameNormalization(t *testing.T) {
	id := ident("python-dist-info", nil, s("Requests_Toolbelt"), s("1.0.0"), nil)
	if got := BuildPurl(id); got != "pkg:pypi/requests-toolbelt@1.0.0" {
		t.Errorf("BuildPurl = %q", got)
	}
}

func TestBuildPurl_NugetDepsJson(t *testing.T) {
	id := ident("dotnet-deps-json", nil, s("Newtonsoft.Json"), s("13.0.3"), nil)
	if got := BuildPurl(id); got != "pkg:nuget/Newtonsoft.Json@13.0.3" {
		t.Errorf("BuildPurl = %q", got)
	}
}

func TestBuildPurl_JarManifestHasNoPurl(t *testing.T) {
	id := ident("jar-manifest", nil, s("legacy-widget"), s("4.5.6"), map[string]any{})
	if got := BuildPurl(id); got != "" {
		t.Errorf("BuildPurl = %q, want empty", got)
	}
}

func TestBuildPurl_NativePEHasNoPurl(t *testing.T) {
	id := ident("pe-version-resource", s("Acme"), s("Acme Widget"), s("1.0.0"), nil)
	if got := BuildPurl(id); got != "" {
		t.Errorf("BuildPurl = %q, want empty", got)
	}
}

func TestBuildPurl_StringSignatureHasNoPurl(t *testing.T) {
	id := ident("string-signature", nil, s("OpenSSL"), s("1.1.1w"), nil)
	if got := BuildPurl(id); got != "" {
		t.Errorf("BuildPurl = %q, want empty", got)
	}
}

func TestBuildPurl_NoneIdentity(t *testing.T) {
	if got := BuildPurl(nil); got != "" {
		t.Errorf("BuildPurl(nil) = %q, want empty", got)
	}
}

func TestBuildPurl_MissingVersionOrProduct(t *testing.T) {
	if got := BuildPurl(ident("node-package-json", nil, s("x"), nil, nil)); got != "" {
		t.Errorf("BuildPurl = %q, want empty", got)
	}
	if got := BuildPurl(ident("node-package-json", nil, nil, s("1.0.0"), nil)); got != "" {
		t.Errorf("BuildPurl = %q, want empty", got)
	}
}

// -- BuildCPECandidate --------------------------------------------------

var emptyMappings = cpemap.New(nil)

func openSSLMappings() *cpemap.Mappings {
	method := "string-signature"
	product := "OpenSSL"
	return cpemap.New([]cpemap.Entry{
		{
			Match: cpemap.Match{Method: &method, Product: &product},
			CPE:   cpemap.CPE{Vendor: "openssl", Product: "openssl"},
		},
	})
}

func TestBuildCPECandidate_MappedOverrideIsMappedCpeConfidence(t *testing.T) {
	id := ident("string-signature", nil, s("OpenSSL"), s("1.1.1w"), nil)
	cpe, confidence := BuildCPECandidate(id, openSSLMappings())
	if cpe != "cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*" {
		t.Errorf("cpe = %q", cpe)
	}
	if confidence != ConfidenceMappedCPE {
		t.Errorf("confidence = %q", confidence)
	}
}

func TestBuildCPECandidate_HeuristicFallbackStripsCorpSuffix(t *testing.T) {
	id := ident("pe-version-resource", s("Acme Corporation"), s("Acme Widget"), s("1.2.3"), nil)
	cpe, confidence := BuildCPECandidate(id, emptyMappings)
	if cpe != "cpe:2.3:a:acme:acme_widget:1.2.3:*:*:*:*:*:*:*" {
		t.Errorf("cpe = %q", cpe)
	}
	if confidence != ConfidenceHeuristic {
		t.Errorf("confidence = %q", confidence)
	}
}

func TestBuildCPECandidate_PurlExpressibleMethodsReturnEmpty(t *testing.T) {
	id := ident("jar-pom-properties", nil, s("outer-app"), s("9.9.9"), nil)
	cpe, confidence := BuildCPECandidate(id, emptyMappings)
	if cpe != "" || confidence != "" {
		t.Errorf("got (%q, %q), want (\"\", \"\")", cpe, confidence)
	}
}

func TestBuildCPECandidate_NoVersionReturnsEmpty(t *testing.T) {
	id := ident("string-signature", nil, s("OpenSSL"), nil, nil)
	cpe, confidence := BuildCPECandidate(id, emptyMappings)
	if cpe != "" || confidence != "" {
		t.Errorf("got (%q, %q), want (\"\", \"\")", cpe, confidence)
	}
}

func TestBuildCPECandidate_NoneIdentityReturnsEmpty(t *testing.T) {
	cpe, confidence := BuildCPECandidate(nil, emptyMappings)
	if cpe != "" || confidence != "" {
		t.Errorf("got (%q, %q), want (\"\", \"\")", cpe, confidence)
	}
}

func TestEscapeCPEComponent_EscapesColon(t *testing.T) {
	if got := escapeCPEComponent("foo:bar"); got != `foo\:bar` {
		t.Errorf("escapeCPEComponent = %q", got)
	}
}

func TestBuildCPECandidate_VersionWithRawSpaceAndColonIsEscaped(t *testing.T) {
	id := ident("pe-version-resource", s("Google LLC"), s("ANGLE libEGL Dynamic Link Library"),
		s("2.1.23296 git hash: e323abb5b08e"), nil)
	cpe, confidence := BuildCPECandidate(id, emptyMappings)
	want := `cpe:2.3:a:google:angle_libegl_dynamic_link_library:2.1.23296\ git\ hash\:\ e323abb5b08e:*:*:*:*:*:*:*`
	if cpe != want {
		t.Errorf("cpe = %q, want %q", cpe, want)
	}
	if confidence != ConfidenceHeuristic {
		t.Errorf("confidence = %q", confidence)
	}
}

func ffmpegMappings() *cpemap.Mappings {
	product := "FFmpeg"
	return cpemap.New([]cpemap.Entry{
		{
			Match: cpemap.Match{Product: &product},
			CPE: cpemap.CPE{
				Vendor: "ffmpeg", Product: "ffmpeg",
				VersionPattern: `^n?(\d+\.\d+\.\d+)`, VersionGroup: 1,
			},
		},
	})
}

func qtMappings() *cpemap.Mappings {
	product := "Qt6"
	return cpemap.New([]cpemap.Entry{
		{
			Match: cpemap.Match{Product: &product},
			CPE: cpemap.CPE{
				Vendor: "qt", Product: "qt",
				VersionPattern: `^(\d+\.\d+\.\d+)(?:\.\d+)?$`, VersionGroup: 1,
			},
		},
	})
}

func TestBuildCPECandidate_VersionTransformStripsFFmpegGitTagPrefix(t *testing.T) {
	id := ident("pe-version-resource", nil, s("FFmpeg"), s("n7.1.1"), nil)
	cpe, confidence := BuildCPECandidate(id, ffmpegMappings())
	if cpe != "cpe:2.3:a:ffmpeg:ffmpeg:7.1.1:*:*:*:*:*:*:*" {
		t.Errorf("cpe = %q", cpe)
	}
	if confidence != ConfidenceMappedCPE {
		t.Errorf("confidence = %q", confidence)
	}
}

func TestBuildCPECandidate_VersionTransformDropsQtFourthSegment(t *testing.T) {
	id := ident("pe-version-resource", nil, s("Qt6"), s("6.8.3.0"), nil)
	cpe, confidence := BuildCPECandidate(id, qtMappings())
	if cpe != "cpe:2.3:a:qt:qt:6.8.3:*:*:*:*:*:*:*" {
		t.Errorf("cpe = %q", cpe)
	}
	if confidence != ConfidenceMappedCPE {
		t.Errorf("confidence = %q", confidence)
	}
}

func TestBuildCPECandidate_VersionTransformFallsBackWhenPatternDoesNotMatch(t *testing.T) {
	id := ident("pe-version-resource", nil, s("FFmpeg"), s("unknown-build"), nil)
	cpe, confidence := BuildCPECandidate(id, ffmpegMappings())
	if cpe != "cpe:2.3:a:ffmpeg:ffmpeg:unknown-build:*:*:*:*:*:*:*" {
		t.Errorf("cpe = %q", cpe)
	}
	if confidence != ConfidenceMappedCPE {
		t.Errorf("confidence = %q", confidence)
	}
}
