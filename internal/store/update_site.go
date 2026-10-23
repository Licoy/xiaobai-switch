package store

import (
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"strings"
)

func (s *Store) UpdateSite(id string, in domain.UpdateSiteInput) (domain.SiteDTO, error) {
	e := s.transaction(func(tx *sql.Tx) error { return s.updateSite(tx, id, in) })
	if e != nil {
		return domain.SiteDTO{}, e
	}
	return s.GetSite(id)
}
func (s *Store) updateSite(tx *sql.Tx, id string, in domain.UpdateSiteInput) error {
	v, e := readSite(tx, id)
	if e != nil {
		return e
	}
	if in.Name != nil {
		v.Name = strings.TrimSpace(*in.Name)
		if v.Name == "" {
			return domain.NewError("validation_failed", "site name is required")
		}
	}
	if in.BaseURLs != nil {
		v.BaseURLs, e = normalizeURLs(*in.BaseURLs)
	} else if in.BaseURL != nil {
		var next []string
		next, e = normalizeURLs([]string{*in.BaseURL})
		if e == nil {
			v.BaseURLs = replaceFront(v.BaseURLs, next[0])
		}
	}
	if e != nil {
		return e
	}
	v.BaseURL = v.BaseURLs[0]
	if in.APIKeys != nil {
		if e = s.syncKeys(tx, id, *in.APIKeys); e != nil {
			return e
		}
	} else if in.APIKey != nil && *in.APIKey != "" {
		if e = s.updateKey(tx, id, *v.ActiveAPIKeyID, domain.UpdateSiteApiKeyInput{APIKey: in.APIKey}); e != nil {
			return e
		}
	}
	if in.Protocol != nil {
		v.Protocol = *in.Protocol
	}
	if in.ClaudeAuthKeyStyle != nil {
		v.ClaudeAuthKeyStyle = *in.ClaudeAuthKeyStyle
	}
	if _, _, e = siteEnums(&v.Protocol, &v.ClaudeAuthKeyStyle); e != nil {
		return e
	}
	if in.Notes != nil {
		v.Notes = in.Notes
	}
	if in.Enabled != nil {
		v.Enabled = *in.Enabled
	}
	if in.SortOrder != nil {
		v.SortOrder = *in.SortOrder
	}
	if in.Capabilities != nil {
		v.Capabilities = *in.Capabilities
	}
	if in.SelectedModelID != nil {
		k, e := readKey(tx, id, "")
		if e != nil {
			return e
		}
		if e = selectModel(tx, id, k.ID, *in.SelectedModelID); e != nil {
			return e
		}
	}
	urls, e := json.Marshal(v.BaseURLs)
	if e != nil {
		return e
	}
	caps, e := json.Marshal(v.Capabilities)
	if e != nil {
		return e
	}
	return requireChanged(tx.Exec(`UPDATE sites SET name=?,base_url=?,protocol=?,claude_auth_key_style=?,notes=?,enabled=?,sort_order=?,updated_at=?,base_urls_json=?,capabilities_json=? WHERE id=?`, v.Name, v.BaseURL, v.Protocol, v.ClaudeAuthKeyStyle, v.Notes, v.Enabled, v.SortOrder, now(), string(urls), string(caps), id))
}
func replaceFront(urls []string, selected string) []string {
	out := []string{selected}
	found := false
	for _, u := range urls {
		found = found || u == selected
	}
	for i, u := range urls {
		if u != selected && (found || i > 0) {
			out = append(out, u)
		}
	}
	return out
}
func (s *Store) SwitchSiteRoute(id, route string) (domain.SiteDTO, error) {
	e := s.transaction(func(tx *sql.Tx) error {
		v, e := readSite(tx, id)
		if e != nil {
			return e
		}
		normalized, e := normalizeURLs([]string{route})
		if e != nil {
			return e
		}
		found := false
		for _, u := range v.BaseURLs {
			found = found || u == normalized[0]
		}
		if !found {
			return domain.NewError("validation_failed", "route does not belong to site")
		}
		routes, e := json.Marshal(replaceFront(v.BaseURLs, normalized[0]))
		if e != nil {
			return e
		}
		return requireChanged(tx.Exec("UPDATE sites SET base_url=?,base_urls_json=?,updated_at=? WHERE id=?", normalized[0], string(routes), now(), id))
	})
	if e != nil {
		return domain.SiteDTO{}, e
	}
	return s.GetSite(id)
}
func (s *Store) syncKeys(tx *sql.Tx, siteID string, items []domain.UpsertSiteApiKeyInput) error {
	if len(items) == 0 {
		return domain.NewError("validation_failed", "at least one API key is required")
	}
	existing, e := readKeys(tx, siteID)
	if e != nil {
		return e
	}
	existingIDs := map[string]bool{}
	for _, k := range existing {
		existingIDs[k.ID] = true
	}
	keep := map[string]bool{}
	assigned := make([]string, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.APIKey) == "" {
			return domain.NewError("validation_failed", "API key is required")
		}
		if item.ID != nil {
			if !existingIDs[*item.ID] || keep[*item.ID] {
				return domain.NewError("validation_failed", "invalid or duplicate API key ID")
			}
			assigned[i] = *item.ID
			keep[*item.ID] = true
		}
	}
	// Keep a valid active key while removing obsolete entries
	active := ""
	for _, id := range assigned {
		if id != "" {
			active = id
			break
		}
	}
	if active == "" {
		id, e := s.insertKey(tx, keyInsert{siteID: siteID, label: items[0].Label, secret: items[0].APIKey})
		if e != nil {
			return e
		}
		active = id
		assigned[0] = id
		keep[id] = true
	}
	if _, e = tx.Exec("UPDATE site_api_keys SET is_active=0 WHERE site_id=?", siteID); e != nil {
		return e
	}
	if _, e = tx.Exec("UPDATE site_api_keys SET is_active=1 WHERE id=?", active); e != nil {
		return e
	}
	for _, k := range existing {
		if !keep[k.ID] {
			if e = deleteKey(tx, siteID, k.ID); e != nil {
				return e
			}
		}
	}
	for i, item := range items {
		if assigned[i] == "" {
			assigned[i], e = s.insertKey(tx, keyInsert{siteID: siteID, label: item.Label, secret: item.APIKey})
		} else {
			e = s.updateKey(tx, siteID, assigned[i], domain.UpdateSiteApiKeyInput{Label: item.Label, APIKey: &item.APIKey})
		}
		if e != nil {
			return e
		}
	}
	if _, e = tx.Exec("UPDATE site_api_keys SET is_active=0 WHERE site_id=?", siteID); e != nil {
		return e
	}
	_, e = tx.Exec("UPDATE site_api_keys SET is_active=1 WHERE id=?", assigned[0])
	return e
}
