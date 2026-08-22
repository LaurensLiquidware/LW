package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"github.com/liquidware/profileunity-splashscreen-manager/internal/version"
)

// CycloneDX 1.6 subset, enough to describe this application accurately.

type cdxHash struct {
	Alg     string `json:"alg"`
	Content string `json:"content"`
}

type cdxLicenseBody struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type cdxLicense struct {
	License    *cdxLicenseBody `json:"license,omitempty"`
	Expression string          `json:"expression,omitempty"`
}

type cdxExternalRef struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type cdxComponent struct {
	Type               string           `json:"type"`
	BOMRef             string           `json:"bom-ref"`
	Name               string           `json:"name"`
	Version            string           `json:"version"`
	Purl               string           `json:"purl,omitempty"`
	Description        string           `json:"description,omitempty"`
	Scope              string           `json:"scope,omitempty"`
	Licenses           []cdxLicense     `json:"licenses,omitempty"`
	Hashes             []cdxHash        `json:"hashes,omitempty"`
	ExternalReferences []cdxExternalRef `json:"externalReferences,omitempty"`
}

type cdxProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type cdxMetadata struct {
	Timestamp  string        `json:"timestamp"`
	Component  cdxComponent  `json:"component"`
	Properties []cdxProperty `json:"properties,omitempty"`
}

type cdxBOM struct {
	BOMFormat    string         `json:"bomFormat"`
	SpecVersion  string         `json:"specVersion"`
	SerialNumber string         `json:"serialNumber"`
	Version      int            `json:"version"`
	Metadata     cdxMetadata    `json:"metadata"`
	Components   []cdxComponent `json:"components"`
}

// primeUILicense is used for every PrimeTek package. Those declare
// "SEE LICENSE IN LICENSE.md" rather than an SPDX identifier, and section 4
// requires a named license in that case rather than an invented SPDX id.
var primeUILicense = cdxLicense{License: &cdxLicenseBody{
	Name: "PrimeUI Commercial License (PrimeTek Informatics), proprietary; see LICENSE.md in the package",
	URL:  "https://primeui.dev/licenses/commercial",
}}

// spdxLicense maps a declared npm license string onto an SBOM license entry.
func spdxLicense(declared string) cdxLicense {
	d := strings.TrimSpace(strings.Trim(declared, `"`))
	if d == "" || strings.HasPrefix(strings.ToUpper(d), "SEE LICENSE") {
		return primeUILicense
	}
	// A compound expression must be recorded as an expression, not an id.
	if strings.ContainsAny(d, " ") && (strings.Contains(d, " OR ") || strings.Contains(d, " AND ")) {
		return cdxLicense{Expression: d}
	}
	return cdxLicense{License: &cdxLicenseBody{ID: d}}
}

var pkgLicensePattern = regexp.MustCompile(`(?m)^Package:\s*(.+?)\s*$\n^License:\s*(.*?)\s*$`)

// bundledNPM reads the packages the Angular build actually bundled, from the
// builder's own attribution file. That is authoritative for what ships: build-only
// dependencies such as the CLI and TypeScript never appear in it.
func bundledNPM(webDir string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(webDir, "dist", "3rdpartylicenses.txt"))
	if err != nil {
		return nil, fmt.Errorf("cannot read the bundled-license manifest: %w", err)
	}
	out := map[string]string{}
	for _, m := range pkgLicensePattern.FindAllStringSubmatch(string(b), -1) {
		out[m[1]] = m[2]
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no packages found in the bundled-license manifest")
	}
	return out, nil
}

// lockVersions resolves exact installed versions from the lockfile, so the SBOM
// records what was actually built rather than a semver range.
func lockVersions(webDir string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(webDir, "package-lock.json"))
	if err != nil {
		return nil, fmt.Errorf("cannot read package-lock.json: %w", err)
	}
	var lock struct {
		Packages map[string]struct {
			Version   string `json:"version"`
			Resolved  string `json:"resolved"`
			Integrity string `json:"integrity"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(b, &lock); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for path, info := range lock.Packages {
		if path == "" || info.Version == "" {
			continue
		}
		// "node_modules/@scope/name" -> "@scope/name"
		idx := strings.LastIndex(path, "node_modules/")
		if idx < 0 {
			continue
		}
		name := path[idx+len("node_modules/"):]
		// A nested copy must not overwrite the top-level resolution.
		if _, exists := out[name]; !exists || !strings.Contains(path[:idx], "node_modules") {
			out[name] = info.Version
		}
	}
	return out, nil
}

var goModPattern = regexp.MustCompile(`(?m)^\s+(?:dep|mod)\s+(\S+)\s+(\S+)`)

// mainModulePath is this application's own module, excluded from the dependency list.
const mainModulePath = "github.com/liquidware/profileunity-splashscreen-manager"

// goModules reads the module list Go stamped into the compiled binary. This is
// authoritative: it is what the linker actually included.
func goModules(exePath string) (map[string]string, error) {
	out, err := exec.Command("go", "version", "-m", exePath).Output()
	if err != nil {
		return nil, fmt.Errorf("cannot read module metadata from the executable: %w", err)
	}
	mods := map[string]string{}
	for _, m := range goModPattern.FindAllStringSubmatch(string(out), -1) {
		name, ver := m[1], m[2]
		if name == "" || ver == "(devel)" {
			continue
		}
		// The main module is the subject of the SBOM (metadata.component), not one
		// of its own dependencies.
		if name == mainModulePath {
			continue
		}
		mods[name] = ver
	}
	return mods, nil
}

// goLicenses records the license of each Go dependency. These are few and stable,
// so they are stated explicitly rather than guessed from the module path; the
// build fails loudly on an unrecognised module so a new dependency cannot slip
// into the SBOM with an unknown license.
// Each value was read from the LICENSE file inside the resolved module, not
// inferred from the module path.
var goLicenses = map[string]string{
	"github.com/jchv/go-webview2":  "MIT",
	"github.com/jchv/go-winloader": "ISC",
	"golang.org/x/image":           "BSD-3-Clause",
	"golang.org/x/sys":             "BSD-3-Clause",
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func newSerialNumber() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// RFC 4122 version 4.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// writeSBOM produces the CycloneDX 1.6 document and returns the component count.
func writeSBOM(repo, webDir, sbomPath, exePath, goarch string, buildTime time.Time) (int, error) {
	npmLicenses, err := bundledNPM(webDir)
	if err != nil {
		return 0, err
	}
	versions, err := lockVersions(webDir)
	if err != nil {
		return 0, err
	}
	mods, err := goModules(exePath)
	if err != nil {
		return 0, err
	}
	exeHash, err := fileSHA256(exePath)
	if err != nil {
		return 0, err
	}
	serial, err := newSerialNumber()
	if err != nil {
		return 0, err
	}

	var components []cdxComponent

	// npm packages bundled into the UI.
	names := make([]string, 0, len(npmLicenses))
	for n := range npmLicenses {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		ver := versions[name]
		if ver == "" {
			return 0, fmt.Errorf("package %q is bundled but has no resolved version in package-lock.json; the SBOM must not record a guess", name)
		}
		components = append(components, cdxComponent{
			Type:     "library",
			BOMRef:   "pkg:npm/" + name + "@" + ver,
			Name:     name,
			Version:  ver,
			Purl:     "pkg:npm/" + name + "@" + ver,
			Scope:    "required",
			Licenses: []cdxLicense{spdxLicense(npmLicenses[name])},
		})
	}

	// Go modules linked into the executable.
	modNames := make([]string, 0, len(mods))
	for n := range mods {
		modNames = append(modNames, n)
	}
	sort.Strings(modNames)
	for _, name := range modNames {
		lic, known := goLicenses[name]
		if !known {
			return 0, fmt.Errorf("Go module %q is linked into the executable but its license is not recorded in cmd/build/sbom.go; add it rather than shipping an SBOM with an unknown license", name)
		}
		ver := mods[name]
		components = append(components, cdxComponent{
			Type:     "library",
			BOMRef:   "pkg:golang/" + name + "@" + ver,
			Name:     name,
			Version:  ver,
			Purl:     "pkg:golang/" + name + "@" + ver,
			Scope:    "required",
			Licenses: []cdxLicense{{License: &cdxLicenseBody{ID: lic}}},
		})
	}

	// go-webview2 embeds Microsoft's WebView2Loader.dll and loads it from memory
	// when it is not present on disk, so a Microsoft redistributable ships inside
	// our executable. Its own LICENSE.txt is BSD-3-Clause, and the binary
	// redistribution clause requires the notice to accompany the distribution --
	// which is what THIRD-PARTY-NOTICES.txt is for.
	if dll, ver, hash, ok := webView2Loader(mods, goarch); ok {
		components = append(components, cdxComponent{
			Type:        "library",
			BOMRef:      "microsoft-webview2-loader",
			Name:        "Microsoft Edge WebView2 Loader (WebView2Loader.dll)",
			Version:     ver,
			Description: "Embedded in the executable by github.com/jchv/go-webview2 and loaded from memory when not present on disk. Part of the Microsoft Edge WebView2 SDK.",
			Scope:       "required",
			Licenses:    []cdxLicense{{License: &cdxLicenseBody{ID: "BSD-3-Clause"}}},
			Hashes:      []cdxHash{{Alg: "SHA-256", Content: hash}},
			ExternalReferences: []cdxExternalRef{
				{Type: "website", URL: "https://developer.microsoft.com/microsoft-edge/webview2/"},
			},
		})
		_ = dll
	}

	// The self-hosted webfont is redistributed, so it belongs here too.
	interPath := filepath.Join(webDir, "public", "fonts", "Inter-roman-var.woff2")
	if _, err := os.Stat(interPath); err == nil {
		interHash, err := fileSHA256(interPath)
		if err != nil {
			return 0, err
		}
		components = append(components, cdxComponent{
			Type:        "file",
			BOMRef:      "inter-variable-font",
			Name:        "Inter",
			Version:     "3.19",
			Description: "Inter variable font (roman), self-hosted so no font is fetched at runtime. Supplied with the Liquidware design system.",
			Scope:       "required",
			Licenses:    []cdxLicense{{License: &cdxLicenseBody{ID: "OFL-1.1"}}},
			Hashes:      []cdxHash{{Alg: "SHA-256", Content: interHash}},
			ExternalReferences: []cdxExternalRef{
				{Type: "website", URL: "https://rsms.me/inter/"},
			},
		})
	}

	bom := cdxBOM{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.6",
		SerialNumber: serial,
		Version:      1,
		Metadata: cdxMetadata{
			Timestamp: buildTime.Format("2006-01-02T15:04:05Z"),
			Component: cdxComponent{
				Type:        "application",
				BOMRef:      "ProfileUnitySplashScreenManager",
				Name:        "ProfileUnitySplashScreenManager",
				Version:     version.AppVersion,
				Description: "Sets the ProfileUnity client splash screen logo (Liquidware KB 12914471137293), with archive and restore history. Go service with an embedded Angular user interface in a WebView2 window.",
				Licenses: []cdxLicense{{License: &cdxLicenseBody{
					Name: "Liquidware Sparks Tool License and Disclaimer v1.0",
				}}},
				Hashes: []cdxHash{{Alg: "SHA-256", Content: exeHash}},
			},
			Properties: []cdxProperty{
				{
					Name:  "liquidware:sbom:scope",
					Value: "Describes the shipped executable. npm entries are the packages the Angular builder actually bundled, taken from its own attribution output, so build-only tooling (the Angular CLI, TypeScript, Tailwind, the compiler) is deliberately excluded: it is not distributed. Go entries are the modules the linker recorded in the binary.",
				},
				{
					Name:  "liquidware:sbom:proprietary-components",
					Value: "The PrimeNG and PrimeUI packages are proprietary (PrimeUI Commercial License), not open source. PrimeNG was MIT through 17.x; 18.x and later are commercial. This requires an escalation under section 4 of the Sparks Tool Project Review Checklist.",
				},
				{
					Name:  "liquidware:build:primeui-license-key-embedded",
					Value: fmt.Sprintf("%t", os.Getenv("PRIMEUI_LICENSE_KEY") != ""),
				},
			},
		},
		Components: components,
	}

	b, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(sbomPath, append(b, '\n'), 0o644); err != nil {
		return 0, err
	}
	return len(components), nil
}

// webView2LoaderPath returns the path of the WebView2Loader.dll that
// go-webview2 embeds for the target architecture, from the module cache.
func webView2LoaderPath(mods map[string]string, goarch string) string {
	ver, ok := mods["github.com/jchv/go-webview2"]
	if !ok {
		return ""
	}
	dir := moduleDir("github.com/jchv/go-webview2", ver)
	if dir == "" {
		return ""
	}
	sub, known := map[string]string{"amd64": "x64", "386": "x86", "arm64": "arm64"}[goarch]
	if !known {
		return ""
	}
	return filepath.Join(dir, "webviewloader", "sdk", sub, "WebView2Loader.dll")
}

// webView2Loader reads the embedded loader's version and hash.
func webView2Loader(mods map[string]string, goarch string) (path, version, hash string, ok bool) {
	p := webView2LoaderPath(mods, goarch)
	if p == "" {
		return "", "", "", false
	}
	if _, err := os.Stat(p); err != nil {
		return "", "", "", false
	}
	h, err := fileSHA256(p)
	if err != nil {
		return "", "", "", false
	}
	return p, peFileVersion(p), h, true
}

// peFileVersion pulls the FileVersion string out of a PE version resource. The
// value is read from the file rather than hardcoded, so an upgrade of
// go-webview2 cannot leave a stale version in the SBOM.
func peFileVersion(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	key := utf16LE("FileVersion")
	i := indexBytes(b, key)
	if i < 0 {
		return "unknown"
	}
	tail := b[i+len(key):]
	if len(tail) > 160 {
		tail = tail[:160]
	}
	// The value follows the key as a padded UTF-16 run.
	var out []rune
	for j := 0; j+1 < len(tail); j += 2 {
		c := rune(tail[j]) | rune(tail[j+1])<<8
		if c == 0 {
			if len(out) > 0 {
				break
			}
			continue
		}
		if c < 0x20 || c > 0x7e {
			break
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func utf16LE(s string) []byte {
	out := make([]byte, 0, len(s)*2)
	for _, r := range s {
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}

func indexBytes(haystack, needle []byte) int {
	return strings.Index(string(haystack), string(needle))
}
