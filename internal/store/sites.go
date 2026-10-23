package store

import (
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"github.com/licoy/xiaobai-switch/internal/secure"
	"strings"
)

type queryer interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
	Exec(string, ...any) (sql.Result, error)
}

const siteSelect = `SELECT s.id,s.name,s.base_url,s.protocol,s.claude_auth_key_style,s.notes,s.enabled,s.sort_order,s.created_at,s.updated_at,s.base_urls_json,s.capabilities_json FROM sites s `

func normalizeURLs(raw []string) ([]string, error) { return netx.NormalizeBaseURLs(raw) }

func readSite(q queryer, id string) (domain.SiteDTO, error) {
	var v domain.SiteDTO
	var urls, caps sql.NullString
	e := q.QueryRow(siteSelect+"WHERE s.id=?", id).Scan(&v.ID, &v.Name, &v.BaseURL, &v.Protocol, &v.ClaudeAuthKeyStyle, &v.Notes, &v.Enabled, &v.SortOrder, &v.CreatedAt, &v.UpdatedAt, &urls, &caps)
	if e == sql.ErrNoRows {
		return v, domain.NewError("not_found", "site not found")
	}
	if e != nil {
		return v, e
	}
	v.BaseURLs = []string{v.BaseURL}
	if urls.Valid && urls.String != "" {
		if e = json.Unmarshal([]byte(urls.String), &v.BaseURLs); e != nil {
			return v, e
		}
		if len(v.BaseURLs) == 0 {
			return v, domain.NewError("invalid_config", "empty stored site routes")
		}
		v.BaseURL = v.BaseURLs[0]
	}
	v.Capabilities = domain.SiteCapabilities{}
	if caps.Valid && caps.String != "" {
		if e = json.Unmarshal([]byte(caps.String), &v.Capabilities); e != nil {
			return v, e
		}
	}
	keys, e := readKeys(q, id)
	if e != nil {
		return v, e
	}
	v.APIKeys = []domain.SiteApiKeySummary{}
	for _, k := range keys {
		summary := keySummary(k)
		v.APIKeys = append(v.APIKeys, summary)
		if k.IsActive {
			keyID := k.ID
			v.ActiveAPIKeyID = &keyID
			v.KeyPrefix = k.KeyPrefix
			v.QuotaRevision = summary.QuotaRevision
			v.HasKey = k.APIKeyEncrypted != ""
			v.SelectedModelID = k.SelectedModelID
			v.LastModelFetchAt = k.LastModelFetchAt
			v.LastModelFetchLatencyMS = k.LastModelFetchLatencyMS
			v.LastModelFetchError = k.LastModelFetchError
		}
	}
	if v.ActiveAPIKeyID == nil {
		return v, domain.NewError("invalid_config", "site has no active API key")
	}
	return v, nil
}
func (s *Store) GetSite(id string) (domain.SiteDTO, error) { return readSite(s.db, id) }
func (s *Store) ListSites() ([]domain.SiteDTO, error) {
	rows, e := s.db.Query("SELECT id FROM sites ORDER BY sort_order,created_at")
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []domain.SiteDTO{}
	for _, id := range ids {
		v, e := s.GetSite(id)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (s *Store) CreateSite(in domain.CreateSiteInput) (domain.SiteDTO, error) {
	if strings.TrimSpace(in.Name) == "" {
		return domain.SiteDTO{}, domain.NewError("validation_failed", "site name is required")
	}
	raw := []string{in.BaseURL}
	if in.BaseURLs != nil {
		raw = *in.BaseURLs
	}
	urls, e := normalizeURLs(raw)
	if e != nil {
		return domain.SiteDTO{}, e
	}
	protocol, auth, e := siteEnums(in.Protocol, in.ClaudeAuthKeyStyle)
	if e != nil {
		return domain.SiteDTO{}, e
	}
	caps := domain.SiteCapabilities{}
	if in.Capabilities != nil {
		caps = *in.Capabilities
	}
	uj, e := json.Marshal(urls)
	if e != nil {
		return domain.SiteDTO{}, e
	}
	cj, e := json.Marshal(caps)
	if e != nil {
		return domain.SiteDTO{}, e
	}
	id := domain.NewID()
	ts := now()
	e = s.transaction(func(tx *sql.Tx) error {
		var n int
		if e := tx.QueryRow("SELECT count(*) FROM sites").Scan(&n); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO sites(id,name,base_url,protocol,claude_auth_key_style,notes,enabled,sort_order,created_at,updated_at,base_urls_json,capabilities_json) VALUES(?,?,?,?,?,?,1,?,?,?,?,?)`, id, strings.TrimSpace(in.Name), urls[0], protocol, auth, in.Notes, n, ts, ts, string(uj), string(cj)); e != nil {
			return e
		}
		if _, e := s.insertKey(tx, keyInsert{siteID: id, label: in.APIKeyLabel, secret: in.APIKey, active: true}); e != nil {
			return e
		}
		for _, extra := range in.ExtraAPIKeys {
			if _, e := s.insertKey(tx, keyInsert{siteID: id, label: extra.Label, secret: extra.APIKey}); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		return domain.SiteDTO{}, e
	}
	return s.GetSite(id)
}
func siteEnums(p, a *string) (string, string, error) {
	protocol := "openai_compatible"
	auth := "anthropic_auth_token"
	if p != nil {
		protocol = *p
	}
	if a != nil {
		auth = *a
	}
	if protocol != "openai_compatible" && protocol != "anthropic" {
		return "", "", domain.NewError("validation_failed", "unknown site protocol")
	}
	if auth != "anthropic_auth_token" && auth != "anthropic_api_key" {
		return "", "", domain.NewError("validation_failed", "unknown Claude auth style")
	}
	return protocol, auth, nil
}
func (s *Store) DeleteSite(id string) error {
	return s.transaction(func(tx *sql.Tx) error {
		if _, e := tx.Exec("UPDATE target_bindings SET site_id=NULL,orphan=1 WHERE site_id=?", id); e != nil {
			return e
		}
		return requireChanged(tx.Exec("DELETE FROM sites WHERE id=?", id))
	})
}
func (s *Store) GetSiteAPIKey(siteID, keyID string) (string, error) {
	key, e := readKey(s.db, siteID, keyID)
	if e != nil {
		return "", e
	}
	return s.crypto.Decrypt(key.APIKeyEncrypted)
}
func keySummary(k domain.SiteApiKeyRow) domain.SiteApiKeySummary {
	return domain.SiteApiKeySummary{ID: k.ID, Label: k.Label, KeyPrefix: k.KeyPrefix, IsActive: k.IsActive, QuotaRevision: secure.KeyFingerprint(k.APIKeyEncrypted), SelectedModelID: k.SelectedModelID, LastModelFetchAt: k.LastModelFetchAt, LastModelFetchLatencyMS: k.LastModelFetchLatencyMS, LastModelFetchError: k.LastModelFetchError}
}
