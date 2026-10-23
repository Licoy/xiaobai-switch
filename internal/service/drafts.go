package service

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
)

func (s *Service) SaveTargetPreset(ctx context.Context, p domain.TargetPreset) (err error) {
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
	held, e := s.siteLock(p.SiteID)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	site, e := s.st.GetSite(p.SiteID)
	if e != nil {
		return e
	}
	if e = adapters.ValidateOptions(adapters.Request{Target: p.Target, ModelID: p.Options.ModelID, Site: domain.SiteRow{Protocol: domain.SiteProtocol(site.Protocol)}, Claude: p.Options.Claude, Codex: p.Options.Codex, Pi: p.Options.Pi, Prime: p.Options.Prime}); e != nil {
		return e
	}
	if e = s.st.SaveTargetPreset(p); e != nil {
		return e
	}
	notify = true
	return nil
}
func (s *Service) DeleteModel(ctx context.Context, siteID, modelID string) (err error) {
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
	held, e := s.siteLock(siteID)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	if e = s.st.DeleteModel(siteID, modelID); e != nil {
		return e
	}
	notify = true
	return nil
}
func (s *Service) ClearModels(ctx context.Context, siteID string) (err error) {
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
	held, e := s.siteLock(siteID)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	if e = s.st.ClearModels(siteID); e != nil {
		return e
	}
	notify = true
	return nil
}
func (s *Service) ReorderSites(ctx context.Context, ids []string) (err error) {
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
	root, e := s.path("locks")
	if e != nil {
		return e
	}
	keys := []string{}
	for _, id := range ids {
		keys = append(keys, "site:"+id)
	}
	held, e := fileops.Acquire(root, keys, nil)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	if e = s.st.ReorderSites(ids); e != nil {
		return e
	}
	notify = true
	return nil
}
