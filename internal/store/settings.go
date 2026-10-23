package store

import (
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func (s *Store) GetSettings() (domain.AppSettings, error) {
	var b string
	e := s.db.QueryRow("SELECT json FROM settings WHERE id=1").Scan(&b)
	if e == sql.ErrNoRows {
		return domain.DefaultSettings(), nil
	}
	if e != nil {
		return domain.AppSettings{}, e
	}
	var v domain.AppSettings
	if e = json.Unmarshal([]byte(b), &v); e != nil {
		return v, domain.NewError("invalid_config", "settings JSON is invalid")
	}
	return domain.NormalizeSettings(v), nil
}
func (s *Store) SaveSettings(v domain.AppSettings) error {
	v = domain.NormalizeSettings(v)
	return s.transaction(func(tx *sql.Tx) error {
		var old string
		e := tx.QueryRow("SELECT json FROM settings WHERE id=1").Scan(&old)
		if e != nil && e != sql.ErrNoRows {
			return e
		}
		extras := map[string]json.RawMessage{}
		if e == nil {
			var previous domain.AppSettings
			if e = json.Unmarshal([]byte(old), &previous); e != nil {
				return e
			}
			for k, value := range previous.Extra {
				extras[k] = value
			}
		}
		for k, value := range v.Extra {
			extras[k] = value
		}
		v.Extra = extras
		b, e := json.Marshal(v)
		if e != nil {
			return e
		}
		_, e = tx.Exec("INSERT INTO settings(id,json) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET json=excluded.json", string(b))
		return e
	})
}

func (s *Store) MergeSettings(partial map[string]json.RawMessage) (domain.AppSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.GetSettings()
	if e != nil {
		return v, e
	}
	b, e := json.Marshal(v)
	if e != nil {
		return v, e
	}
	var m map[string]json.RawMessage
	if e = json.Unmarshal(b, &m); e != nil {
		return v, e
	}
	for k, x := range partial {
		m[k] = x
	}
	b, e = json.Marshal(m)
	if e != nil {
		return v, e
	}
	if e = json.Unmarshal(b, &v); e != nil {
		return v, e
	}
	v = domain.NormalizeSettings(v)
	return v, s.SaveSettings(v)
}
