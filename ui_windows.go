//go:build windows

package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
	"golang.org/x/sys/windows"
)

// showError works also before walk is initialized.
func showError(msg string) {
	t, _ := windows.UTF16PtrFromString(appName)
	m, _ := windows.UTF16PtrFromString(msg)
	windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONERROR|windows.MB_TOPMOST)
}

type trayUI struct {
	rec          *Recorder
	ni           *walk.NotifyIcon
	iconOn       *walk.Icon
	iconOff      *walk.Icon
	pauseAction  *walk.Action
	settingsOpen bool
}

func runUI(cfg Config) {
	app, err := walk.InitApp()
	if err != nil {
		log.Fatal(err)
	}

	ui := &trayUI{}
	if ui.iconOn, err = walk.NewIconFromResource("TRAY"); err != nil {
		log.Fatal(err)
	}
	if ui.iconOff, err = walk.NewIconFromResource("PAUSED"); err != nil {
		log.Fatal(err)
	}
	if ui.ni, err = walk.NewNotifyIcon(); err != nil {
		log.Fatal(err)
	}
	defer ui.ni.Dispose()

	ui.rec = NewRecorder(cfg, func(s State, msg string) {
		walk.App().Synchronize(func() { ui.showState(s, msg) })
	})

	ui.buildMenu()
	ui.showState(StateRunning, "")
	ui.ni.SetVisible(true)

	// left click opens today's folder
	ui.ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			ui.openFolder()
		}
	})

	go ui.rec.Run()
	app.Run()
}

func (ui *trayUI) buildMenu() {
	actions := ui.ni.ContextMenu().Actions()

	ui.pauseAction = walk.NewAction()
	ui.pauseAction.SetText("&Pause")
	ui.pauseAction.SetCheckable(true)
	ui.pauseAction.Triggered().Attach(func() {
		ui.rec.SetPaused(ui.pauseAction.Checked())
	})
	actions.Add(ui.pauseAction)

	settings := walk.NewAction()
	settings.SetText("&Settings...")
	settings.Triggered().Attach(ui.openSettings)
	actions.Add(settings)

	delMenu, err := walk.NewMenu()
	if err != nil {
		log.Fatal(err)
	}
	for _, opt := range deleteOptions {
		opt := opt
		a := walk.NewAction()
		a.SetText(opt.label)
		a.Triggered().Attach(func() { ui.runDelete(opt) })
		delMenu.Actions().Add(a)
	}
	del := walk.NewMenuAction(delMenu)
	del.SetText("&Delete (shred)")
	actions.Add(del)

	actions.Add(walk.NewSeparatorAction())

	exit := walk.NewAction()
	exit.SetText("E&xit")
	exit.Triggered().Attach(func() { walk.App().Exit(0) })
	actions.Add(exit)
}

func (ui *trayUI) showState(s State, msg string) {
	if s == StatePaused || s == StatePrivacy {
		ui.ni.SetIcon(ui.iconOff)
	} else {
		ui.ni.SetIcon(ui.iconOn)
	}
	tip := stateText(s, msg)
	if len(tip) > 120 {
		tip = tip[:120]
	}
	ui.ni.SetToolTip(tip)
	ui.pauseAction.SetChecked(ui.rec.Paused())
}

func (ui *trayUI) openFolder() {
	dir := ui.rec.Config().TargetDir
	today := filepath.Join(dir, time.Now().Format(dayFormat))
	if _, err := os.Stat(today); err == nil {
		dir = today
	}
	exec.Command("explorer.exe", dir).Start()
}

func (ui *trayUI) runDelete(opt deleteOption) {
	if opt.confirm {
		q := "Shred screenshots: " + opt.label + "?\nThey can not be recovered."
		if walk.MsgBox(nil, appName, q, walk.MsgBoxIconWarning|walk.MsgBoxYesNo) != walk.DlgCmdYes {
			return
		}
	}
	go func() {
		n, err := opt.run(ui.rec)
		walk.App().Synchronize(func() {
			if err != nil {
				walk.MsgBox(nil, appName, deleteResultText(n, err), walk.MsgBoxIconError|walk.MsgBoxOK)
				return
			}
			ui.ni.ShowInfo(appName, opt.label+": "+deleteResultText(n, nil))
		})
	}()
}

func (ui *trayUI) openSettings() {
	if ui.settingsOpen {
		return
	}
	ui.settingsOpen = true
	defer func() { ui.settingsOpen = false }()

	cfg := ui.rec.Config()

	var (
		dlg                        *walk.Dialog
		okPB, cancelPB             *walk.PushButton
		intervalNE, keepNE, qualNE *walk.NumberEdit
		folderLE                   *walk.LineEdit
		skipCB, autoCB             *walk.CheckBox
		privacyTE                  *walk.TextEdit
	)

	err := Dialog{
		AssignTo:      &dlg,
		Title:         appName + " - Settings",
		Icon:          ui.iconOn,
		DefaultButton: &okPB,
		CancelButton:  &cancelPB,
		MinSize:       Size{Width: 460},
		Layout:        Grid{Columns: 2},
		Children: []Widget{
			Label{Text: "Interval (seconds):"},
			NumberEdit{AssignTo: &intervalNE, MinValue: 1, MaxValue: 3600, Decimals: 0, Value: float64(cfg.IntervalSec)},

			Label{Text: "Keep days (0 = today only):"},
			NumberEdit{AssignTo: &keepNE, MinValue: 0, MaxValue: 365, Decimals: 0, Value: float64(cfg.KeepDays)},

			Label{Text: "JPEG quality (10-100):"},
			NumberEdit{AssignTo: &qualNE, MinValue: 10, MaxValue: 100, Decimals: 0, Value: float64(cfg.JPEGQuality)},

			Label{Text: "Target folder:"},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					LineEdit{AssignTo: &folderLE, Text: cfg.TargetDir},
					PushButton{Text: "...", MaxSize: Size{Width: 30}, OnClicked: func() {
						fd := walk.FileDialog{Title: "Target folder", InitialDirPath: folderLE.Text()}
						if ok, _ := fd.ShowBrowseFolder(dlg); ok {
							folderLE.SetText(fd.FilePath)
						}
					}},
				},
			},

			CheckBox{AssignTo: &skipCB, ColumnSpan: 2, Checked: cfg.SkipUnchanged,
				Text: "Do not save a screenshot when the screen did not change"},
			CheckBox{AssignTo: &autoCB, ColumnSpan: 2, Checked: cfg.StartWithOS,
				Text: "Start with Windows"},

			Label{ColumnSpan: 2, Text: "Privacy window titles (one per line). Recording pauses while the\nfocused window title contains one of them (not case sensitive):"},
			TextEdit{AssignTo: &privacyTE, ColumnSpan: 2, VScroll: true, MinSize: Size{Height: 60}, MaxSize: Size{Height: 60},
				Text: strings.Join(cfg.PrivacyTitles, "\r\n")},

			Composite{
				ColumnSpan: 2,
				Layout:     HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &okPB, Text: "OK", OnClicked: func() {
						c := cfg
						c.IntervalSec = int(intervalNE.Value())
						c.KeepDays = int(keepNE.Value())
						c.JPEGQuality = int(qualNE.Value())
						c.TargetDir = strings.TrimSpace(folderLE.Text())
						c.SkipUnchanged = skipCB.Checked()
						c.StartWithOS = autoCB.Checked()
						c.PrivacyTitles = strings.Split(strings.ReplaceAll(privacyTE.Text(), "\r", ""), "\n")
						c.normalize()
						if err := ui.applyConfig(c); err != nil {
							walk.MsgBox(dlg, appName, err.Error(), walk.MsgBoxIconError|walk.MsgBoxOK)
							return
						}
						dlg.Accept()
					}},
					PushButton{AssignTo: &cancelPB, Text: "Cancel", OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}.Create(nil)
	if err != nil {
		showError(err.Error())
		return
	}
	dlg.Run()
}

func (ui *trayUI) applyConfig(c Config) error {
	if err := os.MkdirAll(c.TargetDir, 0o755); err != nil {
		return err
	}
	if err := setAutostart(c.StartWithOS); err != nil {
		return err
	}
	if err := saveConfig(c); err != nil {
		return err
	}
	ui.rec.SetConfig(c)
	return nil
}
