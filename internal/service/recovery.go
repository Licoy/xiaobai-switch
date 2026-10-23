package service

import (
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/envinject"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"os"
	"path/filepath"
	"strings"
)

func nativeState(dir string) (string, error) {
	raw, e := os.ReadFile(filepath.Join(dir, "transaction.json"))
	if errors.Is(e, os.ErrNotExist) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	var j struct {
		State string `json:"state"`
	}
	if e = json.Unmarshal(raw, &j); e != nil {
		return "", e
	}
	if j.State != "prepared" && j.State != "committed" && j.State != "rolled_back" {
		return "", domain.NewError("invalid_config", "unknown file transaction state")
	}
	return j.State, nil
}
func (s *Service) recoveryPlan(dir string, target domain.TargetKind, siteID string) (adapters.Plan, error) {
	p := adapters.Plan{Target: target, SiteID: siteID}
	files, e := fileops.InspectBackup(dir)
	if errors.Is(e, os.ErrNotExist) {
		return p, nil
	}
	if e != nil {
		return p, e
	}
	for _, f := range files {
		if _, e = s.validatePath(f.OriginalPath); e != nil {
			return p, e
		}
		p.Changes = append(p.Changes, fileops.Change{Path: f.OriginalPath})
	}
	return p, nil
}
func (s *Service) recoverPending() error {
	root, e := s.path("backups")
	if e != nil {
		return e
	}
	return filepath.WalkDir(root, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() {
			return nil
		}
		if strings.HasSuffix(item.Name(), ".service.json") {
			return s.recoverService(path)
		}
		if item.Name() != "transaction.json" {
			return nil
		}
		dir := filepath.Dir(path)
		if _, e := os.Stat(dir + ".service.json"); e == nil {
			return nil
		} else if !errors.Is(e, os.ErrNotExist) {
			return e
		}
		target := domain.TargetKind(filepath.Base(filepath.Dir(dir)))
		if !target.Valid() {
			return domain.NewError("invalid_config", "native recovery target is unknown")
		}
		return s.recoverNative(dir, target)
	})
}
func (s *Service) recoverNative(dir string, target domain.TargetKind) (err error) {
	if _, e := s.validatePath(dir); e != nil {
		return e
	}
	p, e := s.recoveryPlan(dir, target, "")
	if e != nil {
		return e
	}
	held, e := s.acquire(p)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	return envinject.Recover(dir)
}
func (s *Service) recoverService(path string) (err error) {
	valid, e := s.validatePath(path)
	if e != nil {
		return e
	}
	raw, e := os.ReadFile(valid)
	if e != nil {
		return e
	}
	var j serviceJournal
	if e = json.Unmarshal(raw, &j); e != nil {
		return e
	}
	if !j.Target.Valid() {
		return domain.NewError("invalid_config", "recovery target is unknown")
	}
	if j.Phase != "prepared" && j.Phase != "committed" && j.Phase != "rolled_back" {
		return domain.NewError("invalid_config", "recovery phase is unknown")
	}
	dir := strings.TrimSuffix(valid, ".service.json")
	p, e := s.recoveryPlan(dir, j.Target, j.SiteID)
	if e != nil {
		return e
	}
	held, e := s.acquire(p)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	state, e := nativeState(dir)
	if e != nil {
		return e
	}
	if j.Phase == "rolled_back" {
		return rollbackNative(dir)
	}
	if j.Phase == "committed" {
		if state == "prepared" || state == "rolled_back" {
			return domain.NewError("recovery_conflict", "committed service journal conflicts with file transaction")
		}
		return nil
	}
	committed := false
	if j.NoDB {
		committed = state == "committed"
	} else if j.Operation == "clear" {
		binding, e := s.st.GetBinding(j.Target)
		if e != nil {
			return e
		}
		committed = j.BeforeBinding != nil && binding == nil
	} else {
		record, e := s.st.GetRecord(j.RecordID)
		if e != nil {
			return e
		}
		committed = record != nil && record.Target == string(j.Target) && record.Status == "applied"
	}
	if committed {
		if state == "prepared" || state == "rolled_back" {
			return domain.NewError("recovery_conflict", "database commit conflicts with file transaction")
		}
		j.Phase = "committed"
	} else {
		if e = rollbackNative(dir); e != nil {
			return e
		}
		j.Phase = "rolled_back"
	}
	return s.writeJournal(valid, j)
}
