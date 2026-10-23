package store

import (
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

const recordSelect = `SELECT id,site_id,site_name_snapshot,target,model_id,provider_id,status,backup_dir,error,applied_at,site_api_key_id,site_api_key_label_snapshot,site_api_key_prefix_snapshot FROM apply_records `

func insertRecord(q queryer, r domain.ApplyRecordDTO, t domain.TouchedKeys) error {
	if !domain.TargetKind(r.Target).Valid() {
		return domain.NewError("validation_failed", "unknown target")
	}
	if r.ID == "" {
		return domain.NewError("validation_failed", "apply record ID is required")
	}
	raw, e := json.Marshal(t)
	if e != nil {
		return e
	}
	_, e = q.Exec(`INSERT INTO apply_records(id,site_id,site_name_snapshot,target,model_id,provider_id,status,backup_dir,touched_keys_json,error,applied_at,site_api_key_id,site_api_key_label_snapshot,site_api_key_prefix_snapshot) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, r.ID, r.SiteID, r.SiteNameSnapshot, r.Target, r.ModelID, r.ProviderID, r.Status, r.BackupDir, string(raw), r.Error, r.AppliedAt, r.SiteAPIKeyID, r.SiteAPIKeyLabelSnapshot, r.SiteAPIKeyPrefixSnapshot)
	return e
}
func (s *Store) InsertRecord(r domain.ApplyRecordDTO, t domain.TouchedKeys) error {
	return insertRecord(s.db, r, t)
}
func (s *Store) ListRecords(limit int) ([]domain.ApplyRecordDTO, error) {
	if limit < 1 || limit > 10000 {
		return nil, domain.NewError("validation_failed", "record limit must be between 1 and 10000")
	}
	rows, e := s.db.Query(recordSelect+"ORDER BY applied_at DESC LIMIT ?", limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.ApplyRecordDTO{}
	for rows.Next() {
		var r domain.ApplyRecordDTO
		if e = rows.Scan(&r.ID, &r.SiteID, &r.SiteNameSnapshot, &r.Target, &r.ModelID, &r.ProviderID, &r.Status, &r.BackupDir, &r.Error, &r.AppliedAt, &r.SiteAPIKeyID, &r.SiteAPIKeyLabelSnapshot, &r.SiteAPIKeyPrefixSnapshot); e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetRecord(id string) (*domain.ApplyRecordDTO, error) {
	var r domain.ApplyRecordDTO
	e := s.db.QueryRow(recordSelect+"WHERE id=?", id).Scan(&r.ID, &r.SiteID, &r.SiteNameSnapshot, &r.Target, &r.ModelID, &r.ProviderID, &r.Status, &r.BackupDir, &r.Error, &r.AppliedAt, &r.SiteAPIKeyID, &r.SiteAPIKeyLabelSnapshot, &r.SiteAPIKeyPrefixSnapshot)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &r, nil
}
