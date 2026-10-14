# Screen History

A tiny tray app that takes a screenshot of **all monitors** every few seconds
and keeps them for a few days. Use it to look back in history: which email you
used in a form, what data you typed before an app failed, and so on.

- One JPG per capture, in one folder per day: `<target>\2026-10-14\2026-10-14 18.23.43.jpg`
- **Skips unchanged screens**: no file is written when the screen did not change.
- **Auto cleanup**: day folders older than *Keep days* are deleted (permanent, no recycle bin).
- **Shred on demand**: delete recent screenshots so they can not be recovered
  (for example after you typed bank data and forgot to pause).
- **Privacy titles**: recording pauses by itself while a window whose title
  contains one of your words is focused.
- Single instance: a second copy shows an error and exits.
- Small: about 7 MB exe, about 20 MB RAM.

## Tray menu

Right-click the camera icon:

| Item | What it does |
|---|---|
| Pause | Check it to stop taking screenshots. The icon turns gray. |
| Settings... | Opens the settings window. |
| Delete (shred) | Recent 10 min, Recent 30 min, Recent 1 hour, Recent 3 hours, Today, Yesterday, All. Today / Yesterday / All ask first. |
| Exit | Closes the app. |

Left-click the icon to open today's folder in Explorer.

The icon is gray while paused (by you, or by a privacy window). The tooltip
shows the state and the last error, if any.

## Settings

Stored in `config.json` next to the exe.

| Setting | Meaning |
|---|---|
| Interval (seconds) | Time between two captures. Default 10. |
| Keep days | `0` = keep only today. `1` = today and yesterday. `2` = today and 2 days before (3 days in total). Older day folders are deleted. Cleanup runs at start, every 30 min and after you save settings. |
| JPEG quality | 10-100. Default 70 (text is still easy to read). |
| Target folder | Where the day folders are created. |
| Do not save when the screen did not change | Compares the new screen with the last saved one and skips it when almost nothing changed (for example a blinking cursor or the clock). |
| Start with Windows | Adds / removes `HKCU\...\Run\ScreenHistory`. |
| Privacy window titles | One per line. Not case sensitive. "contains" match. Example: `bank`, `1Password`, `Bitwarden`. |

## About shredding

Shred = overwrite the file with random data, rename it to a random name, then
delete it.

- On a **hard disk (HDD)** this works: the data can not be recovered.
- On an **SSD or USB flash drive** it is **not guaranteed**. The drive controller
  may keep old copies of the data (wear leveling). Store the screenshots on an
  HDD if you need real shredding.

Automatic cleanup (*Keep days*) does a normal permanent delete, not a shred.

## Build

Needs [Go](https://go.dev) 1.26+ and `go-winres`:

```powershell
go install github.com/tc-hib/go-winres@latest
.\build.ps1
```

Output in `dist\`:

- `screen-history.exe` - Windows
- `screen-history-linux-amd64` - Linux

No C compiler (CGO) is needed for either build.

## Linux

The Linux build is a first version and is not tested yet.

- Tray icon uses the StatusNotifier / DBus tray (most desktops; GNOME needs the AppIndicator extension).
- Screen capture uses X11. Wayland is not supported.
- Settings opens `config.json` in your default editor. Changes are loaded within 3 seconds.
- Privacy titles need `xdotool`. Confirm dialogs and errors use `zenity`.

## Code

| File | Purpose |
|---|---|
| `main.go` | Start, single instance, delete menu items |
| `config.go` | `config.json` load / save |
| `capture.go` | Capture loop, change detection, JPG writing |
| `cleanup.go` | Keep-days cleanup and shredding |
| `platform_windows.go`, `platform_linux.go` | Active window title, single instance lock, autostart |
| `ui_windows.go` | Tray + settings window ([tailscale/walk](https://github.com/tailscale/walk)) |
| `ui_linux.go` | Tray ([fyne.io/systray](https://github.com/fyne-io/systray)) |
| `winres/` | Windows icons, manifest (per-monitor DPI aware) and version info |

## License and credits

GPL-3.0 (see `LICENSE`). The camera icons come from
[AutoScreenshot](https://github.com/artem78/AutoScreenshot), which is also GPL-3.0.
