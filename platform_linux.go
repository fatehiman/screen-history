//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// activeWindowTitle needs xdotool on X11. It returns "" when it is missing.
func activeWindowTitle() string {
	out, err := exec.Command("xdotool", "getactivewindow", "getwindowname").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

var lockFile *os.File

// acquireSingleInstance returns false when another copy is already running.
func acquireSingleInstance() bool {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	f, err := os.OpenFile(filepath.Join(dir, "screen-history.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return true
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return false
	}
	lockFile = f
	return true
}

// setAutostart writes or removes an XDG autostart entry.
func setAutostart(enabled bool) error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "autostart", "screen-history.desktop")
	if !enabled {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	exe, err := exePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	desktop := "[Desktop Entry]\nType=Application\nName=Screen History\nExec=\"" + exe + "\"\nX-GNOME-Autostart-enabled=true\n"
	return os.WriteFile(path, []byte(desktop), 0o644)
}
