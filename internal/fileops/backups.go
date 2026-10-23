package fileops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type BackupFile struct {
	Name         string `json:"name"`
	OriginalPath string `json:"originalPath,omitempty"`
	Existed      bool   `json:"existed"`
}

// InspectBackup supports native transaction journals and existing Rust backup layouts.
func InspectBackup(dir string) ([]BackupFile, error) {
	raw, e := os.ReadFile(filepath.Join(dir, "transaction.json"))
	if e == nil {
		var j journal
		if e = json.Unmarshal(raw, &j); e != nil {
			return nil, e
		}
		out := []BackupFile{}
		for _, f := range j.Files {
			if filepath.Base(f.Backup) != f.Backup || !filepath.IsAbs(f.Path) {
				return nil, errors.New("invalid backup manifest path")
			}
			if f.Existed {
				info, e := os.Lstat(filepath.Join(dir, f.Backup))
				if e != nil {
					return nil, e
				}
				if !info.Mode().IsRegular() {
					return nil, errors.New("backup payload must be a regular file")
				}
			}
			out = append(out, BackupFile{Name: f.Backup, OriginalPath: f.Path, Existed: f.Existed})
		}
		return out, nil
	}
	if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	origins := map[string]string{}
	raw, e = os.ReadFile(filepath.Join(dir, ".origins.json"))
	if e == nil {
		if e = json.Unmarshal(raw, &origins); e != nil {
			return nil, e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	raw, e = os.ReadFile(filepath.Join(dir, "xiaobai-backup.json"))
	if e == nil {
		var meta struct {
			Files []struct {
				Name         string  `json:"name"`
				OriginalPath *string `json:"originalPath"`
			} `json:"files"`
		}
		if e = json.Unmarshal(raw, &meta); e != nil {
			return nil, e
		}
		for _, f := range meta.Files {
			if f.OriginalPath != nil {
				origins[f.Name] = *f.OriginalPath
			}
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil, e
	}
	out := []BackupFile{}
	for _, item := range entries {
		name := item.Name()
		if item.IsDir() || name == "xiaobai-backup.json" || name == ".origins.json" || strings.HasSuffix(name, ".new") {
			continue
		}
		info, e := item.Info()
		if e != nil {
			return nil, e
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("backup payload must be a regular file")
		}
		out = append(out, BackupFile{Name: name, OriginalPath: origins[name], Existed: true})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// BuildRestore requires an explicit destination map, keeping backup metadata from authorizing writes.
func BuildRestore(dir string, destinations map[string]string) ([]Change, error) {
	files, e := InspectBackup(dir)
	if e != nil {
		return nil, e
	}
	out := []Change{}
	for _, f := range files {
		dest, ok := destinations[f.Name]
		if !ok {
			continue
		}
		if !filepath.IsAbs(dest) {
			return nil, errors.New("backup destination must be absolute")
		}
		var content []byte
		if f.Existed {
			content, e = os.ReadFile(filepath.Join(dir, f.Name))
			if e != nil {
				return nil, e
			}
		}
		c, e := Snapshot(dest, content)
		if e != nil {
			return nil, e
		}
		c.Delete = !f.Existed
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, errors.New("backup has no selected restorable files")
	}
	return out, nil
}

// DeleteBackup removes the transaction directory and its private registry snapshot.
func DeleteBackup(dir string) error {
	info, e := os.Lstat(dir)
	if e != nil {
		return e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("backup must be a directory")
	}
	if e = os.RemoveAll(dir); e != nil {
		return e
	}
	if e = os.Remove(dir + ".user-env.json"); e != nil && !errors.Is(e, os.ErrNotExist) {
		return fmt.Errorf("remove registry backup: %w", e)
	}
	return nil
}
