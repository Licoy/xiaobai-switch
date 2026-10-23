package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/envinject"
)

func (s *Service) GetSettings(ctx context.Context) (domain.AppSettings, error) {
	if e := checkContext(ctx); e != nil {
		return domain.AppSettings{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.GetSettings()
}
func (s *Service) SaveSettings(ctx context.Context, v domain.AppSettings) error {
	if e := checkContext(ctx); e != nil {
		return e
	}
	s.mu.Lock()
	for _, target := range targets {
		if _, e := s.targetHome(target, v); e != nil {
			s.mu.Unlock()
			return e
		}
	}
	if _, e := envinject.EffectiveMode(v.CodexEnvInjectMode, s.options.Platform); e != nil {
		s.mu.Unlock()
		return e
	}
	e := s.st.SaveSettings(v)
	s.mu.Unlock()
	if e == nil {
		s.changed()
	}
	return e
}
func (s *Service) MergeSettings(ctx context.Context, partial map[string]json.RawMessage) (domain.AppSettings, error) {
	if e := checkContext(ctx); e != nil {
		return domain.AppSettings{}, e
	}
	s.mu.Lock()
	notify := false
	defer func() {
		s.mu.Unlock()
		if notify {
			s.changed()
		}
	}()
	old, e := s.st.GetSettings()
	if e != nil {
		return old, e
	}
	raw, e := json.Marshal(old)
	if e != nil {
		return old, e
	}
	var values map[string]json.RawMessage
	if e = json.Unmarshal(raw, &values); e != nil {
		return old, e
	}
	for k, v := range partial {
		values[k] = v
	}
	raw, e = json.Marshal(values)
	if e != nil {
		return old, e
	}
	var next domain.AppSettings
	if e = json.Unmarshal(raw, &next); e != nil {
		return old, e
	}
	for _, target := range targets {
		if _, e = s.targetHome(target, next); e != nil {
			return old, e
		}
	}
	if _, e = envinject.EffectiveMode(next.CodexEnvInjectMode, s.options.Platform); e != nil {
		return old, e
	}
	if e = s.st.SaveSettings(next); e != nil {
		return old, e
	}
	notify = true
	return next, nil
}
func (s *Service) SaveThinkingPreset(ctx context.Context, p domain.SiteThinkingPreset) error {
	if e := checkContext(ctx); e != nil {
		return e
	}
	s.mu.Lock()
	notify := false
	defer func() {
		s.mu.Unlock()
		if notify {
			s.changed()
		}
	}()
	site, e := s.st.GetSite(p.SiteID)
	if e != nil {
		return e
	}
	if e = domain.ValidateThinking(p.ToWrite(), p.Target, domain.SiteProtocol(site.Protocol), ""); e != nil {
		return e
	}
	held, e := s.siteLock(p.SiteID)
	if e != nil {
		return e
	}
	if e = s.st.SaveThinkingPreset(p); e != nil {
		return errors.Join(e, held.Close())
	}
	if e = held.Close(); e != nil {
		return e
	}
	notify = true
	return nil
}
func (s *Service) SetSelectedModel(ctx context.Context, siteID, keyID, modelID string) error {
	if e := checkContext(ctx); e != nil {
		return e
	}
	s.mu.Lock()
	held, e := s.siteLock(siteID)
	if e != nil {
		s.mu.Unlock()
		return e
	}
	e = s.st.SetSelectedModel(siteID, keyID, modelID)
	e = errors.Join(e, held.Close())
	s.mu.Unlock()
	if e == nil {
		s.changed()
	}
	return e
}
func (s *Service) ListModels(ctx context.Context, siteID, keyID string) ([]domain.SiteModelDTO, error) {
	if e := checkContext(ctx); e != nil {
		return nil, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.ListModelsForKey(siteID, keyID)
}
