package service

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/models"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"github.com/licoy/xiaobai-switch/internal/quota"
	"github.com/licoy/xiaobai-switch/internal/secure"
	"time"
)

type networkSnapshot struct {
	Site                 domain.SiteDTO
	KeyID, Key, Revision string
	Settings             domain.AppSettings
}

func (s *Service) networkState(siteID, keyID string) (networkSnapshot, error) {
	v := networkSnapshot{}
	var e error
	v.Site, e = s.st.GetSite(siteID)
	if e != nil {
		return v, e
	}
	if !v.Site.Enabled {
		return v, domain.NewError("validation_failed", "site is disabled")
	}
	if keyID == "" && v.Site.ActiveAPIKeyID != nil {
		keyID = *v.Site.ActiveAPIKeyID
	}
	found := false
	for _, k := range v.Site.APIKeys {
		found = found || k.ID == keyID
	}
	if !found {
		return v, domain.NewError("validation_failed", "key does not belong to site")
	}
	v.KeyID = keyID
	v.Key, e = s.st.GetSiteAPIKey(siteID, keyID)
	if e != nil {
		return v, e
	}
	v.Settings, e = s.st.GetSettings()
	if e != nil {
		return v, e
	}
	v.Revision = revision([]any{v.Site.BaseURL, v.Site.Protocol, v.Site.Enabled, v.Site.ActiveAPIKeyID, keyID, secure.KeyFingerprint(v.Key), v.Settings})
	v.Site.ActiveAPIKeyID = &keyID
	return v, nil
}
func (s *Service) FetchModels(ctx context.Context, siteID, keyID string) (result domain.FetchModelsResult, err error) {
	if e := checkContext(ctx); e != nil {
		return domain.FetchModelsResult{}, e
	}
	s.mu.Lock()
	v, e := s.networkState(siteID, keyID)
	s.mu.Unlock()
	if e != nil {
		return domain.FetchModelsResult{}, publicError(e)
	}
	client, e := netx.NewClient(v.Settings, 30*time.Second)
	if e != nil {
		return domain.FetchModelsResult{}, e
	}
	defer client.CloseIdleConnections()
	result, fetchErr := models.Fetch(ctx, client, v.Site, v.Key)
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
		return result, e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	current, e := s.networkState(siteID, v.KeyID)
	if e != nil {
		return result, publicError(e, v.Key)
	}
	if current.Revision != v.Revision {
		return result, domain.NewError("config_changed", "site or key changed while fetching models")
	}
	if fetchErr != nil {
		message := publicError(fetchErr, v.Key).Error()
		if e = s.st.UpdateModelFetch(siteID, v.KeyID, nil, nil, &message); e != nil {
			return result, publicError(e, v.Key)
		}
		notify = true
		return result, publicError(fetchErr, v.Key)
	}
	if e = s.st.ReplaceModels(siteID, v.KeyID, result.Models); e != nil {
		return result, publicError(e, v.Key)
	}
	at, latency := result.FetchedAt, int64(result.LatencyMS)
	if e = s.st.UpdateModelFetch(siteID, v.KeyID, &at, &latency, nil); e != nil {
		return result, publicError(e, v.Key)
	}
	result.Models, e = s.st.ListModelsForKey(siteID, v.KeyID)
	if e != nil {
		return result, e
	}
	notify = true
	return result, nil
}
func (s *Service) ProbeModel(ctx context.Context, siteID, keyID, modelID string) (domain.ModelProbeResult, error) {
	if e := checkContext(ctx); e != nil {
		return domain.ModelProbeResult{}, e
	}
	s.mu.Lock()
	v, e := s.networkState(siteID, keyID)
	s.mu.Unlock()
	if e != nil {
		return domain.ModelProbeResult{}, e
	}
	client, e := netx.NewClient(v.Settings, 30*time.Second)
	if e != nil {
		return domain.ModelProbeResult{}, e
	}
	defer client.CloseIdleConnections()
	result, e := models.Probe(ctx, client, v.Site, v.Key, modelID)
	if e == nil {
		e = s.checkNetworkRevision(v)
	}
	return result, publicError(e, v.Key)
}
func (s *Service) ProbeKey(ctx context.Context, siteID, keyID string) (domain.ProbeSiteApiKeyResult, error) {
	if e := checkContext(ctx); e != nil {
		return domain.ProbeSiteApiKeyResult{}, e
	}
	s.mu.Lock()
	v, e := s.networkState(siteID, keyID)
	s.mu.Unlock()
	if e != nil {
		return domain.ProbeSiteApiKeyResult{}, e
	}
	client, e := netx.NewClient(v.Settings, 30*time.Second)
	if e != nil {
		return domain.ProbeSiteApiKeyResult{}, e
	}
	defer client.CloseIdleConnections()
	result, e := models.ProbeKey(ctx, client, v.Site, v.Key)
	if e == nil {
		e = s.checkNetworkRevision(v)
	}
	return result, publicError(e, v.Key)
}
func (s *Service) ProbeQuota(ctx context.Context, siteID, keyID string) (domain.SiteQuota, error) {
	if e := checkContext(ctx); e != nil {
		return domain.SiteQuota{}, e
	}
	s.mu.Lock()
	v, e := s.networkState(siteID, keyID)
	s.mu.Unlock()
	if e != nil {
		return domain.SiteQuota{}, e
	}
	client, e := netx.NewClient(v.Settings, 30*time.Second)
	if e != nil {
		return domain.SiteQuota{}, e
	}
	defer client.CloseIdleConnections()
	result, e := quota.Probe(ctx, client, v.Site, v.Key)
	if e == nil {
		e = s.checkNetworkRevision(v)
	}
	return result, publicError(e, v.Key)
}

func (s *Service) checkNetworkRevision(v networkSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, e := s.networkState(v.Site.ID, v.KeyID)
	if e != nil {
		return e
	}
	if current.Revision != v.Revision {
		return domain.NewError("config_changed", "site or key changed while probing")
	}
	return nil
}
