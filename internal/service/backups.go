package service

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func (s *Service) backupPath(id string) (domain.TargetKind, string, error) {
	for _, t := range targets {
		prefix := string(t) + "-"
		if strings.HasPrefix(id, prefix) {
			name := strings.TrimPrefix(id, prefix)
			if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, "/\\") {
				break
			}
			path, e := s.path("backups", string(t), name)
			return t, path, e
		}
	}
	return "", "", domain.NewError("validation_failed", "invalid backup ID")
}
func (s *Service) listBackups() ([]domain.BackupInfo, error) {
	out := []domain.BackupInfo{}
	for _, t := range targets {
		dir, e := s.path("backups", string(t))
		if e != nil {
			return nil, e
		}
		entries, e := os.ReadDir(dir)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return nil, e
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path, e := paths.ValidatePath(filepath.Join(dir, entry.Name()))
			if e != nil {
				return nil, e
			}
			files, e := fileops.InspectBackup(path)
			if e != nil {
				return nil, e
			}
			if len(files) == 0 {
				continue
			}
			names := []string{}
			for _, f := range files {
				if f.Existed {
					names = append(names, f.Name)
				}
			}
			stamp, _ := strconv.ParseInt(strings.Split(entry.Name(), "-")[0], 10, 64)
			if stamp > 1e15 {
				stamp /= 1e6
			}
			row := domain.BackupInfo{ID: string(t) + "-" + entry.Name(), Target: string(t), Dir: path, CreatedAt: stamp, Files: names}
			if e = s.backupMetadata(path, &row); e != nil {
				return nil, e
			}
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}
func (s *Service) ListBackups(ctx context.Context) ([]domain.BackupInfo, error) {
	if e := checkContext(ctx); e != nil {
		return nil, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listBackups()
}
func (s *Service) PreviewBackup(ctx context.Context, id string) (domain.BackupPreview, error) {
	out := domain.BackupPreview{ID: id, Summary: map[string]*string{}, Files: []domain.BackupFileInfo{}}
	if e := checkContext(ctx); e != nil {
		return out, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	target, dir, e := s.backupPath(id)
	if e != nil {
		return out, e
	}
	files, e := fileops.InspectBackup(dir)
	if e != nil {
		return out, e
	}
	name := string(target)
	count := strconv.Itoa(len(files))
	out.Summary["target"] = &name
	out.Summary["files"] = &count
	contents := map[string][]byte{}
	for _, f := range files {
		payload := filepath.Join(dir, f.Name)
		if _, e = s.validatePath(payload); e != nil {
			return out, e
		}
		out.Files = append(out.Files, domain.BackupFileInfo{Name: f.Name, Path: payload})
		if !f.Existed {
			continue
		}
		name := f.Name
		if f.OriginalPath != "" {
			name = filepath.Base(f.OriginalPath)
		}
		if name != "settings.json" && name != "models.json" && name != "auth.json" && name != "config.toml" {
			continue
		}
		raw, e := os.ReadFile(payload)
		if e != nil {
			return out, e
		}
		contents[name] = raw
	}
	summary, e := adapters.Summarize(target, contents)
	if e != nil {
		return out, e
	}
	for k, v := range summary {
		value := v
		out.Summary[k] = &value
	}
	return out, nil
}
func (s *Service) DeleteBackup(ctx context.Context, id string) error {
	if e := checkContext(ctx); e != nil {
		return e
	}
	s.mu.Lock()
	_, dir, e := s.backupPath(id)
	if e == nil {
		e = s.backupRemovable(dir)
		if e == nil {
			e = fileops.DeleteBackup(dir)
		}
		if e == nil {
			if err := os.Remove(dir + ".service.json"); err != nil && !errors.Is(err, os.ErrNotExist) {
				e = err
			}
		}
	}
	s.mu.Unlock()
	if e == nil {
		s.changed()
	}
	return e
}
func (s *Service) prune(target domain.TargetKind, max uint32) error {
	if max < 1 {
		max = 1
	}
	if max > 200 {
		max = 200
	}
	backups, e := s.listBackups()
	if e != nil {
		return e
	}
	count := uint32(0)
	for _, b := range backups {
		if b.Target != string(target) {
			continue
		}
		count++
		if count <= max {
			continue
		}
		if e = s.backupRemovable(b.Dir); e != nil {
			return e
		}
		if e = fileops.DeleteBackup(b.Dir); e != nil {
			return e
		}
		if e = os.Remove(b.Dir + ".service.json"); e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
	}
	return nil
}
func (s *Service) RestoreBackup(ctx context.Context, id string) (err error) {
	if e := checkContext(ctx); e != nil {
		return e
	}
	s.mu.Lock()
	notify := false
	defer func() {
		s.mu.Unlock()
		if notify {
			s.changed()
		}
	}()
	if e := s.recoverPending(); e != nil {
		return publicError(e)
	}
	target, dir, e := s.backupPath(id)
	if e != nil {
		return e
	}
	settings, e := s.st.GetSettings()
	if e != nil {
		return e
	}
	home, e := s.targetHome(target, settings)
	if e != nil {
		return e
	}
	files, e := fileops.InspectBackup(dir)
	if e != nil {
		return e
	}
	dest := map[string]string{}
	allowed := map[string]bool{"settings.json": true}
	if target == domain.Codex {
		allowed = map[string]bool{"config.toml": true, "xiaobai-model-catalog.json": true, "codex.env": true, ".zshrc": true, ".bashrc": true, ".profile": true}
	}
	if target == domain.Pi || target == domain.Prime {
		allowed = map[string]bool{"models.json": true, "auth.json": true, "settings.json": true}
	}
	for _, f := range files {
		name := f.Name
		if strings.HasSuffix(name, ".bak") {
			name = filepath.Base(f.OriginalPath)
		}
		if !allowed[name] {
			continue
		}
		path := filepath.Join(home, name)
		if name == ".zshrc" || name == ".bashrc" || name == ".profile" {
			path = filepath.Join(s.options.Home, name)
		}
		if name == "codex.env" {
			path, e = s.path("env", name)
			if e != nil {
				return e
			}
		}
		if path, e = s.validatePath(path); e != nil {
			return e
		}
		dest[f.Name] = path
	}
	changes, e := fileops.BuildRestore(dir, dest)
	if e != nil {
		return e
	}
	before, e := s.st.GetBinding(target)
	if e != nil {
		return e
	}
	plan := adapters.Plan{Target: target, Changes: changes}
	if before != nil && before.SiteID != nil {
		plan.SiteID = *before.SiteID
	}
	held, e := s.acquire(plan)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	_, e = s.transact(plan, serviceJournal{Operation: "clear", BeforeBinding: before, NoDB: before == nil}, func(string) error {
		recordID := ""
		if before != nil && before.ApplyRecordID != nil {
			recordID = *before.ApplyRecordID
		}
		return s.st.ClearApplication(target, recordID)
	})
	if e != nil && !isCommitted(e) {
		return e
	}
	notify = true
	if e != nil {
		return publicError(e)
	}
	return committedWarning("restore_backup", target, nil, s.prune(target, settings.MaxBackupCopies))
}
