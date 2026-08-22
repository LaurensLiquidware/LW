// Package normalize converts a Stage 1 resolved identity into a Package
// URL (purl) or CPE 2.3 string, where possible.
//
// Only identity methods that map cleanly onto an OSV-supported ecosystem
// (Maven, npm, PyPI, NuGet) get a purl. Native/OS components (PE,
// string-signature, dotnet-manifest, electron-embedded) get a CPE
// candidate instead, via BuildCPECandidate -- either a curated
// cpe-mappings.yaml override (confidence "mapped-cpe") or an automatic
// heuristic normalization (confidence "heuristic", never to be presented
// as a confirmed finding).
package normalize

import (
	"regexp"
	"strings"

	"github.com/package-url/packageurl-go"

	"flexapp-vuln-scanner/internal/cpemap"
	"flexapp-vuln-scanner/internal/inventory"
)

// Confidence levels a vulnerability match can carry. Never present a
// heuristic match as a confirmed finding.
const (
	ConfidenceExactPurl = "exact-purl"
	ConfidenceMappedCPE = "mapped-cpe"
	ConfidenceHeuristic = "heuristic"
)

// cpeEligibleMethods are the identity methods a CPE candidate is even
// worth attempting for -- native/OS components with no purl-expressible
// ecosystem. jar-manifest is excluded: a jar with no groupId is still a
// Java library, not a native/OS component, and guessing a CPE for it
// from MANIFEST.MF alone would be pure noise.
var cpeEligibleMethods = map[string]bool{
	"pe-version-resource": true,
	"dotnet-manifest":     true,
	"string-signature":    true,
	"electron-embedded":   true,
}

var (
	corpSuffixes = regexp.MustCompile(`(?i)\b(inc|incorporated|corp|corporation|llc|ltd|limited|gmbh|co)\b\.?`)
	nonAlnumRun  = regexp.MustCompile(`[^a-z0-9]+`)

	// CPE 2.3 formatted-string "special characters" (NIST IR 7695
	// §6.1.2.4) that must be backslash-escaped.
	cpeSpecialChars = regexp.MustCompile(`([!"#$%&'()*+,/:;<=>?@\[\]^` + "`" + `{|}~\\ ])`)

	pypiNameRun = regexp.MustCompile(`[-_.]+`)
)

// heuristicNormalize produces a best-effort CPE-vendor/product-shaped
// string: lowercase, corporate suffixes stripped, everything else
// collapsed to single underscores. This is a guess, not a lookup --
// callers must tag it "heuristic".
func heuristicNormalize(text string) string {
	stripped := corpSuffixes.ReplaceAllString(text, "")
	collapsed := nonAlnumRun.ReplaceAllString(strings.ToLower(stripped), "_")
	return strings.Trim(collapsed, "_")
}

func escapeCPEComponent(text string) string {
	return cpeSpecialChars.ReplaceAllString(text, `\$1`)
}

func normalizePypiName(name string) string {
	return strings.ToLower(pypiNameRun.ReplaceAllString(name, "-"))
}

// splitNPMScope splits an npm package name into (namespace, name) for
// scoped packages: "@scope/pkg" -> ("@scope", "pkg"); "pkg" -> ("", "pkg").
func splitNPMScope(name string) (namespace, rest string) {
	if strings.HasPrefix(name, "@") {
		if i := strings.Index(name, "/"); i >= 0 {
			return name[:i], name[i+1:]
		}
	}
	return "", name
}

func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// BuildPurl returns a purl string for a Stage 1 identity, or "" if this
// identity method doesn't map onto a purl-expressible ecosystem.
func BuildPurl(identity *inventory.Identity) string {
	if identity == nil {
		return ""
	}
	version := str(identity.Version)
	product := str(identity.Product)
	if version == "" || product == "" {
		return ""
	}

	switch identity.Method {
	case "jar-pom-properties":
		groupID, _ := identity.Raw["groupId"].(string)
		artifactID, ok := identity.Raw["artifactId"].(string)
		if !ok || artifactID == "" {
			artifactID = product
		}
		if groupID == "" || artifactID == "" {
			return ""
		}
		return packageurl.NewPackageURL(packageurl.TypeMaven, groupID, artifactID, version, nil, "").ToString()

	case "node-package-json":
		namespace, name := splitNPMScope(product)
		return packageurl.NewPackageURL(packageurl.TypeNPM, namespace, name, version, nil, "").ToString()

	case "python-dist-info":
		return packageurl.NewPackageURL(packageurl.TypePyPi, "", normalizePypiName(product), version, nil, "").ToString()

	case "dotnet-deps-json":
		return packageurl.NewPackageURL(packageurl.TypeNuget, "", product, version, nil, "").ToString()
	}

	// jar-manifest has no groupId, so no Maven purl can be built
	// reliably. dotnet-manifest, pe-version-resource, string-signature,
	// electron-embedded are native/OS components -- not purl-expressible,
	// handled via CPE (BuildCPECandidate, below) instead.
	return ""
}

// BuildCPECandidate returns (cpe23, confidence) for a Stage 1 identity,
// or ("", "") if this identity isn't CPE-eligible or lacks a version.
func BuildCPECandidate(identity *inventory.Identity, mappings *cpemap.Mappings) (cpe23, confidence string) {
	if identity == nil {
		return "", ""
	}
	if !cpeEligibleMethods[identity.Method] {
		return "", ""
	}
	version := str(identity.Version)
	if version == "" {
		return "", ""
	}

	var vendor, product string
	if mVendor, mProduct, ok := mappings.Find(identity); ok {
		vendor, product = mVendor, mProduct
		confidence = ConfidenceMappedCPE
		if pattern, group, ok := mappings.VersionTransform(identity); ok {
			re, err := regexp.Compile(pattern)
			if err == nil {
				// A version that doesn't fit the expected shape falls
				// back to the raw value unchanged -- worst case is a CPE
				// that doesn't match anything in NVD (silently zero
				// findings), not a wrong one.
				if m := re.FindStringSubmatch(version); m != nil && group < len(m) {
					version = m[group]
				}
			}
		}
	} else {
		vendorRaw := str(identity.Vendor)
		if vendorRaw == "" {
			vendorRaw = str(identity.Product)
		}
		productRaw := str(identity.Product)
		if productRaw == "" {
			return "", ""
		}
		vendor = heuristicNormalize(vendorRaw)
		product = heuristicNormalize(productRaw)
		if vendor == "" || product == "" {
			return "", ""
		}
		confidence = ConfidenceHeuristic
	}

	cpe23 = "cpe:2.3:a:" + escapeCPEComponent(vendor) + ":" + escapeCPEComponent(product) + ":" + escapeCPEComponent(version) + ":*:*:*:*:*:*:*"
	return cpe23, confidence
}
