package store

import (
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

const bindingSelect = `SELECT target,site_id,site_name_snapshot,model_id,provider_id,key_fingerprint,managed_paths_json,managed_env_keys_json,expected_fields_json,orphan,apply_record_id,applied_at,site_api_key_id,site_api_key_label_snapshot,site_api_key_prefix_snapshot FROM target_bindings `

func scanBinding(row scanner) (domain.TargetBinding, error) {
	var b domain.TargetBinding
	var paths, env, expected string
	e := row.Scan(&b.Target, &b.SiteID, &b.SiteNameSnapshot, &b.ModelID, &b.ProviderID, &b.KeyFingerprint, &paths, &env, &expected, &b.Orphan, &b.ApplyRecordID, &b.AppliedAt, &b.APIKey.ID, &b.APIKey.Label, &b.APIKey.Prefix)
	if e != nil {
		return b, e
	}
	if !b.Target.Valid() {
		return b, domain.NewError("invalid_config", "binding has unknown target")
	}
	for _, pair := range []struct {
		raw string
		dst any
	}{{paths, &b.ManagedPaths}, {env, &b.ManagedEnvKeys}, {expected, &b.ExpectedFields}} {
		if e = json.Unmarshal([]byte(pair.raw), pair.dst); e != nil {
			return b, e
		}
	}
	return b, nil
}
func (s *Store) GetBinding(target domain.TargetKind) (*domain.TargetBinding, error) {
	if !target.Valid() {
		return nil, domain.NewError("validation_failed", "unknown target")
	}
	b, e := scanBinding(s.db.QueryRow(bindingSelect+"WHERE target=?", target))
	if e == sql.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &b, nil
}
func (s *Store) ListBindings() ([]domain.TargetBinding, error) {
	rows, e := s.db.Query(bindingSelect + "ORDER BY target")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.TargetBinding{}
	for rows.Next() {
		b, e := scanBinding(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func upsertBinding(q queryer, b domain.TargetBinding) error {
	if !b.Target.Valid() {
		return domain.NewError("validation_failed", "unknown target")
	}
	values := []string{}
	for _, v := range []any{b.ManagedPaths, b.ManagedEnvKeys, b.ExpectedFields} {
		raw, e := json.Marshal(v)
		if e != nil {
			return e
		}
		values = append(values, string(raw))
	}
	_, e := q.Exec(`INSERT INTO target_bindings(target,site_id,site_name_snapshot,model_id,provider_id,key_fingerprint,managed_paths_json,managed_env_keys_json,expected_fields_json,orphan,apply_record_id,applied_at,site_api_key_id,site_api_key_label_snapshot,site_api_key_prefix_snapshot) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(target) DO UPDATE SET site_id=excluded.site_id,site_name_snapshot=excluded.site_name_snapshot,model_id=excluded.model_id,provider_id=excluded.provider_id,key_fingerprint=excluded.key_fingerprint,managed_paths_json=excluded.managed_paths_json,managed_env_keys_json=excluded.managed_env_keys_json,expected_fields_json=excluded.expected_fields_json,orphan=excluded.orphan,apply_record_id=excluded.apply_record_id,applied_at=excluded.applied_at,site_api_key_id=excluded.site_api_key_id,site_api_key_label_snapshot=excluded.site_api_key_label_snapshot,site_api_key_prefix_snapshot=excluded.site_api_key_prefix_snapshot`, b.Target, b.SiteID, b.SiteNameSnapshot, b.ModelID, b.ProviderID, b.KeyFingerprint, values[0], values[1], values[2], b.Orphan, b.ApplyRecordID, b.AppliedAt, b.APIKey.ID, b.APIKey.Label, b.APIKey.Prefix)
	return e
}
func (s *Store) UpsertBinding(b domain.TargetBinding) error { return upsertBinding(s.db, b) }
func (s *Store) DeleteBinding(t domain.TargetKind) error {
	if !t.Valid() {
		return domain.NewError("validation_failed", "unknown target")
	}
	_, e := s.db.Exec("DELETE FROM target_bindings WHERE target=?", t)
	return e
}

// CommitApplication atomically remembers a successful application and its options
func (s *Store) CommitApplication(b domain.TargetBinding, r domain.ApplyRecordDTO, touched domain.TouchedKeys, p domain.TargetPreset) error {
	if r.Status != "applied" {
		return domain.NewError("validation_failed", "only successful applications can update bindings and presets")
	}
	if b.Target != p.Target || r.Target != string(p.Target) || b.SiteID == nil || *b.SiteID != p.SiteID || b.APIKey.ID == nil || *b.APIKey.ID != p.APIKeyID || r.SiteID == nil || *r.SiteID != p.SiteID || r.SiteAPIKeyID == nil || *r.SiteAPIKeyID != p.APIKeyID {
		return domain.NewError("validation_failed", "application identifiers do not match")
	}
	return s.transaction(func(tx *sql.Tx) error {
		if e := insertRecord(tx, r, touched); e != nil {
			return e
		}
		if e := upsertBinding(tx, b); e != nil {
			return e
		}
		return saveTargetPreset(tx, p)
	})
}

// CommitRewrite remembers rewritten managed fields without replacing target options
func (s *Store) CommitRewrite(b domain.TargetBinding, r domain.ApplyRecordDTO, touched domain.TouchedKeys) error {
	if r.Status != "applied" || r.Target != string(b.Target) {
		return domain.NewError("validation_failed", "rewrite requires a matching successful application")
	}
	return s.transaction(func(tx *sql.Tx) error {
		if e := insertRecord(tx, r, touched); e != nil {
			return e
		}
		return upsertBinding(tx, b)
	})
}
