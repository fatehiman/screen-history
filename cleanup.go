package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// dayDirs returns the day folders (named 2006-01-02) inside dir.
func dayDirs(dir string) (map[string]time.Time, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := map[string]time.Time{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d, err := time.ParseInLocation(dayFormat, e.Name(), time.Local)
		if err != nil {
			continue // not one of our folders - never touch it
		}
		out[filepath.Join(dir, e.Name())] = d
	}
	return out, nil
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// cleanupOldDays permanently deletes (no recycle bin, no shredding) every day
// folder older than keepDays. keepDays=0 keeps only today, 1 keeps today and
// yesterday, and so on.
func cleanupOldDays(dir string, keepDays int, now time.Time) error {
	dirs, err := dayDirs(dir)
	if err != nil {
		return err
	}
	oldest := startOfDay(now).AddDate(0, 0, -keepDays)
	var firstErr error
	for path, day := range dirs {
		if day.Before(oldest) {
			if err := os.RemoveAll(path); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// fileTime reads the capture time from the file name, or uses the modify time.
func fileTime(path string, info os.FileInfo) time.Time {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if t, err := time.ParseInLocation(fileFormat, base, time.Local); err == nil {
		return t
	}
	return info.ModTime()
}

// shredRange shreds the files whose capture time is in [from, to).
func shredRange(dir string, from, to time.Time) (int, error) {
	dirs, err := dayDirs(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	var firstErr error
	for path, day := range dirs {
		if day.Before(startOfDay(from)) || !day.Before(to) {
			continue
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			firstErr = errors.Join(firstErr, err)
			continue
		}
		for _, e := range entries {
			if !e.Type().IsRegular() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			f := filepath.Join(path, e.Name())
			if t := fileTime(f, info); t.Before(from) || !t.Before(to) {
				continue
			}
			if err := shredFile(f); err != nil {
				firstErr = errors.Join(firstErr, err)
				continue
			}
			n++
		}
	}
	return n, firstErr
}

// shredDay shreds all files of one day and removes the day folder.
func shredDay(dir string, day time.Time) (int, error) {
	path := filepath.Join(dir, day.Format(dayFormat))
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return shredFolder(path)
}

func shredAll(dir string) (int, error) {
	dirs, err := dayDirs(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	var firstErr error
	for path := range dirs {
		c, err := shredFolder(path)
		n += c
		firstErr = errors.Join(firstErr, err)
	}
	return n, firstErr
}

func shredFolder(path string) (int, error) {
	n := 0
	var firstErr error
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			firstErr = errors.Join(firstErr, err)
			return nil
		}
		if d.Type().IsRegular() {
			if err := shredFile(p); err != nil {
				firstErr = errors.Join(firstErr, err)
			} else {
				n++
			}
		}
		return nil
	})
	firstErr = errors.Join(firstErr, err)
	if firstErr == nil {
		firstErr = os.RemoveAll(path)
	}
	return n, firstErr
}

// shredFile overwrites the file with random bytes, renames it to a random
// name (so the old file name is not left in the folder entry) and deletes it.
//
// This is reliable on a hard disk (HDD). On an SSD or a USB flash drive the
// controller may keep old copies of the blocks (wear leveling), so the data
// can not be guaranteed to be gone there.
func shredFile(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	buf := make([]byte, 256*1024)
	for left := info.Size(); left > 0; {
		chunk := int64(len(buf))
		if left < chunk {
			chunk = left
		}
		if _, err := rand.Read(buf[:chunk]); err != nil {
			f.Close()
			return err
		}
		if _, err := f.Write(buf[:chunk]); err != nil {
			f.Close()
			return err
		}
		left -= chunk
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Truncate(0); err != nil {
		f.Close()
		return err
	}
	f.Close()

	rnd := make([]byte, 8)
	_, _ = rand.Read(rnd)
	tmp := filepath.Join(filepath.Dir(path), hex.EncodeToString(rnd))
	if err := os.Rename(path, tmp); err == nil {
		path = tmp
	}
	return os.Remove(path)
}
