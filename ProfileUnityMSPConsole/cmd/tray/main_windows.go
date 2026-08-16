//go:build windows

// Command tray is the double-click entry point on Windows: a small
// system-tray launcher that starts/stops/restarts
// profileunity-msp-console-server.exe (the actual headless server,
// unchanged by any of this) and pops out a live log viewer. Running the
// server directly — from PowerShell, a Scheduled Task, a Windows
// Service — is untouched; this only changes the double-click
// experience.
package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/lxn/walk"
	"golang.org/x/sys/windows"
)

//go:embed icon.ico
var iconBytes []byte

//go:embed logo.png
var logoBytes []byte

const (
	appTitle            = "ProfileUnity MSP Licensing Console"
	serverExeName       = "profileunity-msp-console-server.exe"
	gracefulStopTimeout = 10 * time.Second
	logTailInterval     = 500 * time.Millisecond
	logInitialTailBytes = 50_000
	logViewerMaxRunes   = 500_000
)

type app struct {
	installDir string
	serverPath string
	serverURL  string
	icon       *walk.Icon
	logo       *walk.Bitmap

	mw            *walk.MainWindow
	statusText    *walk.TextLabel
	serverLink    *walk.LinkLabel
	startBtn      *walk.PushButton
	stopBtn       *walk.PushButton
	restartBtn    *walk.PushButton
	changePortBtn *walk.PushButton
	notifyIcon    *walk.NotifyIcon

	logWindow    *walk.MainWindow
	logEdit      *walk.TextEdit
	logTailOnce  sync.Once
	logOffset    int64
	logRuneCount int

	mu  sync.Mutex
	cmd *exec.Cmd
}

// main is a windowsgui-subsystem entry point: there is no console
// attached at all, so anything written to stdout/stderr (fmt.Println,
// an unhandled panic's default crash output) is invisible to whoever
// double-clicked this .exe -- a single failure anywhere in startup
// looks exactly like "nothing happens." Every failure path below goes
// through fatal/warnDialog instead, which show a native message box, so
// a startup problem is diagnosable instead of silent.
func main() {
	defer func() {
		if r := recover(); r != nil {
			fatal("panic", fmt.Errorf("%v\n\n%s", r, debug.Stack()))
		}
	}()

	exePath, err := os.Executable()
	if err != nil {
		fatal("determine install directory", err)
	}
	installDir := filepath.Dir(exePath)

	a := &app{
		installDir: installDir,
		serverPath: filepath.Join(installDir, serverExeName),
		serverURL:  resolveServerURL(installDir),
	}

	if icon, err := loadEmbeddedIcon(); err != nil {
		warnDialog("load icon", err)
	} else {
		a.icon = icon
	}
	if logo, err := loadEmbeddedLogo(); err != nil {
		warnDialog("load logo", err)
	} else {
		a.logo = logo
	}

	if err := a.buildMainWindow(); err != nil {
		fatal("build window", err)
	}
	if err := a.buildNotifyIcon(); err != nil {
		warnDialog("build tray icon", err)
	}

	a.start()
	a.mw.Run()
}

// fatal shows a blocking native message box describing what failed and
// exits -- see main's comment for why this exists instead of printing.
func fatal(context string, err error) {
	showMessageBox(appTitle+" — Startup Error", fmt.Sprintf("Failed to %s:\n\n%v", context, err), windows.MB_ICONERROR)
	os.Exit(1)
}

// warnDialog reports a non-fatal startup problem the same way, without
// exiting -- e.g. a missing icon or tray-icon failure shouldn't stop the
// window itself from showing.
func warnDialog(context string, err error) {
	showMessageBox(appTitle+" — Warning", fmt.Sprintf("Failed to %s:\n\n%v", context, err), windows.MB_ICONWARNING)
}

func showMessageBox(caption, text string, iconFlag uint32) {
	captionPtr, err := windows.UTF16PtrFromString(caption)
	if err != nil {
		return
	}
	textPtr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	windows.MessageBox(0, textPtr, captionPtr, windows.MB_OK|iconFlag)
}

// loadEmbeddedIcon writes the embedded .ico to a temp file and loads it
// from there -- walk has no "load icon from bytes" API, only from a file
// path or a Win32 resource, so a short-lived temp file is the simplest
// bridge. The icon itself is copied into the loaded Icon by Win32
// (LoadImage), so the temp file doesn't need to persist afterward.
func loadEmbeddedIcon() (*walk.Icon, error) {
	f, err := os.CreateTemp("", "pumc-tray-*.ico")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	defer os.Remove(path)

	if _, err := f.Write(iconBytes); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return walk.NewIconFromFile(path)
}

// loadEmbeddedLogo decodes the embedded Liquidware wordmark PNG and
// wraps it as a walk.Bitmap for display in an ImageView -- unlike the
// icon, walk can build a Bitmap directly from a decoded image.Image, no
// temp file needed.
func loadEmbeddedLogo() (*walk.Bitmap, error) {
	img, err := png.Decode(bytes.NewReader(logoBytes))
	if err != nil {
		return nil, err
	}
	return walk.NewBitmapFromImage(img)
}

func (a *app) buildMainWindow() error {
	mw, err := walk.NewMainWindow()
	if err != nil {
		return err
	}
	a.mw = mw
	mw.SetTitle(appTitle)
	if a.icon != nil {
		mw.SetIcon(a.icon)
	}
	mw.SetLayout(walk.NewVBoxLayout())
	mw.SetMinMaxSize(walk.Size{Width: 420, Height: 220}, walk.Size{})
	// walk.MainWindow has no "size to content" default -- without an
	// explicit initial size it opens at whatever default is much larger
	// than this window needs, and VBoxLayout spreads the leftover space
	// evenly across every child since none of them set a stretch factor
	// (hence big gaps between the title/status/buttons instead of a
	// snug window). Widened from 420 to 500 to fit the fourth
	// ("Change Port...") button in the row without crowding.
	mw.SetSize(walk.Size{Width: 500, Height: 260})

	if a.logo != nil {
		logoView, err := walk.NewImageView(mw)
		if err != nil {
			return err
		}
		if err := logoView.SetImage(a.logo); err != nil {
			return err
		}
	}

	titleLabel, err := walk.NewTextLabel(mw)
	if err != nil {
		return err
	}
	titleLabel.SetText(appTitle)

	a.statusText, err = walk.NewTextLabel(mw)
	if err != nil {
		return err
	}

	a.serverLink, err = walk.NewLinkLabel(mw)
	if err != nil {
		return err
	}
	a.serverLink.SetText(fmt.Sprintf(`<a href="%s">%s</a>`, a.serverURL, a.serverURL))
	a.serverLink.LinkActivated().Attach(func(link *walk.LinkLabelLink) {
		openInBrowser(link.URL())
	})

	buttonRow, err := walk.NewComposite(mw)
	if err != nil {
		return err
	}
	buttonRow.SetLayout(walk.NewHBoxLayout())

	a.startBtn, err = walk.NewPushButton(buttonRow)
	if err != nil {
		return err
	}
	a.startBtn.SetText("Start")
	a.startBtn.Clicked().Attach(func() { a.start() })

	a.stopBtn, err = walk.NewPushButton(buttonRow)
	if err != nil {
		return err
	}
	a.stopBtn.SetText("Stop")
	a.stopBtn.Clicked().Attach(func() { a.stop() })

	a.restartBtn, err = walk.NewPushButton(buttonRow)
	if err != nil {
		return err
	}
	a.restartBtn.SetText("Restart")
	a.restartBtn.Clicked().Attach(func() { a.restart() })

	a.changePortBtn, err = walk.NewPushButton(buttonRow)
	if err != nil {
		return err
	}
	a.changePortBtn.SetText("Change Port...")
	a.changePortBtn.Clicked().Attach(func() { a.changePort() })

	// A second row, below the primary lifecycle buttons -- Show Log and
	// Exit are secondary/utility actions, not part of Start/Stop/Restart/
	// Change Port's row.
	secondaryRow, err := walk.NewComposite(mw)
	if err != nil {
		return err
	}
	secondaryRow.SetLayout(walk.NewHBoxLayout())

	logBtn, err := walk.NewPushButton(secondaryRow)
	if err != nil {
		return err
	}
	logBtn.SetText("Show Log")
	logBtn.Clicked().Attach(func() { a.showLog() })

	exitBtn, err := walk.NewPushButton(secondaryRow)
	if err != nil {
		return err
	}
	exitBtn.SetText("Exit")
	exitBtn.Clicked().Attach(func() { a.quit() })

	// Closing the window (the [x] button) just hides it -- "goes to a
	// tray icon when it's running." Exit (this button, or the tray
	// context menu's matching item) is what actually quits the app.
	mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		*canceled = true
		mw.Hide()
	})

	a.setRunningState(false)
	return nil
}

func (a *app) buildNotifyIcon() error {
	ni, err := walk.NewNotifyIcon(a.mw)
	if err != nil {
		return err
	}
	a.notifyIcon = ni
	if a.icon != nil {
		ni.SetIcon(a.icon)
	}
	ni.SetToolTip(appTitle)

	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			a.showWindow()
		}
	})

	menuItem := func(text string, fn func()) error {
		action := walk.NewAction()
		if err := action.SetText(text); err != nil {
			return err
		}
		action.Triggered().Attach(fn)
		return ni.ContextMenu().Actions().Add(action)
	}
	if err := menuItem("Show", func() { a.showWindow() }); err != nil {
		return err
	}
	if err := menuItem("Start", func() { a.start() }); err != nil {
		return err
	}
	if err := menuItem("Stop", func() { a.stop() }); err != nil {
		return err
	}
	if err := menuItem("Restart", func() { a.restart() }); err != nil {
		return err
	}
	if err := menuItem("Change Port...", func() { a.changePort() }); err != nil {
		return err
	}
	if err := menuItem("Show Log", func() { a.showLog() }); err != nil {
		return err
	}
	if err := menuItem("Exit", func() { a.quit() }); err != nil {
		return err
	}

	return ni.SetVisible(true)
}

// openInBrowser launches url in the user's default browser via the
// standard Windows technique for a GUI app that isn't itself a shell
// association handler.
func openInBrowser(url string) {
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func (a *app) showWindow() {
	a.mw.Show()
	a.mw.Activate()
}

// start launches the server as a background process. It's spawned with
// CREATE_NEW_PROCESS_GROUP|CREATE_NO_WINDOW: the child gets its own
// hidden console (so no stray window appears) that's still addressable
// by GenerateConsoleCtrlEvent for a graceful stop -- the standard
// technique for controlling a console-subsystem child from a
// windowsgui-subsystem parent.
func (a *app) start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cmd != nil {
		return
	}

	if _, err := os.Stat(a.serverPath); err != nil {
		a.reportError(fmt.Sprintf("Can't find %s next to the launcher.", serverExeName))
		return
	}

	cmd := exec.Command(a.serverPath)
	cmd.Dir = a.installDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
	}
	if err := cmd.Start(); err != nil {
		a.reportError(fmt.Sprintf("Failed to start the server: %v", err))
		return
	}
	a.cmd = cmd
	a.setRunningState(true)

	go func() {
		cmd.Wait()
		a.mu.Lock()
		if a.cmd == cmd {
			a.cmd = nil
		}
		a.mu.Unlock()
		a.mw.Synchronize(func() { a.setRunningState(false) })
	}()
}

// stop asks the running server to shut down gracefully (matching its
// own signal.NotifyContext/server.Shutdown handling in cmd/server/main.go,
// unchanged) and falls back to a hard kill if it hasn't exited within
// gracefulStopTimeout -- so the button always eventually works even if
// graceful signal delivery has some Windows-specific quirk.
func (a *app) stop() {
	a.mu.Lock()
	cmd := a.cmd
	a.mu.Unlock()
	if cmd == nil {
		return
	}

	windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid))

	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(gracefulStopTimeout):
		cmd.Process.Kill()
		<-done
	}
}

func (a *app) restart() {
	a.stop()
	a.start()
}

func (a *app) quit() {
	a.stop()
	if a.notifyIcon != nil {
		a.notifyIcon.Dispose()
	}
	walk.App().Exit(0)
}

func (a *app) setRunningState(running bool) {
	if running {
		a.statusText.SetText("Status: Running")
		a.startBtn.SetEnabled(false)
		a.stopBtn.SetEnabled(true)
		a.restartBtn.SetEnabled(true)
		if a.notifyIcon != nil {
			a.notifyIcon.SetToolTip(appTitle + " (Running)")
		}
	} else {
		a.statusText.SetText("Status: Stopped")
		a.startBtn.SetEnabled(true)
		a.stopBtn.SetEnabled(false)
		a.restartBtn.SetEnabled(false)
		if a.notifyIcon != nil {
			a.notifyIcon.SetToolTip(appTitle + " (Stopped)")
		}
	}
}

func (a *app) reportError(message string) {
	if a.notifyIcon != nil {
		a.notifyIcon.ShowError(appTitle, message)
		return
	}
	// No tray icon to balloon from (e.g. buildNotifyIcon itself failed) --
	// fall back to a message box so this is still visible rather than
	// only going to a console that doesn't exist.
	showMessageBox(appTitle, message, windows.MB_ICONERROR)
}

// showLog opens (or focuses) the log viewer window and, the first time
// it's shown, starts a background goroutine that keeps it live-updated
// for the rest of the app's lifetime.
func (a *app) showLog() {
	if a.logWindow == nil {
		if err := a.buildLogWindow(); err != nil {
			a.reportError(fmt.Sprintf("Failed to open the log viewer: %v", err))
			return
		}
	}
	a.logWindow.Show()
	a.logWindow.Activate()

	a.logTailOnce.Do(func() { go a.tailLog() })
}

func (a *app) buildLogWindow() error {
	lw, err := walk.NewMainWindow()
	if err != nil {
		return err
	}
	lw.SetTitle(appTitle + " — Log")
	if a.icon != nil {
		lw.SetIcon(a.icon)
	}
	lw.SetLayout(walk.NewVBoxLayout())
	lw.SetMinMaxSize(walk.Size{Width: 500, Height: 300}, walk.Size{})

	edit, err := walk.NewTextEditWithStyle(lw, 0)
	if err != nil {
		return err
	}
	edit.SetReadOnly(true)

	// Hide (not close) on the [x] button too, same as the main window --
	// the tailing goroutine keeps running so reopening it doesn't lose
	// anything.
	lw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		*canceled = true
		lw.Hide()
	})

	a.logWindow = lw
	a.logEdit = edit
	return nil
}

// tailLog polls the resolved log file for appended content and streams
// it into the log viewer. It shows the last logInitialTailBytes on
// first open (so there's immediately something to read, not a blank
// box) and then only ever reads forward from there. If the file shrinks
// (e.g. an operator manually cleared it) it's treated as a fresh file.
func (a *app) tailLog() {
	path := resolveLogFilePath(a.installDir)

	ticker := time.NewTicker(logTailInterval)
	defer ticker.Stop()
	for range ticker.C {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		size := info.Size()
		if size < a.logOffset {
			a.logOffset = 0
		}
		if a.logOffset == 0 && size > logInitialTailBytes {
			a.logOffset = size - logInitialTailBytes
		}
		if size == a.logOffset {
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			continue
		}
		if _, err := f.Seek(a.logOffset, 0); err != nil {
			f.Close()
			continue
		}
		buf := make([]byte, size-a.logOffset)
		n, _ := f.Read(buf)
		f.Close()
		if n == 0 {
			continue
		}
		a.logOffset += int64(n)
		chunk := string(buf[:n])

		a.mw.Synchronize(func() {
			a.logEdit.AppendText(chunk)
			a.logRuneCount += len([]rune(chunk))
			if a.logRuneCount > logViewerMaxRunes {
				// Trim from the front so a long-running app's log
				// viewer doesn't grow without bound; the log FILE
				// itself is untouched, only the on-screen buffer.
				text := []rune(a.logEdit.Text())
				keep := logViewerMaxRunes * 3 / 4
				if len(text) > keep {
					a.logEdit.SetText(string(text[len(text)-keep:]))
					a.logRuneCount = keep
				}
			}
			a.logEdit.ScrollToCaret()
		})
	}
}
