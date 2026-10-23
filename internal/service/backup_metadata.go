package service

import (
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
)

func (s *Service) backupMetadata(dir string, row *domain.BackupInfo) error {
	path, e := s.validatePath(dir + ".service.json")
	if e != nil {
		return e
	}
	raw, e := os.ReadFile(path)
	if e == nil {
		var j serviceJournal
		if e = json.Unmarshal(raw, &j); e != nil {
			return e
		}
		if j.RecordID != "" {
			id := j.RecordID
			row.ApplyRecordID = &id
		}
		if j.SiteNameSnapshot != "" {
			name := j.SiteNameSnapshot
			row.SiteNameSnapshot = &name
		}
		if j.ModelID != "" {
			model := j.ModelID
			row.ModelID = &model
		}
		if j.RecordID != "" {
			record, e := s.st.GetRecord(j.RecordID)
			if e != nil {
				return e
			}
			if record != nil {
				row.SiteNameSnapshot = &record.SiteNameSnapshot
				row.ModelID = &record.ModelID
			}
		}
		return nil
	}
	if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	path, e = s.validatePath(filepath.Join(dir, "xiaobai-backup.json"))
	if e != nil {
		return e
	}
	raw, e = os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	var meta struct {
		SiteName      *string `json:"siteName"`
		ModelID       *string `json:"modelId"`
		ApplyRecordID *string `json:"applyRecordId"`
		CreatedAt     int64   `json:"createdAt"`
	}
	if e = json.Unmarshal(raw, &meta); e != nil {
		return e
	}
	row.SiteNameSnapshot = meta.SiteName
	row.ModelID = meta.ModelID
	row.ApplyRecordID = meta.ApplyRecordID
	if meta.CreatedAt > 0 {
		row.CreatedAt = meta.CreatedAt
	}
	return nil
}
func (s *Service) backupRemovable(dir string) error {
	path, e := s.validatePath(dir + ".service.json")
	if e != nil {
		return e
	}
	raw, e := os.ReadFile(path)
	if e == nil {
		var j serviceJournal
		if e = json.Unmarshal(raw, &j); e != nil {
			return e
		}
		if j.Phase == "prepared" {
			return domain.NewError("backup_busy", "backup has unfinished recovery work")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	state, e := nativeState(dir)
	if e != nil {
		return e
	}
	if state == "prepared" {
		return domain.NewError("backup_busy", "backup has unfinished file recovery")
	}
	return nil
}
