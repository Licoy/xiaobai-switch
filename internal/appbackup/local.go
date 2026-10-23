package appbackup

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func (m *Manager) Resolve(name string) (string, error) {
	if !safeName(name) {
		return "", invalid("invalid local backup file name")
	}
	if e := guard(m.root, m.Dir()); e != nil {
		return "", e
	}
	path := filepath.Join(m.Dir(), name)
	if _, e := regular(path); e != nil {
		return "", e
	}
	return path, nil
}
func (m *Manager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	path, e := m.Resolve(name)
	if e != nil {
		return e
	}
	if e = os.Remove(path); e != nil {
		return e
	}
	return syncDir(m.Dir())
}
func readManifest(path string) (Manifest, error) {
	var m Manifest
	z, e := zip.OpenReader(path)
	if e != nil {
		return m, e
	}
	defer z.Close()
	count := 0
	for _, entry := range z.File {
		if entry.Name != "manifest.json" {
			continue
		}
		count++
		if count > 1 || entry.UncompressedSize64 > uint64(maxManifestBytes) || !entry.Mode().IsRegular() {
			return m, invalid("invalid backup manifest")
		}
		r, e := entry.Open()
		if e != nil {
			return m, e
		}
		raw, e := io.ReadAll(io.LimitReader(r, maxManifestBytes+1))
		ce := r.Close()
		if e = errors.Join(e, ce); e != nil {
			return m, e
		}
		if int64(len(raw)) > maxManifestBytes {
			return m, invalid("manifest exceeds limit")
		}
		if e = json.Unmarshal(raw, &m); e != nil {
			return m, e
		}
	}
	if count != 1 || m.FormatVersion != 1 {
		return m, invalid("missing or unsupported manifest")
	}
	return m, nil
}
func (m *Manager) List() ([]domain.LocalBackupInfo, error) {
	if e := guard(m.root, m.Dir()); e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(m.Dir())
	if errors.Is(e, os.ErrNotExist) {
		return []domain.LocalBackupInfo{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []domain.LocalBackupInfo{}
	for _, entry := range entries {
		if !safeName(entry.Name()) || !entry.Type().IsRegular() {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			return nil, e
		}
		v := domain.LocalBackupInfo{FileName: entry.Name(), Size: uint64(info.Size()), CreatedAt: info.ModTime().UnixMilli(), DeviceName: ParseDeviceFromFilename(entry.Name())}
		mft, e := readManifest(filepath.Join(m.Dir(), entry.Name()))
		if e != nil {
			message := e.Error()
			v.Error = &message
		} else {
			v.CreatedAt = mft.CreatedAt
			v.DeviceName = mft.DeviceName
			v.Reason = &mft.Reason
			v.AppVersion = &mft.AppVersion
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].FileName > out[j].FileName
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out, nil
}
func Prune(dir string, maxCopies uint32) (int, error) {
	if _, e := paths.ValidatePath(dir); e != nil {
		return 0, e
	}
	if info, e := os.Lstat(dir); e == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return 0, invalid("backup directory must be regular")
	} else if e != nil && !os.IsNotExist(e) {
		return 0, e
	}
	entries, e := os.ReadDir(dir)
	if errors.Is(e, os.ErrNotExist) {
		return 0, nil
	}
	if e != nil {
		return 0, e
	}
	type backupFile struct {
		name     string
		modified int64
	}
	files := []backupFile{}
	for _, entry := range entries {
		if entry.Type().IsRegular() && safeName(entry.Name()) {
			info, e := entry.Info()
			if e != nil {
				return 0, e
			}
			files = append(files, backupFile{entry.Name(), info.ModTime().UnixNano()})
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modified == files[j].modified {
			return files[i].name > files[j].name
		}
		return files[i].modified > files[j].modified
	})
	keep := int(min(200, max(1, maxCopies)))
	removed := 0
	for _, file := range files[min(keep, len(files)):] {
		if e = os.Remove(filepath.Join(dir, file.name)); e != nil {
			return removed, e
		}
		removed++
	}
	return removed, syncDir(dir)
}
