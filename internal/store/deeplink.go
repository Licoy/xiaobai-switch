package store

import (
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"sort"
	"strings"
)

func sameRoutes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a = append([]string(nil), a...)
	b = append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func mergeImportCapabilities(old, in domain.SiteCapabilities) domain.SiteCapabilities {
	out := domain.SiteCapabilities{}
	for k, v := range old {
		out[k] = v
	}
	for k, v := range in {
		out[k] = v
	}
	for _, k := range []string{"codex-compact", "codex-vision", "codex-imagegen", "codex-search"} {
		out[k] = in[k]
	}
	return out
}

// ImportDeepLink keeps metadata updates and key insertion in one transaction
func (s *Store) ImportDeepLink(in domain.DeepLinkSiteImportInput) (domain.DeepLinkSiteImportResult, error) {
	result := domain.DeepLinkSiteImportResult{}
	id := ""
	e := s.transaction(func(tx *sql.Tx) error {
		rows, e := tx.Query("SELECT id FROM sites ORDER BY sort_order,created_at")
		if e != nil {
			return e
		}
		ids := []string{}
		for rows.Next() {
			var v string
			if e = rows.Scan(&v); e != nil {
				rows.Close()
				return e
			}
			ids = append(ids, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		protocol := "openai_compatible"
		if in.Protocol != nil {
			protocol = *in.Protocol
		}
		for _, candidate := range ids {
			v, e := readSite(tx, candidate)
			if e != nil {
				return e
			}
			if v.Protocol != protocol || !sameRoutes(v.BaseURLs, in.BaseURLs) {
				continue
			}
			id = v.ID
			keys, e := readKeys(tx, id)
			if e != nil {
				return e
			}
			for _, key := range keys {
				plain, e := s.crypto.Decrypt(key.APIKeyEncrypted)
				if e != nil {
					return e
				}
				if plain == in.APIKey {
					result.ReusedAPIKey = true
					break
				}
			}
			update := domain.UpdateSiteInput{Name: &in.Name, Notes: in.Notes}
			if in.Capabilities != nil {
				caps := mergeImportCapabilities(v.Capabilities, *in.Capabilities)
				update.Capabilities = &caps
			}
			if e = s.updateSite(tx, id, update); e != nil {
				return e
			}
			if !result.ReusedAPIKey {
				if _, e = s.insertKey(tx, keyInsert{siteID: id, label: in.KeyName, secret: in.APIKey}); e != nil {
					return e
				}
				result.AddedAPIKey = true
			}
			return nil
		}
		if strings.TrimSpace(in.Name) == "" || len(in.BaseURLs) == 0 {
			return domain.NewError("validation_failed", "site name and routes are required")
		}
		urls, e := normalizeURLs(in.BaseURLs)
		if e != nil {
			return e
		}
		caps := domain.SiteCapabilities{}
		if in.Capabilities != nil {
			caps = mergeImportCapabilities(caps, *in.Capabilities)
		}
		uj, e := json.Marshal(urls)
		if e != nil {
			return e
		}
		cj, e := json.Marshal(caps)
		if e != nil {
			return e
		}
		id = domain.NewID()
		ts := now()
		if _, e = tx.Exec(`INSERT INTO sites(id,name,base_url,protocol,claude_auth_key_style,notes,enabled,sort_order,created_at,updated_at,base_urls_json,capabilities_json) VALUES(?,?,?,?,'anthropic_auth_token',?,1,?,?,?,?,?)`, id, in.Name, urls[0], protocol, in.Notes, len(ids), ts, ts, string(uj), string(cj)); e != nil {
			return e
		}
		if _, e = s.insertKey(tx, keyInsert{siteID: id, label: in.KeyName, secret: in.APIKey, active: true}); e != nil {
			return e
		}
		result.Created = true
		result.AddedAPIKey = true
		result.ActivatedAPIKey = true
		return nil
	})
	if e != nil {
		return result, e
	}
	result.Site, e = s.GetSite(id)
	return result, e
}
