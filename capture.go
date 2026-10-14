package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
)

const (
	dayFormat  = "2006-01-02"
	fileFormat = "2006-01-02 15.04.05"

	// Change detection: compare every sampleStep-th pixel of the new screen with
	// the last saved one. A sample "changed" when its brightness moved more than
	// lumaDelta. The screenshot is saved when at least minChanged samples changed.
	sampleStep = 4
	lumaDelta  = 20
	minChanged = 30
)

// State is what the tray shows.
type State int

const (
	StateRunning State = iota
	StatePaused        // paused by the user
	StatePrivacy       // paused because a privacy window is focused
	StateError
)

// Recorder takes the screenshots and owns all file operations.
type Recorder struct {
	mu        sync.Mutex // guards cfg, paused, state, lastErr
	cfg       Config
	paused    bool
	state     State
	lastErr   string
	fsMu      sync.Mutex // serializes writing and deleting files
	prev      []byte     // samples of the last saved screenshot
	prevW     int
	prevH     int
	reload    chan struct{}
	onState   func(State, string)
	cleanNow  bool      // guarded by mu
	lastClean time.Time // used only by the Run goroutine
}

func NewRecorder(cfg Config, onState func(State, string)) *Recorder {
	return &Recorder{cfg: cfg, reload: make(chan struct{}, 1), onState: onState}
}

func (r *Recorder) Config() Config {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cfg
}

func (r *Recorder) SetConfig(c Config) {
	r.mu.Lock()
	r.cfg = c
	r.cleanNow = true
	r.mu.Unlock()
	select {
	case r.reload <- struct{}{}:
	default:
	}
}

func (r *Recorder) Paused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.paused
}

func (r *Recorder) SetPaused(p bool) {
	r.mu.Lock()
	r.paused = p
	r.mu.Unlock()
	select {
	case r.reload <- struct{}{}:
	default:
	}
}

func (r *Recorder) setState(s State, msg string) {
	r.mu.Lock()
	changed := r.state != s || r.lastErr != msg
	r.state, r.lastErr = s, msg
	r.mu.Unlock()
	if changed && r.onState != nil {
		r.onState(s, msg)
	}
}

// Run blocks forever. Call it in its own goroutine.
func (r *Recorder) Run() {
	for {
		r.tick()
		// A screenshot is a big short-lived buffer (about 15 MB for two
		// monitors). Give that memory back to the OS right away instead of
		// keeping it in the Go heap until the next capture.
		debug.FreeOSMemory()
		wait := time.Duration(r.Config().IntervalSec) * time.Second
		select {
		case <-time.After(wait):
		case <-r.reload:
		}
	}
}

func (r *Recorder) tick() {
	r.mu.Lock()
	cfg, cleanNow := r.cfg, r.cleanNow
	r.cleanNow = false
	r.mu.Unlock()

	if cleanNow || time.Since(r.lastClean) > 30*time.Minute {
		r.lastClean = time.Now()
		r.fsMu.Lock()
		_ = cleanupOldDays(cfg.TargetDir, cfg.KeepDays, time.Now())
		r.fsMu.Unlock()
	}

	if r.Paused() {
		r.setState(StatePaused, "")
		return
	}
	if t := activeWindowTitle(); t != "" && matchesPrivacy(t, cfg.PrivacyTitles) {
		r.setState(StatePrivacy, t)
		return
	}

	if cfg.MinFreeGB > 0 {
		if free, err := freeSpace(existingParent(cfg.TargetDir)); err == nil && float64(free) < cfg.MinFreeGB*(1<<30) {
			// keep the text stable, so the warning balloon is shown only once
			r.setState(StateError, fmt.Sprintf("less than %.1f GB free on the disk, not saving", cfg.MinFreeGB))
			return
		}
	}

	if err := r.capture(cfg); err != nil {
		r.setState(StateError, err.Error())
		return
	}
	r.setState(StateRunning, "")
}

// existingParent returns dir, or its nearest parent that exists, so the free
// space can be checked before the target folder is created.
func existingParent(dir string) string {
	for {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

func matchesPrivacy(title string, list []string) bool {
	t := strings.ToLower(title)
	for _, p := range list {
		if p != "" && strings.Contains(t, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// captureAll returns one image that contains all monitors.
func captureAll() (*image.RGBA, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, fmt.Errorf("no active display")
	}
	all := screenshot.GetDisplayBounds(0)
	for i := 1; i < n; i++ {
		all = all.Union(screenshot.GetDisplayBounds(i))
	}
	return screenshot.CaptureRect(all)
}

func (r *Recorder) capture(cfg Config) error {
	img, err := captureAll()
	if err != nil {
		return fmt.Errorf("capture: %w", err)
	}

	samples, w, h := sampleLuma(img)

	r.fsMu.Lock()
	defer r.fsMu.Unlock()
	if cfg.SkipUnchanged && w == r.prevW && h == r.prevH && countChanged(samples, r.prev) < minChanged {
		return nil
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: cfg.JPEGQuality}); err != nil {
		return fmt.Errorf("jpeg: %w", err)
	}

	now := time.Now()
	dir := filepath.Join(cfg.TargetDir, now.Format(dayFormat))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := filepath.Join(dir, now.Format(fileFormat)+".jpg")
	if err := os.WriteFile(name, buf.Bytes(), 0o644); err != nil {
		return err
	}
	r.prev, r.prevW, r.prevH = samples, w, h
	return nil
}

func sampleLuma(img *image.RGBA) ([]byte, int, int) {
	b := img.Bounds()
	w := (b.Dx() + sampleStep - 1) / sampleStep
	h := (b.Dy() + sampleStep - 1) / sampleStep
	out := make([]byte, 0, w*h)
	for y := 0; y < b.Dy(); y += sampleStep {
		row := img.Pix[y*img.Stride:]
		for x := 0; x < b.Dx(); x += sampleStep {
			p := row[x*4:]
			// fast integer luma: (2R + 5G + B) / 8
			out = append(out, byte((2*int(p[0])+5*int(p[1])+int(p[2]))>>3))
		}
	}
	return out, w, h
}

func countChanged(a, b []byte) int {
	if len(a) != len(b) {
		return len(a)
	}
	n := 0
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d > lumaDelta || d < -lumaDelta {
			n++
			if n >= minChanged {
				return n
			}
		}
	}
	return n
}

// DeleteSince shreds every screenshot taken at or after 'from'.
func (r *Recorder) DeleteSince(from time.Time) (int, error) {
	return r.withFiles(func(dir string) (int, error) { return shredRange(dir, from, time.Now().Add(time.Hour)) })
}

// DeleteDay shreds all screenshots of one day and removes its folder.
func (r *Recorder) DeleteDay(day time.Time) (int, error) {
	return r.withFiles(func(dir string) (int, error) { return shredDay(dir, day) })
}

// DeleteAll shreds every screenshot in the target folder.
func (r *Recorder) DeleteAll() (int, error) {
	return r.withFiles(shredAll)
}

func (r *Recorder) withFiles(fn func(dir string) (int, error)) (int, error) {
	dir := r.Config().TargetDir
	r.fsMu.Lock()
	defer r.fsMu.Unlock()
	// the next capture must be saved even if the screen did not change
	r.prev, r.prevW, r.prevH = nil, 0, 0
	return fn(dir)
}
