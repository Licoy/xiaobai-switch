package store

import (
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func (s *Store) GetThinkingPreset(siteID string, target domain.TargetKind) (domain.SiteThinkingPreset, error) {
	v := domain.SiteThinkingPreset{SiteID: siteID, Target: target, Models: map[string]domain.ModelThinkingConfig{}}
	if !domain.IsThinkingTarget(target) {
		return v, domain.NewError("validation_failed", "thinking target must be Pi or Prime")
	}
	if _, e := s.GetSite(siteID); e != nil {
		return v, e
	}
	var raw string
	e := s.db.QueryRow("SELECT json FROM site_thinking_presets WHERE site_id=? AND target=?", siteID, target).Scan(&raw)
	if e == sql.ErrNoRows {
		return v, nil
	}
	if e != nil {
		return v, e
	}
	if e = json.Unmarshal([]byte(raw), &v); e != nil {
		return v, e
	}
	v.SiteID = siteID
	v.Target = target
	return v, nil
}
func (s *Store) SaveThinkingPreset(p domain.SiteThinkingPreset) error {
	site, e := s.GetSite(p.SiteID)
	if e != nil {
		return e
	}
	if e = domain.ValidateThinking(p.ToWrite(), p.Target, domain.SiteProtocol(site.Protocol), ""); e != nil {
		return e
	}
	raw, e := json.Marshal(p)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO site_thinking_presets(site_id,target,json,updated_at) VALUES(?,?,?,?) ON CONFLICT(site_id,target) DO UPDATE SET json=excluded.json,updated_at=excluded.updated_at", p.SiteID, p.Target, string(raw), now())
	return e
}
func (s *Store) GetTargetPreset(siteID, keyID string, target domain.TargetKind) (*domain.TargetPreset, error) {
	if !target.Valid() {
		return nil, domain.NewError("validation_failed", "unknown target")
	}
	if _, e := readKey(s.db, siteID, keyID); e != nil {
		return nil, e
	}
	var raw string
	var ts int64
	e := s.db.QueryRow("SELECT json,updated_at FROM target_presets WHERE site_id=? AND api_key_id=? AND target=?", siteID, keyID, target).Scan(&raw, &ts)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var options domain.ApplyRequest
	if e = json.Unmarshal([]byte(raw), &options); e != nil {
		return nil, e
	}
	return &domain.TargetPreset{SiteID: siteID, APIKeyID: keyID, Target: target, Options: options, UpdatedAt: ts}, nil
}
func saveTargetPreset(q queryer, p domain.TargetPreset) error {
	if !p.Target.Valid() {
		return domain.NewError("validation_failed", "unknown target")
	}
	if _, e := readKey(q, p.SiteID, p.APIKeyID); e != nil {
		return e
	}
	p.Options.SiteID = p.SiteID
	p.Options.APIKeyID = p.APIKeyID
	p.Options.Target = p.Target
	raw, e := json.Marshal(p.Options)
	if e != nil {
		return e
	}
	_, e = q.Exec("INSERT INTO target_presets(site_id,api_key_id,target,json,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(site_id,api_key_id,target) DO UPDATE SET json=excluded.json,updated_at=excluded.updated_at", p.SiteID, p.APIKeyID, p.Target, string(raw), now())
	return e
}

// SaveTargetPreset records options only after a successful target application
func (s *Store) SaveTargetPreset(p domain.TargetPreset) error {
	return s.transaction(func(tx *sql.Tx) error { return saveTargetPreset(tx, p) })
}
