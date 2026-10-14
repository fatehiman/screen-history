package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config is stored as config.json next to the executable.
type Config struct {
	IntervalSec   int      `json:"intervalSec"`   // seconds between two captures
	KeepDays      int      `json:"keepDays"`      // 0 = only today, 1 = today + yesterday, ...
	TargetDir     string   `json:"targetDir"`     // one subfolder per day (2006-01-02) is created here
	JPEGQuality   int      `json:"jpegQuality"`   // 1..100
	SkipUnchanged bool     `json:"skipUnchanged"` // do not save when the screen did not change
	StartWithOS   bool     `json:"startWithOS"`   // start automatically after login
	PrivacyTitles []string `json:"privacyTitles"` // auto-pause while the focused window title contains one of these
}

func defaultConfig() Config {
	return Config{
		IntervalSec:   10,
		KeepDays:      2,
		TargetDir:     filepath.Join(exeDir(), "screenshots"),
		JPEGQuality:   70,
		SkipUnchanged: true,
		PrivacyTitles: []string{},
	}
}

func exePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

func exeDir() string {
	p, err := exePath()
	if err != nil {
		return "."
	}
	return filepath.Dir(p)
}

func configPath() string {
	return filepath.Join(exeDir(), "config.json")
}

func loadConfig() Config {
	c := defaultConfig()
	b, err := os.ReadFile(configPath())
	if err == nil {
		_ = json.Unmarshal(b, &c)
	}
	c.normalize()
	return c
}

func (c *Config) normalize() {
	if c.IntervalSec < 1 {
		c.IntervalSec = 1
	}
	if c.KeepDays < 0 {
		c.KeepDays = 0
	}
	if c.JPEGQuality < 10 || c.JPEGQuality > 100 {
		c.JPEGQuality = 70
	}
	if strings.TrimSpace(c.TargetDir) == "" {
		c.TargetDir = defaultConfig().TargetDir
	}
	var titles []string
	for _, t := range c.PrivacyTitles {
		if t = strings.TrimSpace(t); t != "" {
			titles = append(titles, t)
		}
	}
	c.PrivacyTitles = titles
}

func saveConfig(c Config) error {
	c.normalize()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), b, 0o644)
}
