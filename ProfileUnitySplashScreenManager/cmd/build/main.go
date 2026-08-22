// Command build produces the shipping artefacts: the Angular bundle, the embedded
// UI, the Windows executable, the SBOM and the distributable zip.
//
// It lives in Go rather than in a shell script so there is one implementation
// rather than a bash copy and a PowerShell copy drifting apart. build/build.sh
// and build/Build.ps1 are thin wrappers around it.
//
// Steps, in the order section 4/5/6/7 of the Sparks Tool Project Review Checklist
// requires them: version, dependencies, UI, external-reference scan, embed,
// compile, SBOM, package.
package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/liquidware/profileunity-splashscreen-manager/internal/version"
)

func main() {
	var (
		skipUI      = flag.Bool("skip-ui", false, "reuse the existing Angular build instead of rebuilding it")
		skipPackage = flag.Bool("skip-package", false, "build the executable but do not produce the zip")
		goos        = flag.String("goos", "windows", "target operating system")
		goarch      = flag.String("goarch", "amd64", "target architecture")
		outDir      = flag.String("out", "dist", "output directory for the executable and the zip")
		stamp       = flag.String("timestamp", "", "RFC 3339 build timestamp for the SBOM; defaults to now")
	)
	flag.Parse()

	if err := run(*skipUI, *skipPackage, *goos, *goarch, *outDir, *stamp); err != nil {
		fmt.Fprintf(os.Stderr, "\nbuild failed: %v\n", err)
		os.Exit(1)
	}
}

// root is the repository root, resolved from this file's location at runtime.
func root() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Walk up until go.mod is found, so the tool works from any subdirectory.
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod above %s", wd)
		}
		dir = parent
	}
}

func step(n int, total int, msg string) {
	fmt.Printf("\n[%d/%d] %s\n", n, total, msg)
}

func run(skipUI, skipPackage bool, goos, goarch, outDir, stamp string) error {
	repo, err := root()
	if err != nil {
		return err
	}
	if err := os.Chdir(repo); err != nil {
		return err
	}

	buildTime := time.Now().UTC()
	if stamp != "" {
		buildTime, err = time.Parse(time.RFC3339, stamp)
		if err != nil {
			return fmt.Errorf("invalid -timestamp: %w", err)
		}
		buildTime = buildTime.UTC()
	}

	const total = 8
	fmt.Printf("%s %s -> %s/%s\n", version.ProductName, version.AppVersion, goos, goarch)

	step(1, total, "Checking the toolchain")
	if err := checkToolchain(skipUI); err != nil {
		return err
	}

	webDir := filepath.Join(repo, "web")
	if !skipUI {
		step(2, total, "Installing web dependencies")
		if err := npmInstall(webDir); err != nil {
			return err
		}

		step(3, total, "Building the Angular application")
		if err := runCmd(webDir, nil, npmBin(), "run", "build"); err != nil {
			return err
		}
	} else {
		step(2, total, "Skipping dependency install (-skip-ui)")
		step(3, total, "Skipping the Angular build (-skip-ui)")
	}

	distWeb := filepath.Join(webDir, "dist")
	if _, err := os.Stat(filepath.Join(distWeb, "index.html")); err != nil {
		return fmt.Errorf("the Angular build output is missing at %s: %w", distWeb, err)
	}

	step(4, total, "Scanning the built UI for external references (checklist section 3)")
	refs, err := scanExternalRefs(distWeb)
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		fmt.Println("  external references found in runtime assets:")
		for _, r := range refs {
			fmt.Printf("    %s\n", r)
		}
		return errors.New("the built UI references external hosts; section 3 forbids CDN-loaded runtime dependencies")
	}
	fmt.Println("  none in runtime assets (scripts, styles, markup)")

	step(5, total, "Embedding the UI")
	embedDir := filepath.Join(repo, "internal", "static", "ui")
	if err := replaceDir(embedDir, distWeb); err != nil {
		return err
	}
	fmt.Printf("  %s\n", embedDir)

	step(6, total, "Compiling the executable")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	exeName := "ProfileUnitySplashScreenManager"
	if goos == "windows" {
		exeName += ".exe"
	}
	exePath := filepath.Join(outDir, exeName)

	var sysoPath string
	if goos == "windows" {
		sysoPath, err = writeResources(repo, goarch)
		if err != nil {
			return err
		}
		// The .syso must sit beside the main package for the linker to pick it up.
		defer func() {
			if sysoPath != "" {
				_ = os.Remove(sysoPath)
			}
		}()
	}

	ldflags := "-s -w"
	if goos == "windows" {
		// Without this the process gets a console window alongside the app window.
		ldflags = "-H windowsgui " + ldflags
	}
	env := []string{"CGO_ENABLED=0", "GOOS=" + goos, "GOARCH=" + goarch}
	if err := runCmd(repo, env, "go", "build", "-trimpath", "-ldflags="+ldflags, "-o", exePath, "."); err != nil {
		return err
	}
	info, err := os.Stat(exePath)
	if err != nil {
		return err
	}
	fmt.Printf("  %s (%.1f MiB)\n", exePath, float64(info.Size())/(1<<20))

	// The UI is inside the binary now, so put the committed placeholder back and
	// leave the working tree matching what is in version control.
	if err := restorePlaceholder(repo, embedDir); err != nil {
		return err
	}

	step(7, total, "Generating the SBOM and third-party notices (checklist section 4)")
	sbomPath := filepath.Join(repo, "bom.cdx.json")
	count, err := writeSBOM(repo, webDir, sbomPath, exePath, goarch, buildTime)
	if err != nil {
		return err
	}
	fmt.Printf("  %s -- %d components\n", sbomPath, count)

	noticesPath := filepath.Join(repo, "THIRD-PARTY-NOTICES.txt")
	sections, err := writeNotices(repo, webDir, exePath, noticesPath, goarch)
	if err != nil {
		return err
	}
	fmt.Printf("  %s -- %d notices\n", noticesPath, sections)

	if skipPackage {
		step(8, total, "Skipping packaging (-skip-package)")
		fmt.Println("\nDone.")
		return nil
	}

	step(8, total, "Packaging the distributable (checklist section 7)")
	zipPath := filepath.Join(outDir, fmt.Sprintf("ProfileUnitySplashScreenManager-%s.zip", version.AppVersion))
	if err := packageZip(repo, zipPath, exePath); err != nil {
		return err
	}
	zi, err := os.Stat(zipPath)
	if err != nil {
		return err
	}
	fmt.Printf("  %s (%.1f MiB)\n", zipPath, float64(zi.Size())/(1<<20))

	fmt.Printf("\nDone. Remaining before submission:\n")
	fmt.Printf("  * Run Grype against %s (checklist section 5). The SBOM just changed.\n", filepath.Base(sbomPath))
	if os.Getenv("PRIMEUI_LICENSE_KEY") == "" {
		fmt.Printf("  * PRIMEUI_LICENSE_KEY was not set, so this build shows PrimeNG's\n")
		fmt.Printf("    \"Invalid PrimeUI License\" banner. See README.md, \"PrimeNG licensing\".\n")
	}
	return nil
}

// --- toolchain --------------------------------------------------------------

func npmBin() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

func checkToolchain(skipUI bool) error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go is not on PATH")
	}
	out, _ := exec.Command("go", "version").Output()
	fmt.Printf("  %s", out)

	if skipUI {
		return nil
	}
	if _, err := exec.LookPath(npmBin()); err != nil {
		return errors.New("npm is not on PATH; Node.js is required to build the user interface")
	}
	nodeOut, err := exec.Command("node", "--version").Output()
	if err != nil {
		return fmt.Errorf("node is not usable: %w", err)
	}
	nodeVer := strings.TrimSpace(string(nodeOut))
	fmt.Printf("  node %s\n", nodeVer)
	if !nodeAtLeast(nodeVer, 22, 22, 3) {
		return fmt.Errorf("node %s is too old; the Angular CLI used here requires 22.22.3 or newer", nodeVer)
	}
	return nil
}

var nodeVerPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

func nodeAtLeast(v string, major, minor, patch int) bool {
	m := nodeVerPattern.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return false
	}
	var got [3]int
	for i := 0; i < 3; i++ {
		fmt.Sscanf(m[i+1], "%d", &got[i])
	}
	want := [3]int{major, minor, patch}
	for i := 0; i < 3; i++ {
		if got[i] != want[i] {
			return got[i] > want[i]
		}
	}
	return true
}

// npmInstall prefers a lockfile-exact install so builds are reproducible.
func npmInstall(webDir string) error {
	if _, err := os.Stat(filepath.Join(webDir, "package-lock.json")); err == nil {
		if err := runCmd(webDir, nil, npmBin(), "ci"); err == nil {
			return nil
		}
		fmt.Println("  npm ci failed; falling back to npm install")
	}
	return runCmd(webDir, nil, npmBin(), "install")
}

func runCmd(dir string, extraEnv []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), extraEnv...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// --- external reference scan -------------------------------------------------

var externalRefPattern = regexp.MustCompile(`https?://[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]{4,}`)

// allowedRefSubstrings are references that are never dereferenced at runtime:
// XML namespace identifiers and documentation links inside framework error text.
var allowedRefSubstrings = []string{
	"http://www.w3.org",
	"https://angular.dev",
	"https://angular.io",
	"http://localhost",
	"https://primeui.dev/licenses",
}

// scanExternalRefs looks for external hosts in the assets the browser actually
// executes or renders. License and attribution files are excluded: they are text
// about third parties, not fetches, and are required to ship.
func scanExternalRefs(dist string) ([]string, error) {
	var findings []string
	err := filepath.WalkDir(dist, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".js", ".mjs", ".css", ".html", ".json":
		default:
			return nil
		}
		if strings.Contains(strings.ToLower(filepath.Base(path)), "licenses") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, m := range externalRefPattern.FindAll(b, -1) {
			ref := string(m)
			allowed := false
			for _, a := range allowedRefSubstrings {
				if strings.Contains(ref, a) {
					allowed = true
					break
				}
			}
			if allowed || seen[ref] {
				continue
			}
			seen[ref] = true
			rel, _ := filepath.Rel(dist, path)
			if len(ref) > 100 {
				ref = ref[:100] + "..."
			}
			findings = append(findings, fmt.Sprintf("%s: %s", rel, ref))
		}
		return nil
	})
	sort.Strings(findings)
	return findings, err
}

// --- embedding ---------------------------------------------------------------

func replaceDir(dst, src string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		// Not needed at runtime; the Go handler does the routing.
		if rel == "prerendered-routes.json" {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// --- Windows resources -------------------------------------------------------

// versionInfo is the goversioninfo input document.
type versionInfo struct {
	FixedFileInfo struct {
		FileVersion    map[string]int `json:"FileVersion"`
		ProductVersion map[string]int `json:"ProductVersion"`
		FileFlagsMask  string         `json:"FileFlagsMask"`
		FileFlags      string         `json:"FileFlags"`
		FileOS         string         `json:"FileOS"`
		FileType       string         `json:"FileType"`
		FileSubType    string         `json:"FileSubType"`
	} `json:"FixedFileInfo"`
	StringFileInfo map[string]string `json:"StringFileInfo"`
	VarFileInfo    struct {
		Translation map[string]int `json:"Translation"`
	} `json:"VarFileInfo"`
	IconPath     string `json:"IconPath"`
	ManifestPath string `json:"ManifestPath"`
}

// writeResources produces the .syso carrying the icon, the version metadata and
// the requireAdmin manifest, and returns its path.
func writeResources(repo, goarch string) (string, error) {
	var major, minor, patch int
	if _, err := fmt.Sscanf(version.AppVersion, "%d.%d.%d", &major, &minor, &patch); err != nil {
		return "", fmt.Errorf("cannot parse version %q: %w", version.AppVersion, err)
	}

	vi := versionInfo{}
	vi.FixedFileInfo.FileVersion = map[string]int{"Major": major, "Minor": minor, "Patch": patch, "Build": 0}
	vi.FixedFileInfo.ProductVersion = map[string]int{"Major": major, "Minor": minor, "Patch": patch, "Build": 0}
	vi.FixedFileInfo.FileFlagsMask = "3f"
	vi.FixedFileInfo.FileOS = "040004"
	vi.FixedFileInfo.FileType = "01"
	vi.FixedFileInfo.FileSubType = "00"
	vi.StringFileInfo = map[string]string{
		"CompanyName":      version.Company,
		"FileDescription":  version.ProductName,
		"FileVersion":      version.AppVersion,
		"InternalName":     "ProfileUnitySplashScreenManager",
		"LegalCopyright":   version.Company,
		"OriginalFilename": "ProfileUnitySplashScreenManager.exe",
		"ProductName":      version.ProductName,
		"ProductVersion":   version.AppVersion,
	}
	vi.VarFileInfo.Translation = map[string]int{"LangID": 1033, "CharsetID": 1200}
	// The manifest carries an assembly version too, so it is generated from the
	// template rather than holding a second copy of the version string.
	manifestPath, err := writeManifest(repo)
	if err != nil {
		return "", err
	}
	defer os.Remove(manifestPath)

	vi.IconPath = filepath.Join("build", "app-icon.ico")
	vi.ManifestPath = manifestPath

	tmp := filepath.Join(repo, "build", "versioninfo.generated.json")
	b, err := json.MarshalIndent(vi, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return "", err
	}
	defer os.Remove(tmp)

	syso := filepath.Join(repo, "resource_windows.syso")
	args := []string{"run", "github.com/josephspurrier/goversioninfo/cmd/goversioninfo",
		"-o", syso, "-platform-specific=false"}
	if goarch == "amd64" || goarch == "arm64" {
		args = append(args, "-64")
	}
	args = append(args, tmp)
	if err := runCmd(repo, nil, "go", args...); err != nil {
		return "", fmt.Errorf("embedding Windows resources: %w", err)
	}
	fmt.Printf("  resources: icon, version metadata, requireAdmin manifest\n")
	return syso, nil
}

// --- packaging ---------------------------------------------------------------

// payload is what the distributable contains. The license PDF and the SBOM sit
// together at the top level, which section 7 requires.
func payload(repo, exePath string) []string {
	return []string{
		exePath,
		filepath.Join(repo, "Spark_License.pdf"),
		filepath.Join(repo, "bom.cdx.json"),
		filepath.Join(repo, "THIRD-PARTY-NOTICES.txt"),
		filepath.Join(repo, "README.md"),
		filepath.Join(repo, "CHANGELOG.md"),
	}
}

func packageZip(repo, zipPath, exePath string) error {
	files := payload(repo, exePath)
	var missing []string
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			missing = append(missing, filepath.Base(f))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("cannot package, these required files are missing: %s", strings.Join(missing, ", "))
	}

	_ = os.Remove(zipPath)
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()
	for _, f := range files {
		if err := addToZip(zw, f); err != nil {
			return err
		}
	}
	return nil
}

func addToZip(zw *zip.Writer, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.Base(path)
	header.Method = zip.Deflate
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(w, in)
	return err
}

// execCommandOutput runs a command and returns its trimmed stdout.
func execCommandOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// restorePlaceholder returns internal/static/ui to its committed state after the
// real UI has been compiled into the binary, so a build does not leave the
// working tree dirty.
func restorePlaceholder(repo, embedDir string) error {
	src := filepath.Join(repo, "build", "ui-placeholder.html")
	b, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("cannot read the UI placeholder: %w", err)
	}
	if err := os.RemoveAll(embedDir); err != nil {
		return err
	}
	if err := os.MkdirAll(embedDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(embedDir, "index.html"), b, 0o644)
}

// writeManifest renders build/app.manifest.template with the real version and
// returns the path of the generated manifest.
func writeManifest(repo string) (string, error) {
	tmplPath := filepath.Join(repo, "build", "app.manifest.template")
	b, err := os.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("cannot read the manifest template: %w", err)
	}
	out := strings.ReplaceAll(string(b), "{{VERSION}}", version.AppVersion)
	if strings.Contains(out, "{{") {
		return "", fmt.Errorf("the manifest template still contains an unreplaced placeholder")
	}
	dst := filepath.Join(repo, "build", "app.manifest.generated")
	if err := os.WriteFile(dst, []byte(out), 0o644); err != nil {
		return "", err
	}
	return filepath.Join("build", "app.manifest.generated"), nil
}
