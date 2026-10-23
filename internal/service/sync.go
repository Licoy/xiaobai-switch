package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func partialError(results []domain.ApplyTargetResult) error {
	failed := false
	for _, r := range results {
		failed = failed || !r.Ok
	}
	if !failed {
		return nil
	}
	raw, _ := json.Marshal(results)
	details := string(raw)
	return &domain.AppError{Code: "partial_failure", Message: "one or more bound targets could not be synchronized", Details: &details}
}
func (s *Service) syncBound(ctx context.Context, siteID, keyFilter string) ([]domain.ApplyTargetResult, error) {
	return s.syncApplications(ctx, siteID, keyFilter, "")
}
func (s *Service) syncApplications(ctx context.Context, siteID, keyFilter, newKeyID string) ([]domain.ApplyTargetResult, error) {
	s.mu.Lock()
	bindings, e := s.st.ListBindings()
	site, siteErr := s.st.GetSite(siteID)
	s.mu.Unlock()
	if e != nil {
		return nil, e
	}
	if siteErr != nil {
		return nil, siteErr
	}
	if newKeyID != "" && (site.ActiveAPIKeyID == nil || *site.ActiveAPIKeyID != newKeyID) {
		return nil, domain.NewError("config_changed", "active key changed before target synchronization")
	}
	out := []domain.ApplyTargetResult{}
	for _, b := range bindings {
		if b.SiteID == nil || *b.SiteID != siteID || b.Orphan {
			continue
		}
		oldKey := ""
		if b.APIKey.ID != nil {
			oldKey = *b.APIKey.ID
		}
		if keyFilter != "" && oldKey != keyFilter {
			continue
		}
		keyID := oldKey
		if newKeyID != "" {
			keyID = newKeyID
		}
		if keyID == "" && site.ActiveAPIKeyID != nil {
			keyID = *site.ActiveAPIKeyID
		}
		s.mu.Lock()
		key, e := s.st.GetSiteAPIKey(siteID, keyID)
		s.mu.Unlock()
		row := domain.ApplyTargetResult{Target: b.Target, Status: domain.Failed}
		if e == nil {
			e = s.rewriteManaged(ctx, b, rewriteGuard{BaseURL: site.BaseURL, KeyID: keyID, Key: key, RequireActive: newKeyID != "", CheckModels: newKeyID != "" || keyFilter != ""})
		}
		if e != nil {
			row.Message = publicError(e, key).Error()
		} else {
			row.Ok = true
			row.Status, e = s.bindingStatus(b.Target)
			if e != nil {
				row.Ok = false
				row.Status = domain.Failed
				row.Message = publicError(e).Error()
			} else {
				row.Message = "credentials_updated"
			}
		}
		out = append(out, row)
	}
	return out, nil
}
func (s *Service) bindingStatus(target domain.TargetKind) (domain.ApplyStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := s.st.GetBinding(target)
	if e != nil {
		return domain.Stale, e
	}
	if b == nil {
		return domain.NotApplied, nil
	}
	state, e := adapters.ReadState(b)
	return state.Status, e
}
func (s *Service) ActivateSiteAPIKey(ctx context.Context, siteID, keyID string) (domain.SiteDTO, error) {
	result, e := s.SwitchKey(ctx, siteID, keyID)
	return result.Site, e
}
func (s *Service) SwitchKey(ctx context.Context, siteID, keyID string) (domain.SwitchSiteApiKeyResult, error) {
	out := domain.SwitchSiteApiKeyResult{}
	if e := checkContext(ctx); e != nil {
		return out, e
	}
	s.mu.Lock()
	held, e := s.siteLock(siteID)
	if e != nil {
		s.mu.Unlock()
		return out, e
	}
	out.Site, e = s.st.ActivateSiteAPIKey(siteID, keyID)
	e = errors.Join(e, held.Close())
	s.mu.Unlock()
	if e != nil {
		return out, publicError(e)
	}
	s.changed()
	f, e := s.FetchModels(ctx, siteID, keyID)
	out.Fetch = domain.ModelFetchOutcome{APIKeyID: keyID, Ok: e == nil}
	if e != nil {
		message := publicError(e).Error()
		out.Fetch.Error = &message
		return out, e
	}
	out.Models = f.Models
	out.Fetch.LatencyMS = f.LatencyMS
	out.Fetch.Endpoint = &f.Endpoint
	out.Fetch.FetchedAt = &f.FetchedAt
	out.Results, e = s.syncApplications(ctx, siteID, "", keyID)
	if e != nil {
		return out, e
	}
	out.Site, e = s.st.GetSite(siteID)
	if e != nil {
		return out, e
	}
	return out, partialError(out.Results)
}
func (s *Service) SwitchSiteRoute(ctx context.Context, siteID, route string) (domain.SiteDTO, error) {
	out, e := s.SwitchRoute(ctx, siteID, route)
	return out.Site, e
}
func (s *Service) SwitchRoute(ctx context.Context, siteID, route string) (domain.SwitchRouteResult, error) {
	return s.SwitchRouteWithOptions(ctx, siteID, route, true)
}
func (s *Service) SwitchRouteWithOptions(ctx context.Context, siteID, route string, apply bool) (domain.SwitchRouteResult, error) {
	out := domain.SwitchRouteResult{}
	if e := checkContext(ctx); e != nil {
		return out, e
	}
	s.mu.Lock()
	held, e := s.siteLock(siteID)
	if e != nil {
		s.mu.Unlock()
		return out, e
	}
	out.Site, e = s.st.SwitchSiteRoute(siteID, route)
	e = errors.Join(e, held.Close())
	bindings, readErr := s.st.ListBindings()
	s.mu.Unlock()
	if e != nil {
		return out, publicError(e)
	}
	if readErr != nil {
		return out, readErr
	}
	s.changed()
	if !apply {
		return out, nil
	}
	for _, b := range bindings {
		if b.SiteID == nil || *b.SiteID != siteID || b.Orphan {
			continue
		}
		row := domain.ApplyTargetResult{Target: b.Target, Status: domain.Failed}
		if e = s.rewriteBinding(ctx, b, out.Site.BaseURL, ""); e != nil {
			row.Message = publicError(e).Error()
		} else {
			row.Ok = true
			row.Status, e = s.bindingStatus(b.Target)
			if e != nil {
				row.Ok = false
				row.Message = publicError(e).Error()
				row.Status = domain.Failed
			} else {
				row.Message = "route_updated"
			}
		}
		out.Results = append(out.Results, row)
	}
	return out, partialError(out.Results)
}
