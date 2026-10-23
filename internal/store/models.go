package store

import (
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"strings"
)

func readModels(q queryer, keyID string) ([]domain.SiteModelDTO, error) {
	rows, e := q.Query("SELECT id,site_id,api_key_id,model_id,display_name,owned_by,raw_json,is_manual FROM site_models WHERE api_key_id=? ORDER BY model_id", keyID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.SiteModelDTO{}
	for rows.Next() {
		var m domain.SiteModelDTO
		var raw *string
		if e = rows.Scan(&m.ID, &m.SiteID, &m.APIKeyID, &m.ModelID, &m.DisplayName, &m.OwnedBy, &raw, &m.IsManual); e != nil {
			return nil, e
		}
		if raw != nil {
			if !json.Valid([]byte(*raw)) {
				return nil, domain.NewError("invalid_config", "model contains invalid raw JSON")
			}
			v := json.RawMessage(*raw)
			m.Raw = &v
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) ListModels(siteID string) ([]domain.SiteModelDTO, error) {
	k, e := readKey(s.db, siteID, "")
	if e != nil {
		return nil, e
	}
	return readModels(s.db, k.ID)
}
func (s *Store) ListModelsForKey(siteID, keyID string) ([]domain.SiteModelDTO, error) {
	k, e := readKey(s.db, siteID, keyID)
	if e != nil {
		return nil, e
	}
	return readModels(s.db, k.ID)
}
func (s *Store) SetSelectedModel(siteID, keyID, modelID string) error {
	return s.transaction(func(tx *sql.Tx) error {
		k, e := readKey(tx, siteID, keyID)
		if e != nil {
			return e
		}
		if !k.IsActive {
			return domain.NewError("validation_failed", "API key is no longer active")
		}
		return selectModel(tx, siteID, k.ID, modelID)
	})
}
func selectModel(q queryer, siteID, keyID, modelID string) error {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return domain.NewError("validation_failed", "model ID is required")
	}
	if _, e := q.Exec("DELETE FROM site_model_exclusions WHERE api_key_id=? AND model_id=?", keyID, modelID); e != nil {
		return e
	}
	if _, e := q.Exec(`INSERT INTO site_models(id,site_id,api_key_id,model_id,display_name,is_manual) VALUES(?,?,?,?,?,1) ON CONFLICT(api_key_id,model_id) DO NOTHING`, domain.NewID(), siteID, keyID, modelID, modelID); e != nil {
		return e
	}
	return requireChanged(q.Exec("UPDATE site_api_keys SET selected_model_id=?,updated_at=? WHERE id=?", modelID, now(), keyID))
}
func (s *Store) ReplaceModels(siteID, keyID string, models []domain.SiteModelDTO) error {
	return s.transaction(func(tx *sql.Tx) error {
		k, e := readKey(tx, siteID, keyID)
		if e != nil {
			return e
		}
		old, e := readModels(tx, k.ID)
		if e != nil {
			return e
		}
		excluded, e := readExclusions(tx, k.ID)
		if e != nil {
			return e
		}
		fetched := map[string]bool{}
		for _, m := range models {
			if strings.TrimSpace(m.ModelID) == "" {
				return domain.NewError("validation_failed", "model ID is required")
			}
			if fetched[m.ModelID] {
				return domain.NewError("validation_failed", "duplicate fetched model ID")
			}
			fetched[m.ModelID] = true
		}
		if _, e = tx.Exec("DELETE FROM site_models WHERE api_key_id=?", k.ID); e != nil {
			return e
		}
		for _, m := range models {
			if excluded[m.ModelID] {
				continue
			}
			if e = insertModel(tx, siteID, k.ID, m, false); e != nil {
				return e
			}
		}
		for _, m := range old {
			if m.IsManual && !fetched[m.ModelID] {
				if e = insertModel(tx, siteID, k.ID, m, true); e != nil {
					return e
				}
			}
		}
		return reconcileSelected(tx, k.ID)
	})
}
func insertModel(q queryer, siteID, keyID string, m domain.SiteModelDTO, manual bool) error {
	id := m.ID
	if id == "" {
		id = domain.NewID()
	}
	display := m.DisplayName
	if display == "" {
		display = m.ModelID
	}
	var raw *string
	if m.Raw != nil {
		if !json.Valid(*m.Raw) {
			return domain.NewError("validation_failed", "invalid model raw JSON")
		}
		s := string(*m.Raw)
		raw = &s
	}
	_, e := q.Exec("INSERT INTO site_models(id,site_id,api_key_id,model_id,display_name,owned_by,raw_json,is_manual) VALUES(?,?,?,?,?,?,?,?)", id, siteID, keyID, m.ModelID, display, m.OwnedBy, raw, manual)
	return e
}
func readExclusions(q queryer, keyID string) (map[string]bool, error) {
	rows, e := q.Query("SELECT model_id FROM site_model_exclusions WHERE api_key_id=?", keyID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		out[id] = true
	}
	return out, rows.Err()
}
func reconcileSelected(q queryer, keyID string) error {
	models, e := readModels(q, keyID)
	if e != nil {
		return e
	}
	var selected *string
	if e = q.QueryRow("SELECT selected_model_id FROM site_api_keys WHERE id=?", keyID).Scan(&selected); e != nil {
		return e
	}
	for _, m := range models {
		if selected != nil && m.ModelID == *selected {
			return nil
		}
	}
	selected = nil
	if len(models) > 0 {
		selected = &models[0].ModelID
	}
	return requireChanged(q.Exec("UPDATE site_api_keys SET selected_model_id=?,updated_at=? WHERE id=?", selected, now(), keyID))
}
func (s *Store) DeleteModel(siteID, modelID string) error {
	return s.transaction(func(tx *sql.Tx) error {
		k, e := readKey(tx, siteID, "")
		if e != nil {
			return e
		}
		if e = requireChanged(tx.Exec("DELETE FROM site_models WHERE api_key_id=? AND model_id=?", k.ID, modelID)); e != nil {
			return e
		}
		if _, e = tx.Exec("INSERT INTO site_model_exclusions(site_id,api_key_id,model_id) VALUES(?,?,?) ON CONFLICT DO NOTHING", siteID, k.ID, modelID); e != nil {
			return e
		}
		return reconcileSelected(tx, k.ID)
	})
}
func (s *Store) ClearModels(siteID string) error {
	return s.transaction(func(tx *sql.Tx) error {
		k, e := readKey(tx, siteID, "")
		if e != nil {
			return e
		}
		if _, e = tx.Exec("DELETE FROM site_models WHERE api_key_id=?", k.ID); e != nil {
			return e
		}
		_, e = tx.Exec("UPDATE site_api_keys SET selected_model_id=NULL,updated_at=? WHERE id=?", now(), k.ID)
		return e
	})
}
func (s *Store) UpdateModelFetch(siteID, keyID string, at, latency *int64, fetchError *string) error {
	k, e := readKey(s.db, siteID, keyID)
	if e != nil {
		return e
	}
	return requireChanged(s.db.Exec("UPDATE site_api_keys SET last_model_fetch_at=?,last_model_fetch_latency_ms=?,last_model_fetch_error=?,updated_at=? WHERE id=?", at, latency, fetchError, now(), k.ID))
}
