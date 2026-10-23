package service

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
)

func (s *Service) siteLock(id string) (*fileops.Locks, error) {
	dir, e := s.path("locks")
	if e != nil {
		return nil, e
	}
	return fileops.Acquire(dir, []string{"site:" + id}, nil)
}
func (s *Service) ListSites(ctx context.Context) ([]domain.SiteDTO, error) {
	if e := checkContext(ctx); e != nil {
		return nil, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.ListSites()
}
func (s *Service) CreateSite(ctx context.Context, in domain.CreateSiteInput) (domain.SiteDTO, error) {
	if e := checkContext(ctx); e != nil {
		return domain.SiteDTO{}, e
	}
	s.mu.Lock()
	site, e := s.st.CreateSite(in)
	s.mu.Unlock()
	if e == nil {
		s.changed()
	}
	return site, publicError(e, in.APIKey)
}
func (s *Service) UpdateSite(ctx context.Context, id string, in domain.UpdateSiteInput) (domain.SiteDTO, error) {
	if e := checkContext(ctx); e != nil {
		return domain.SiteDTO{}, e
	}
	s.mu.Lock()
	held, e := s.siteLock(id)
	if e != nil {
		s.mu.Unlock()
		return domain.SiteDTO{}, e
	}
	changedKeys, e := s.changedInputKeys(id, in)
	if e != nil {
		closeErr := held.Close()
		s.mu.Unlock()
		return domain.SiteDTO{}, errors.Join(e, closeErr)
	}
	site, e := s.st.UpdateSite(id, in)
	e = errors.Join(e, held.Close())
	s.mu.Unlock()
	if e != nil {
		return site, publicError(e)
	}
	s.changed()
	for keyID := range changedKeys {
		if _, e = s.FetchModels(ctx, id, keyID); e != nil {
			return site, e
		}
	}
	results := []domain.ApplyTargetResult{}
	if in.Protocol != nil || in.ClaudeAuthKeyStyle != nil || in.Capabilities != nil {
		rows, e := s.syncConnections(ctx, site, connectionChange{Input: in, ChangedKeys: changedKeys})
		if e != nil {
			return site, e
		}
		results = append(results, rows...)
	} else {
		for keyID := range changedKeys {
			rows, e := s.syncBound(ctx, id, keyID)
			if e != nil {
				return site, e
			}
			results = append(results, rows...)
		}
		if in.BaseURL != nil || in.BaseURLs != nil {
			rows, e := s.syncURL(ctx, site)
			if e != nil {
				return site, e
			}
			results = append(results, rows...)
		}
	}
	if e = partialError(results); e != nil {
		return site, e
	}
	return site, nil
}
func (s *Service) AddSiteAPIKey(ctx context.Context, siteID string, in domain.AddSiteApiKeyInput) (domain.SiteDTO, error) {
	if e := checkContext(ctx); e != nil {
		return domain.SiteDTO{}, e
	}
	s.mu.Lock()
	held, e := s.siteLock(siteID)
	if e != nil {
		s.mu.Unlock()
		return domain.SiteDTO{}, e
	}
	site, e := s.st.AddSiteAPIKey(siteID, in)
	e = errors.Join(e, held.Close())
	s.mu.Unlock()
	if e == nil {
		s.changed()
	}
	return site, publicError(e, in.APIKey)
}
func (s *Service) UpdateSiteAPIKey(ctx context.Context, siteID, keyID string, in domain.UpdateSiteApiKeyInput) (domain.SiteDTO, error) {
	if e := checkContext(ctx); e != nil {
		return domain.SiteDTO{}, e
	}
	s.mu.Lock()
	held, e := s.siteLock(siteID)
	if e != nil {
		s.mu.Unlock()
		return domain.SiteDTO{}, e
	}
	site, e := s.st.UpdateSiteAPIKey(siteID, keyID, in)
	e = errors.Join(e, held.Close())
	s.mu.Unlock()
	if e != nil {
		return site, publicError(e)
	}
	s.changed()
	if in.APIKey != nil {
		if _, e = s.FetchModels(ctx, siteID, keyID); e != nil {
			return site, e
		}
		results, e := s.syncBound(ctx, siteID, keyID)
		if e != nil {
			return site, e
		}
		return site, partialError(results)
	}
	return site, nil
}
func (s *Service) DeleteSiteAPIKey(ctx context.Context, siteID, keyID string) (err error) {
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
	if e = s.st.DeleteSiteAPIKey(siteID, keyID); e != nil {
		return publicError(e)
	}
	notify = true
	return nil
}
func (s *Service) GetSiteAPIKey(ctx context.Context, siteID, keyID string) (string, error) {
	if e := checkContext(ctx); e != nil {
		return "", e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.GetSiteAPIKey(siteID, keyID)
}
