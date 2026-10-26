package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// keepBackups is how many days of automatic backups are kept.
const keepBackups = 14

// backupDir is where the automatic backups go, beside the data.
func backupDir(path string) string { return filepath.Join(filepath.Dir(path), "Backups") }

// backupDaily copies the data b, once a day, into the backups, and drops
// those beyond the last keepBackups days.
func backupDaily(path string, b []byte, now time.Time) error {
	dir := backupDir(path)
	name := filepath.Join(dir, "data-"+now.Format("2006-01-02")+".json")
	if _, err := os.Stat(name); err == nil {
		return nil
	}
	if err := writeFile(name, b); err != nil {
		return err
	}
	list := backups(path)
	for _, old := range list[min(len(list), keepBackups):] {
		os.Remove(old.path)
	}
	return nil
}

// backup is an automatic backup: its file and the day it was made.
type backup struct {
	path string
	day  time.Time
}

// backups lists the automatic backups, the latest first.
func backups(path string) []backup {
	files, _ := filepath.Glob(filepath.Join(backupDir(path), "data-*.json"))
	var out []backup
	for _, f := range files {
		d, err := time.ParseInLocation("2006-01-02", strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "data-"), ".json"), time.Local)
		if err == nil {
			out = append(out, backup{path: f, day: d})
		}
	}
	slices.SortFunc(out, func(x, y backup) int { return y.day.Compare(x.day) })
	return out
}

// restoreBackup puts the data back as an automatic backup has it, keeping
// the settings of now, with Undo.
func (a *app) restoreBackup(b backup) error {
	raw, err := os.ReadFile(b.path)
	if err != nil {
		return err
	}
	d, err := decodeData(raw)
	if err != nil {
		return err
	}
	lang := a.lang
	d.migrate(func() []*Category { return defaultCategories(lang) })
	step := a.remember()
	d.Prefs = a.data.Prefs
	a.data = d
	a.resetSelection()
	a.changed()
	a.restored = step
	return nil
}
