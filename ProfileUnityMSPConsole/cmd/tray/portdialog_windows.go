//go:build windows

package main

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/lxn/walk"

	"profileunity-msp-console/internal/dotenv"
)

// changePort opens a small modal dialog for typing a new port, since walk
// (this app's only GUI dependency) has no ready-made "input box" API --
// this hand-builds one from a Dialog + LineEdit + PushButtons, the same
// way buildMainWindow/buildLogWindow build their windows. On success it
// writes PUMC_HTTP_ADDR to this install's .env, refreshes the displayed
// console link, and -- if the server is currently running -- offers to
// restart it immediately so the new port takes effect.
func (a *app) changePort() {
	currentAddr := currentHTTPAddr(a.installDir)
	currentHost, currentPort, err := net.SplitHostPort(currentAddr)
	if err != nil {
		currentHost, currentPort = "0.0.0.0", "8443"
	}
	if currentHost == "" {
		currentHost = "0.0.0.0"
	}

	dlg, err := walk.NewDialog(a.mw)
	if err != nil {
		a.reportError(fmt.Sprintf("Failed to open the Change Port dialog: %v", err))
		return
	}
	defer dlg.Dispose()
	dlg.SetTitle(a.title + " — Change Port")
	dlg.SetLayout(walk.NewVBoxLayout())
	dlg.SetMinMaxSize(walk.Size{Width: 320, Height: 160}, walk.Size{})

	label, err := walk.NewTextLabel(dlg)
	if err != nil {
		a.reportError(fmt.Sprintf("Failed to open the Change Port dialog: %v", err))
		return
	}
	label.SetText(fmt.Sprintf("Current port: %s\n\nEnter a new port (1-65535):", currentPort))

	portEdit, err := walk.NewLineEdit(dlg)
	if err != nil {
		a.reportError(fmt.Sprintf("Failed to open the Change Port dialog: %v", err))
		return
	}
	portEdit.SetText(currentPort)

	errorLabel, err := walk.NewTextLabel(dlg)
	if err != nil {
		a.reportError(fmt.Sprintf("Failed to open the Change Port dialog: %v", err))
		return
	}

	buttonRow, err := walk.NewComposite(dlg)
	if err != nil {
		a.reportError(fmt.Sprintf("Failed to open the Change Port dialog: %v", err))
		return
	}
	buttonRow.SetLayout(walk.NewHBoxLayout())

	okBtn, err := walk.NewPushButton(buttonRow)
	if err != nil {
		a.reportError(fmt.Sprintf("Failed to open the Change Port dialog: %v", err))
		return
	}
	okBtn.SetText("OK")
	okBtn.Clicked().Attach(func() {
		newPort, err := validatePort(portEdit.Text())
		if err != nil {
			errorLabel.SetText(err.Error())
			return
		}
		newAddr := net.JoinHostPort(currentHost, newPort)
		if err := dotenv.SetValue(filepath.Join(a.installDir, ".env"), "PUMC_HTTP_ADDR", newAddr); err != nil {
			errorLabel.SetText(fmt.Sprintf("Failed to save: %v", err))
			return
		}
		dlg.Accept()
	})

	cancelBtn, err := walk.NewPushButton(buttonRow)
	if err != nil {
		a.reportError(fmt.Sprintf("Failed to open the Change Port dialog: %v", err))
		return
	}
	cancelBtn.SetText("Cancel")
	cancelBtn.Clicked().Attach(func() { dlg.Cancel() })

	dlg.SetDefaultButton(okBtn)
	dlg.SetCancelButton(cancelBtn)

	if dlg.Run() != walk.DlgCmdOK {
		return
	}

	a.serverURL = resolveServerURL(a.installDir)
	a.serverLink.SetText(fmt.Sprintf(`<a href="%s">%s</a>`, a.serverURL, a.serverURL))

	a.mu.Lock()
	running := a.cmd != nil
	a.mu.Unlock()
	if !running {
		return
	}

	if walk.MsgBox(a.mw, a.title, "Port changed. Restart the server now to use the new port?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes {
		a.restart()
	}
}
