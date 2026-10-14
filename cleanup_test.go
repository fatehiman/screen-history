package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeShot(t *testing.T, dir string, at time.Time) string {
	t.Helper()
	d := filepath.Join(dir, at.Format(dayFormat))
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, at.Format(fileFormat)+".jpg")
	if err := os.WriteFile(p, []byte("secret data"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestCleanupOldDays(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.Local)
	today := makeShot(t, dir, now)
	d1 := makeShot(t, dir, now.AddDate(0, 0, -1))
	d2 := makeShot(t, dir, now.AddDate(0, 0, -2))
	d3 := makeShot(t, dir, now.AddDate(0, 0, -3))
	other := filepath.Join(dir, "not-a-day")
	os.MkdirAll(other, 0o755)

	if err := cleanupOldDays(dir, 1, now); err != nil {
		t.Fatal(err)
	}
	if !exists(today) || !exists(d1) || exists(d2) || exists(d3) || !exists(other) {
		t.Fatalf("keepDays=1: today=%v d1=%v d2=%v d3=%v other=%v", exists(today), exists(d1), exists(d2), exists(d3), exists(other))
	}
	if err := cleanupOldDays(dir, 0, now); err != nil {
		t.Fatal(err)
	}
	if !exists(today) || exists(d1) {
		t.Fatal("keepDays=0 must keep only today")
	}
}

func TestShredRange(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := makeShot(t, dir, now.Add(-2*time.Hour))
	recent1 := makeShot(t, dir, now.Add(-5*time.Minute))
	recent2 := makeShot(t, dir, now.Add(-1*time.Minute))

	n, err := shredRange(dir, now.Add(-10*time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || exists(recent1) || exists(recent2) || !exists(old) {
		t.Fatalf("n=%d old=%v r1=%v r2=%v", n, exists(old), exists(recent1), exists(recent2))
	}
	entries, _ := os.ReadDir(filepath.Dir(old))
	if len(entries) != 1 {
		t.Fatalf("expected only the old file left, got %d entries", len(entries))
	}
}

func TestShredDayAndAll(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	makeShot(t, dir, now)
	y := makeShot(t, dir, now.AddDate(0, 0, -1))

	if n, err := shredDay(dir, now); err != nil || n != 1 {
		t.Fatalf("shredDay: n=%d err=%v", n, err)
	}
	if exists(filepath.Join(dir, now.Format(dayFormat))) || !exists(y) {
		t.Fatal("shredDay removed the wrong folder")
	}
	if n, err := shredAll(dir); err != nil || n != 1 {
		t.Fatalf("shredAll: n=%d err=%v", n, err)
	}
}

func TestMatchesPrivacy(t *testing.T) {
	list := []string{"Bank", "1Password"}
	if !matchesPrivacy("My BANK - Online - Google Chrome", list) {
		t.Fatal("should match case-insensitive")
	}
	if matchesPrivacy("GitHub - Google Chrome", list) {
		t.Fatal("should not match")
	}
}
