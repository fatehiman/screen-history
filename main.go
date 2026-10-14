// Screen History takes a screenshot of all monitors every few seconds, keeps
// them for a few days and can shred recent ones on request.
package main

import (
	"fmt"
	"os"
	"time"
)

const appName = "Screen History"

// deleteOption is one item of the tray "Delete" submenu.
type deleteOption struct {
	label   string
	confirm bool // ask before deleting
	run     func(r *Recorder) (int, error)
}

func recent(d time.Duration) func(r *Recorder) (int, error) {
	return func(r *Recorder) (int, error) { return r.DeleteSince(time.Now().Add(-d)) }
}

var deleteOptions = []deleteOption{
	{"Recent 10 min", false, recent(10 * time.Minute)},
	{"Recent 30 min", false, recent(30 * time.Minute)},
	{"Recent 1 hour", false, recent(time.Hour)},
	{"Recent 3 hours", false, recent(3 * time.Hour)},
	{"Today", true, func(r *Recorder) (int, error) { return r.DeleteDay(time.Now()) }},
	{"Yesterday", true, func(r *Recorder) (int, error) { return r.DeleteDay(time.Now().AddDate(0, 0, -1)) }},
	{"All", true, func(r *Recorder) (int, error) { return r.DeleteAll() }},
}

func stateText(s State, msg string) string {
	switch s {
	case StatePaused:
		return appName + " - paused"
	case StatePrivacy:
		return appName + " - paused (privacy window)"
	case StateError:
		return appName + " - error: " + msg
	default:
		return appName + " - recording"
	}
}

func deleteResultText(n int, err error) string {
	if err != nil {
		return fmt.Sprintf("Shredded %d screenshot(s), but some failed:\n%v", n, err)
	}
	return fmt.Sprintf("Shredded %d screenshot(s).", n)
}

func main() {
	if !acquireSingleInstance() {
		showError(appName + " is already running.")
		os.Exit(1)
	}
	runUI(loadConfig())
}
