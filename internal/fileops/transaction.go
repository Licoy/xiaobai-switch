package fileops

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Change struct {
	Path    string `json:"path"`
	Before  []byte `json:"-"`
	Existed bool   `json:"-"`
	After   []byte `json:"-"`
	Delete  bool   `json:"delete,omitempty"`
}
type entry struct {
	Path         string `json:"path"`
	Backup       string `json:"backup"`
	Existed      bool   `json:"existed"`
	Applied      bool   `json:"applied"`
	BeforeDigest string `json:"beforeDigest,omitempty"`
	AfterDigest  string `json:"afterDigest,omitempty"`
}
type journal struct {
	State string  `json:"state"`
	Files []entry `json:"files"`
}
type Result struct {
	Journal string   `json:"journal"`
	Paths   []string `json:"paths"`
}

func Read(path string) ([]byte, bool, error) {
	b, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return nil, false, nil
	}
	return b, e == nil, e
}
func Snapshot(path string, after []byte) (Change, error) {
	b, exists, e := Read(path)
	return Change{Path: path, Before: b, Existed: exists, After: after}, e
}
func saveJournal(path string, j journal) error {
	b, e := json.MarshalIndent(j, "", "  ")
	if e != nil {
		return e
	}
	return AtomicWrite(path, append(b, '\n'))
}

// Execute backs up every original before staging or changing any live file.
// Callers must hold the target locks from before building the changes until return.
func Execute(changes []Change, backupDir string) (Result, error) {
	return execute(changes, backupDir, AtomicWrite)
}
func execute(changes []Change, backupDir string, write func(string, []byte) error) (Result, error) {
	result := Result{Journal: filepath.Join(backupDir, "transaction.json")}
	if len(changes) == 0 {
		return result, errors.New("empty transaction")
	}
	if e := os.Mkdir(backupDir, 0700); e != nil {
		return result, e
	}
	if e := SecureDirectory(backupDir); e != nil {
		return result, e
	}
	j := journal{State: "prepared"}
	seen := map[string]bool{}
	for i, c := range changes {
		if !filepath.IsAbs(c.Path) || seen[filepath.Clean(c.Path)] {
			return result, errors.New("transaction paths must be absolute and unique")
		}
		seen[filepath.Clean(c.Path)] = true
		live, exists, e := Read(c.Path)
		if e != nil {
			return result, e
		}
		if exists != c.Existed || !bytes.Equal(live, c.Before) {
			return result, errors.New("config_changed: preview no longer matches live file")
		}
		item := entry{Path: c.Path, Backup: fmt.Sprintf("%03d.bak", i), Existed: exists, BeforeDigest: contentDigest(live, exists), AfterDigest: contentDigest(c.After, !c.Delete)}
		if exists {
			if e = AtomicWrite(filepath.Join(backupDir, item.Backup), live); e != nil {
				return result, fmt.Errorf("backup failed: %w", e)
			}
		}
		j.Files = append(j.Files, item)
	}
	if e := saveJournal(result.Journal, j); e != nil {
		return result, e
	}
	// All payloads are staged privately before the first live replacement.
	for i, c := range changes {
		if !c.Delete {
			if e := AtomicWrite(filepath.Join(backupDir, fmt.Sprintf("%03d.new", i)), c.After); e != nil {
				return result, e
			}
		}
	}
	for i, c := range changes {
		live, exists, checkErr := Read(c.Path)
		if checkErr != nil || exists != c.Existed || !bytes.Equal(live, c.Before) {
			return result, errors.Join(errors.New("config_changed: external writer changed file before replacement"), checkErr, rollback(backupDir, j))
		}
		// Persist intent before replacement so interruption recovery includes this file.
		j.Files[i].Applied = true
		if e := saveJournal(result.Journal, j); e != nil {
			return result, errors.Join(e, rollback(backupDir, j))
		}
		var e error
		if c.Delete {
			e = os.Remove(c.Path)
			if errors.Is(e, os.ErrNotExist) {
				e = nil
			}
		} else {
			e = write(c.Path, c.After)
		}
		if e == nil {
			live, exists, readErr := Read(c.Path)
			e = readErr
			if e == nil && (exists == c.Delete || (!c.Delete && !bytes.Equal(live, c.After))) {
				e = errors.New("post-write verification failed")
			}
		}
		if e != nil {
			return result, errors.Join(e, rollback(backupDir, j))
		}
		result.Paths = append(result.Paths, c.Path)
	}
	j.State = "committed"
	if e := saveJournal(result.Journal, j); e != nil {
		return result, errors.Join(e, rollback(backupDir, j))
	}
	return result, nil
}
func rollback(dir string, j journal) error {
	for _, f := range j.Files {
		if filepath.Base(f.Backup) != f.Backup || !filepath.IsAbs(f.Path) {
			return errors.New("invalid recovery manifest path")
		}
	}
	var out error
	for i := len(j.Files) - 1; i >= 0; i-- {
		f := j.Files[i]
		if !f.Applied {
			continue
		}
		var e error
		if f.AfterDigest != "" {
			live, exists, readErr := Read(f.Path)
			if readErr != nil {
				out = errors.Join(out, readErr)
				continue
			}
			got := contentDigest(live, exists)
			if got != f.AfterDigest && got != f.BeforeDigest {
				out = errors.Join(out, fmt.Errorf("rollback_conflict: external writer changed %s", f.Path))
				continue
			}
		}
		if f.Existed {
			var b []byte
			b, e = os.ReadFile(filepath.Join(dir, f.Backup))
			if e == nil {
				e = AtomicWrite(f.Path, b)
			}
		} else {
			e = os.Remove(f.Path)
			if errors.Is(e, os.ErrNotExist) {
				e = nil
			}
		}
		out = errors.Join(out, e)
	}
	if out != nil {
		return fmt.Errorf("rollback failed: %w", out)
	}
	j.State = "rolled_back"
	return saveJournal(filepath.Join(dir, "transaction.json"), j)
}

// Recover rolls back an interrupted transaction, using only its private journal.
func Recover(dir string) error {
	b, e := os.ReadFile(filepath.Join(dir, "transaction.json"))
	if e != nil {
		return e
	}
	var j journal
	if e = json.Unmarshal(b, &j); e != nil {
		return e
	}
	if j.State == "committed" || j.State == "rolled_back" {
		return nil
	}
	return rollback(dir, j)
}

// Rollback explicitly restores a committed transaction after a later participant fails.
func Rollback(dir string) error {
	b, e := os.ReadFile(filepath.Join(dir, "transaction.json"))
	if e != nil {
		return e
	}
	var j journal
	if e = json.Unmarshal(b, &j); e != nil {
		return e
	}
	if j.State == "rolled_back" {
		return nil
	}
	return rollback(dir, j)
}

func contentDigest(content []byte, exists bool) string {
	if !exists {
		return "missing"
	}
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum)
}
