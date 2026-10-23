package store

import (
	"database/sql"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/secure"
	"strings"
)

const keySelect = `SELECT id,site_id,label,api_key_encrypted,key_prefix,is_active,selected_model_id,last_model_fetch_at,last_model_fetch_latency_ms,last_model_fetch_error,created_at,updated_at FROM site_api_keys `

type scanner interface{ Scan(...any) error }

func scanKey(r scanner) (domain.SiteApiKeyRow, error) {
	var k domain.SiteApiKeyRow
	e := r.Scan(&k.ID, &k.SiteID, &k.Label, &k.APIKeyEncrypted, &k.KeyPrefix, &k.IsActive, &k.SelectedModelID, &k.LastModelFetchAt, &k.LastModelFetchLatencyMS, &k.LastModelFetchError, &k.CreatedAt, &k.UpdatedAt)
	return k, e
}
func readKey(q queryer, siteID, keyID string) (domain.SiteApiKeyRow, error) {
	where := "WHERE site_id=? AND id=?"
	args := []any{siteID, keyID}
	if keyID == "" {
		where = "WHERE site_id=? AND is_active=1"
		args = []any{siteID}
	}
	k, e := scanKey(q.QueryRow(keySelect+where, args...))
	if e == sql.ErrNoRows {
		return k, domain.NewError("not_found", "API key not found")
	}
	return k, e
}
func readKeys(q queryer, siteID string) ([]domain.SiteApiKeyRow, error) {
	rows, e := q.Query(keySelect+"WHERE site_id=? ORDER BY created_at,id", siteID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.SiteApiKeyRow{}
	for rows.Next() {
		k, e := scanKey(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

type keyInsert struct {
	siteID string
	label  *string
	secret string
	active bool
}

func (s *Store) insertKey(q queryer, in keyInsert) (string, error) {
	secret := strings.TrimSpace(in.secret)
	if secret == "" {
		return "", domain.NewError("validation_failed", "API key is required")
	}
	keys, e := readKeys(q, in.siteID)
	if e != nil {
		return "", e
	}
	label := ""
	if in.label != nil {
		label = strings.TrimSpace(*in.label)
		if label == "" {
			return "", domain.NewError("validation_failed", "API key label is required")
		}
	}
	if label == "" {
		for i := 1; ; i++ {
			label = fmt.Sprintf("K %d", i)
			taken := false
			for _, k := range keys {
				taken = taken || strings.EqualFold(k.Label, label)
			}
			if !taken {
				break
			}
		}
	}
	ts := now()
	for _, k := range keys {
		if strings.EqualFold(k.Label, label) {
			return "", domain.NewError("validation_failed", "key name already exists on this site")
		}
		plain, e := s.crypto.Decrypt(k.APIKeyEncrypted)
		if e != nil {
			return "", e
		}
		if plain == secret {
			return "", domain.NewError("validation_failed", "API key already exists on this site")
		}
		ts = max(ts, k.CreatedAt+1)
	}
	enc, e := s.crypto.Encrypt(secret)
	if e != nil {
		return "", e
	}
	id := domain.NewID()
	_, e = q.Exec(`INSERT INTO site_api_keys(id,site_id,label,api_key_encrypted,key_prefix,is_active,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, id, in.siteID, label, enc, secure.KeyPrefix(secret), in.active, ts, ts)
	return id, e
}
func (s *Store) AddSiteAPIKey(siteID string, in domain.AddSiteApiKeyInput) (domain.SiteDTO, error) {
	e := s.transaction(func(tx *sql.Tx) error {
		if _, e := readSite(tx, siteID); e != nil {
			return e
		}
		_, e := s.insertKey(tx, keyInsert{siteID: siteID, label: in.Label, secret: in.APIKey})
		return e
	})
	if e != nil {
		return domain.SiteDTO{}, e
	}
	return s.GetSite(siteID)
}
func (s *Store) ActivateSiteAPIKey(siteID, keyID string) (domain.SiteDTO, error) {
	e := s.transaction(func(tx *sql.Tx) error {
		if _, e := readKey(tx, siteID, keyID); e != nil {
			return e
		}
		if _, e := tx.Exec("UPDATE site_api_keys SET is_active=0,updated_at=? WHERE site_id=?", now(), siteID); e != nil {
			return e
		}
		return requireChanged(tx.Exec("UPDATE site_api_keys SET is_active=1,updated_at=? WHERE site_id=? AND id=?", now(), siteID, keyID))
	})
	if e != nil {
		return domain.SiteDTO{}, e
	}
	return s.GetSite(siteID)
}
func (s *Store) DeleteSiteAPIKey(siteID, keyID string) error {
	return s.transaction(func(tx *sql.Tx) error { return deleteKey(tx, siteID, keyID) })
}
func deleteKey(q queryer, siteID, keyID string) error {
	k, e := readKey(q, siteID, keyID)
	if e != nil {
		return e
	}
	if k.IsActive {
		return domain.NewError("validation_failed", "switch keys before deleting the active key")
	}
	var live int
	if e = q.QueryRow("SELECT count(*) FROM target_bindings WHERE site_api_key_id=? AND orphan=0", keyID).Scan(&live); e != nil {
		return e
	}
	if live > 0 {
		return domain.NewError("validation_failed", "key is still applied to a target")
	}
	return requireChanged(q.Exec("DELETE FROM site_api_keys WHERE site_id=? AND id=?", siteID, keyID))
}
func (s *Store) UpdateSiteAPIKey(siteID, keyID string, in domain.UpdateSiteApiKeyInput) (domain.SiteDTO, error) {
	e := s.transaction(func(tx *sql.Tx) error { return s.updateKey(tx, siteID, keyID, in) })
	if e != nil {
		return domain.SiteDTO{}, e
	}
	return s.GetSite(siteID)
}
func (s *Store) updateKey(q queryer, siteID, keyID string, in domain.UpdateSiteApiKeyInput) error {
	k, e := readKey(q, siteID, keyID)
	if e != nil {
		return e
	}
	if in.Label != nil {
		k.Label = strings.TrimSpace(*in.Label)
		if k.Label == "" {
			return domain.NewError("validation_failed", "API key label is required")
		}
	}
	keys, e := readKeys(q, siteID)
	if e != nil {
		return e
	}
	for _, other := range keys {
		if other.ID == keyID {
			continue
		}
		if strings.EqualFold(other.Label, k.Label) {
			return domain.NewError("validation_failed", "key name already exists on this site")
		}
		if in.APIKey != nil {
			plain, e := s.crypto.Decrypt(other.APIKeyEncrypted)
			if e != nil {
				return e
			}
			if plain == strings.TrimSpace(*in.APIKey) {
				return domain.NewError("validation_failed", "API key already exists on this site")
			}
		}
	}
	if in.APIKey != nil {
		plain := strings.TrimSpace(*in.APIKey)
		if plain == "" {
			return domain.NewError("validation_failed", "API key is required")
		}
		k.APIKeyEncrypted, e = s.crypto.Encrypt(plain)
		if e != nil {
			return e
		}
		k.KeyPrefix = secure.KeyPrefix(plain)
	}
	return requireChanged(q.Exec("UPDATE site_api_keys SET label=?,api_key_encrypted=?,key_prefix=?,updated_at=? WHERE site_id=? AND id=?", k.Label, k.APIKeyEncrypted, k.KeyPrefix, now(), siteID, keyID))
}
