//go:build windows

package platform

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/image/bmp"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type win struct {
	// dispatch runs a function on the UI thread. The native file dialog runs a
	// modal message loop and must not be opened from an HTTP handler's goroutine,
	// so it is marshalled onto the WebView's thread. Set by SetDispatcher.
	dispatch func(func())
}

// New returns the Windows platform implementation.
func New() Platform { return &win{} }

// SetDispatcher gives the platform layer a way to run work on the UI thread.
// main wires this to the WebView's Dispatch.
func SetDispatcher(p Platform, dispatch func(func())) {
	if w, ok := p.(*win); ok {
		w.dispatch = dispatch
	}
}

// --- elevation --------------------------------------------------------------

// IsElevated reports whether the process token is elevated. Writing to
// Program Files needs it.
func (*win) IsElevated() (bool, error) {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false, fmt.Errorf("cannot open the process token: %w", err)
	}
	defer token.Close()
	return token.IsElevated(), nil
}

// --- well-known paths -------------------------------------------------------

func (*win) ProgramData() string {
	if d := os.Getenv("ProgramData"); d != "" {
		return d
	}
	return `C:\ProgramData`
}

// DefaultTargetDir is the conventional ProfileUnity Client.NET location. The
// folder and the logo filename are fixed by ProfileUnity, not by this tool.
func (*win) DefaultTargetDir() string {
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		return filepath.Join(pf, "ProfileUnity", "Client.NET")
	}
	return `C:\Program Files\ProfileUnity\Client.NET`
}

// --- native file dialog -----------------------------------------------------

var (
	comdlg32          = windows.NewLazySystemDLL("comdlg32.dll")
	procGetOpenFileNW = comdlg32.NewProc("GetOpenFileNameW")

	shell32            = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW  = shell32.NewProc("ShellExecuteW")
	user32             = windows.NewLazySystemDLL("user32.dll")
	procOpenClipboard  = user32.NewProc("OpenClipboard")
	procCloseClipboard = user32.NewProc("CloseClipboard")
	procGetClipboard   = user32.NewProc("GetClipboardData")
	procIsFormatAvail  = user32.NewProc("IsClipboardFormatAvailable")
	procRegisterFormat = user32.NewProc("RegisterClipboardFormatW")

	kernel32       = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalLock = kernel32.NewProc("GlobalLock")
	procGlobalUnlk = kernel32.NewProc("GlobalUnlock")
	procGlobalSize = kernel32.NewProc("GlobalSize")
)

// openFileName mirrors the Win32 OPENFILENAMEW struct.
type openFileName struct {
	StructSize      uint32
	Owner           uintptr
	Instance        uintptr
	Filter          *uint16
	CustomFilter    *uint16
	MaxCustomFilter uint32
	FilterIndex     uint32
	File            *uint16
	MaxFile         uint32
	FileTitle       *uint16
	MaxFileTitle    uint32
	InitialDir      *uint16
	Title           *uint16
	Flags           uint32
	FileOffset      uint16
	FileExtension   uint16
	DefExt          *uint16
	CustData        uintptr
	FnHook          uintptr
	TemplateName    *uintptr
	PvReserved      uintptr
	DwReserved      uint32
	FlagsEx         uint32
}

const (
	ofnFileMustExist = 0x00001000
	ofnPathMustExist = 0x00000800
	ofnExplorer      = 0x00080000
	ofnNoChangeDir   = 0x00000008
)

// OpenFileDialog shows the native picker on the UI thread and returns the chosen
// path. An empty path with a nil error means the user cancelled.
func (w *win) OpenFileDialog(title string, filter FileFilter) (string, error) {
	type result struct {
		path string
		err  error
	}
	done := make(chan result, 1)

	show := func() {
		path, err := showOpenDialog(title, filter)
		done <- result{path, err}
	}

	if w.dispatch != nil {
		w.dispatch(show)
	} else {
		// No WebView yet (headless/test): run it on a dedicated locked thread.
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			show()
		}()
	}

	r := <-done
	return r.path, r.err
}

// buildFilter encodes a Win32 filter string: label\0patterns\0...\0\0
func buildFilter(f FileFilter) []uint16 {
	patterns := strings.Join(f.Patterns, ";")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s (%s)", f.Label, patterns))
	sb.WriteByte(0)
	sb.WriteString(patterns)
	sb.WriteByte(0)
	sb.WriteString("All files (*.*)")
	sb.WriteByte(0)
	sb.WriteString("*.*")
	sb.WriteByte(0)
	sb.WriteByte(0)

	// utf16.Encode drops embedded NULs, so encode rune by rune.
	out := make([]uint16, 0, sb.Len()+1)
	for _, r := range sb.String() {
		out = append(out, uint16(r))
	}
	return out
}

func showOpenDialog(title string, filter FileFilter) (string, error) {
	buf := make([]uint16, 4096)
	titleW, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return "", err
	}
	filterW := buildFilter(filter)

	ofn := openFileName{
		StructSize: uint32(unsafe.Sizeof(openFileName{})),
		Filter:     &filterW[0],
		File:       &buf[0],
		MaxFile:    uint32(len(buf)),
		Title:      titleW,
		Flags:      ofnFileMustExist | ofnPathMustExist | ofnExplorer | ofnNoChangeDir,
	}

	ret, _, _ := procGetOpenFileNW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		// Zero is either cancellation or an error; CommDlgExtendedError
		// distinguishes them, but treating both as "no selection" is right for
		// the caller either way.
		return "", nil
	}
	return windows.UTF16ToString(buf), nil
}

// --- clipboard --------------------------------------------------------------

const (
	cfDIB   = 8
	cfDIBV5 = 17
)

// ClipboardImagePNG returns the clipboard image as PNG bytes.
//
// Two paths, in order of preference:
//
//  1. The registered "PNG" format. Chromium-based browsers and Firefox put a PNG
//     on the clipboard for "Copy image", and using it means no re-encoding and no
//     loss of alpha.
//  2. CF_DIBV5 / CF_DIB. A device-independent bitmap is a BMP file without its
//     14-byte file header, so the header is synthesised and the result handed to
//     the BMP decoder rather than unpacking pixels by hand.
func (*win) ClipboardImagePNG() ([]byte, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := openClipboardRetry(); err != nil {
		return nil, err
	}
	defer procCloseClipboard.Call()

	if pngFmt := registerFormat("PNG"); pngFmt != 0 && formatAvailable(pngFmt) {
		if data, err := readClipboardBytes(pngFmt); err == nil && len(data) > 0 {
			// Confirm it really decodes before handing it on.
			if _, err := png.Decode(bytes.NewReader(data)); err == nil {
				return data, nil
			}
		}
	}

	for _, f := range []uint32{cfDIBV5, cfDIB} {
		if !formatAvailable(f) {
			continue
		}
		dib, err := readClipboardBytes(f)
		if err != nil || len(dib) < 40 {
			continue
		}
		out, err := pngFromDIB(dib)
		if err != nil {
			continue
		}
		return out, nil
	}

	return nil, ErrNoClipboardImage
}

// openClipboardRetry tries a few times: another process may briefly hold the
// clipboard open, and the browser the user just copied from is a likely culprit.
func openClipboardRetry() error {
	var lastErr error
	for i := 0; i < 10; i++ {
		r, _, err := procOpenClipboard.Call(0)
		if r != 0 {
			return nil
		}
		lastErr = err
		windows.SleepEx(20, false)
	}
	return fmt.Errorf("could not open the clipboard: %w", lastErr)
}

func registerFormat(name string) uint32 {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0
	}
	r, _, _ := procRegisterFormat.Call(uintptr(unsafe.Pointer(p)))
	return uint32(r)
}

func formatAvailable(format uint32) bool {
	r, _, _ := procIsFormatAvail.Call(uintptr(format))
	return r != 0
}

func readClipboardBytes(format uint32) ([]byte, error) {
	h, _, _ := procGetClipboard.Call(uintptr(format))
	if h == 0 {
		return nil, fmt.Errorf("clipboard format %d is not present", format)
	}
	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		return nil, fmt.Errorf("could not lock the clipboard memory")
	}
	defer procGlobalUnlk.Call(h)

	size, _, _ := procGlobalSize.Call(h)
	if size == 0 {
		return nil, fmt.Errorf("clipboard data is empty")
	}
	return copyFromNative(ptr, int(size)), nil
}

// copyFromNative copies size bytes out of memory owned by the Windows clipboard
// into a fresh Go slice.
//
// `go vet`'s unsafeptr check flags the uintptr-to-unsafe.Pointer conversion here,
// and it is a false positive for this API family: GlobalLock returns a pointer
// into memory the clipboard owns, not memory the Go garbage collector can move or
// free, and the value is used immediately while the handle is still locked. There
// is no way to express a Win32 HGLOBAL read without this conversion, so it is
// isolated in this one function and the build excludes only the unsafeptr check
// (see build/build.sh). Everything is copied out before the caller sees it, so no
// native pointer escapes.
func copyFromNative(ptr uintptr, size int) []byte {
	out := make([]byte, size)
	copy(out, unsafe.Slice((*byte)(unsafe.Pointer(ptr)), size))
	return out
}

// pngFromDIB wraps a clipboard DIB in a BMP file header and re-encodes it as PNG.
func pngFromDIB(dib []byte) ([]byte, error) {
	if len(dib) < 40 {
		return nil, fmt.Errorf("device-independent bitmap is too short")
	}
	headerSize := binary.LittleEndian.Uint32(dib[0:4])
	bitCount := binary.LittleEndian.Uint16(dib[14:16])
	clrUsed := binary.LittleEndian.Uint32(dib[32:36])

	// Palette size, for the indexed-colour depths.
	paletteEntries := int(clrUsed)
	if paletteEntries == 0 && bitCount <= 8 {
		paletteEntries = 1 << bitCount
	}
	offBits := 14 + int(headerSize) + paletteEntries*4

	var fileHeader bytes.Buffer
	fileHeader.WriteString("BM")
	_ = binary.Write(&fileHeader, binary.LittleEndian, uint32(14+len(dib)))
	_ = binary.Write(&fileHeader, binary.LittleEndian, uint16(0))
	_ = binary.Write(&fileHeader, binary.LittleEndian, uint16(0))
	_ = binary.Write(&fileHeader, binary.LittleEndian, uint32(offBits))

	full := append(fileHeader.Bytes(), dib...)
	img, err := bmp.Decode(bytes.NewReader(full))
	if err != nil {
		return nil, fmt.Errorf("could not decode the clipboard bitmap: %w", err)
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("could not encode the clipboard image: %w", err)
	}
	return out.Bytes(), nil
}

// --- shell ------------------------------------------------------------------

// OpenInBrowser hands a URL to the shell, which opens the user's real default
// browser. Nothing is embedded and nothing is fetched by this process.
func (*win) OpenInBrowser(rawURL string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(rawURL)
	if err != nil {
		return err
	}
	// ShellExecute returns > 32 on success.
	r, _, callErr := procShellExecuteW.Call(0,
		uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, uintptr(windows.SW_SHOWNORMAL))
	if r <= 32 {
		return fmt.Errorf("the shell could not open the link (code %d): %w", r, callErr)
	}
	return nil
}

// LaunchDetached starts an executable and does not wait for it.
func (*win) LaunchDetached(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("not found: %s", path)
	}
	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap the child so it does not linger as a zombie handle.
	go func() { _ = cmd.Wait() }()
	return nil
}

// --- WebView2 runtime detection ---------------------------------------------

// webViewClientKeys are where the Evergreen runtime records itself. Both the
// per-machine and the WOW6432Node views are checked, since a 64-bit process on a
// 64-bit OS does not see the 32-bit view by default.
var webViewClientKeys = []string{
	`SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`,
	`SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`,
}

// WebViewRuntimeVersion returns the installed WebView2 runtime version, or "" if
// it is not installed. The runtime ships with Windows 11 and with patched
// Windows 10, but a locked-down VDI image may not have it -- which is exactly the
// kind of machine this tool runs on, so it is worth a clear message instead of a
// blank window.
func (*win) WebViewRuntimeVersion() string {
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		for _, path := range webViewClientKeys {
			k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			v, _, err := k.GetStringValue("pv")
			k.Close()
			if err == nil && v != "" && v != "0.0.0.0" {
				return v
			}
		}
	}
	return ""
}

// --- process and user interaction -------------------------------------------

const (
	mbOK            = 0x00000000
	mbIconError     = 0x00000010
	mbIconWarning   = 0x00000030
	mbSystemModal   = 0x00001000
	mbSetForeground = 0x00010000
)

var procMessageBoxW = user32.NewProc("MessageBoxW")

// ShowMessage puts a native message box in front of the user. Used for the
// failures that happen before there is any window to render into -- chiefly a
// missing WebView2 runtime, where the alternative is a blank window and no
// explanation.
func ShowMessage(title, text string, isError bool) {
	titleW, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	textW, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	icon := uintptr(mbIconWarning)
	if isError {
		icon = mbIconError
	}
	procMessageBoxW.Call(0,
		uintptr(unsafe.Pointer(textW)), uintptr(unsafe.Pointer(titleW)),
		mbOK|icon|mbSystemModal|mbSetForeground)
}

// RelaunchElevated restarts this executable with a UAC prompt and returns.
//
// The shipped executable already carries a requireAdmin manifest, so Windows
// elevates before any of this code runs and this path is not reached. It matters
// for a developer build with no manifest, and it keeps the tool from silently
// failing to write to Program Files.
func RelaunchElevated(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	exeW, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	var paramsW *uint16
	if len(args) > 0 {
		paramsW, err = windows.UTF16PtrFromString(quoteArgs(args))
		if err != nil {
			return err
		}
	}
	dirW, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return err
	}

	r, _, callErr := procShellExecuteW.Call(0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exeW)),
		uintptr(unsafe.Pointer(paramsW)),
		uintptr(unsafe.Pointer(dirW)),
		uintptr(windows.SW_SHOWNORMAL))
	if r <= 32 {
		// 1223 is ERROR_CANCELLED: the user declined the UAC prompt.
		return fmt.Errorf("elevation was not granted (code %d): %w", r, callErr)
	}
	return nil
}

// quoteArgs builds a Windows command line, quoting anything containing a space.
func quoteArgs(args []string) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		if strings.ContainsAny(a, " \t\"") {
			parts = append(parts, `"`+strings.ReplaceAll(a, `"`, `\"`)+`"`)
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " ")
}
