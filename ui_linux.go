//go:build linux

package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"time"

	"fyne.io/systray"
)

//go:embed assets/tray.png
var trayPNG []byte

//go:embed assets/paused.png
var pausedPNG []byte

func showError(msg string) {
	if exec.Command("zenity", "--error", "--title="+appName, "--text="+msg).Run() != nil {
		fmt.Fprintln(os.Stderr, msg)
	}
}

// runUI on Linux: tray menu with the same items. "Settings" opens config.json
// in the default editor; changes are picked up automatically.
func runUI(cfg Config) {
	var rec *Recorder
	var pause *systray.MenuItem

	onState := func(s State, msg string) {
		if s == StatePaused || s == StatePrivacy {
			systray.SetIcon(pausedPNG)
		} else {
			systray.SetIcon(trayPNG)
		}
		systray.SetTooltip(stateText(s, msg))
	}
	rec = NewRecorder(cfg, onState)

	systray.Run(func() {
		systray.SetIcon(trayPNG)
		systray.SetTitle(appName)
		systray.SetTooltip(stateText(StateRunning, ""))

		pause = systray.AddMenuItemCheckbox("Pause", "Stop taking screenshots", false)
		settings := systray.AddMenuItem("Settings", "Open config.json")
		del := systray.AddMenuItem("Delete (shred)", "")
		for _, opt := range deleteOptions {
			opt := opt
			item := del.AddSubMenuItem(opt.label, "")
			go func() {
				for range item.ClickedCh {
					if opt.confirm && exec.Command("zenity", "--question", "--title="+appName,
						"--text=Shred screenshots: "+opt.label+"?").Run() != nil {
						continue
					}
					n, err := opt.run(rec)
					if err != nil {
						showError(deleteResultText(n, err))
					}
				}
			}()
		}
		systray.AddSeparator()
		exit := systray.AddMenuItem("Exit", "")

		go func() {
			for {
				select {
				case <-pause.ClickedCh:
					if pause.Checked() {
						pause.Uncheck()
					} else {
						pause.Check()
					}
					rec.SetPaused(pause.Checked())
				case <-settings.ClickedCh:
					if _, err := os.Stat(configPath()); err != nil {
						_ = saveConfig(rec.Config())
					}
					_ = exec.Command("xdg-open", configPath()).Start()
				case <-exit.ClickedCh:
					systray.Quit()
				}
			}
		}()

		go watchConfig(rec)
		go rec.Run()
	}, func() {})
}

// watchConfig reloads config.json when the file changes.
func watchConfig(rec *Recorder) {
	var last time.Time
	if st, err := os.Stat(configPath()); err == nil {
		last = st.ModTime()
	}
	for range time.Tick(3 * time.Second) {
		st, err := os.Stat(configPath())
		if err != nil || !st.ModTime().After(last) {
			continue
		}
		last = st.ModTime()
		c := loadConfig()
		_ = setAutostart(c.StartWithOS)
		rec.SetConfig(c)
	}
}
