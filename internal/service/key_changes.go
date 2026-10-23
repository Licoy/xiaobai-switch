package service

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/secure"
)

func (s *Service) changedInputKeys(siteID string, in domain.UpdateSiteInput) (map[string]bool, error) {
	changed := map[string]bool{}
	if in.APIKeys != nil {
		for _, item := range *in.APIKeys {
			if item.ID == nil || item.APIKey == "" {
				continue
			}
			old, e := s.st.GetSiteAPIKey(siteID, *item.ID)
			if e != nil {
				return nil, e
			}
			if secure.KeyFingerprint(old) != secure.KeyFingerprint(item.APIKey) {
				changed[*item.ID] = true
			}
		}
	} else if in.APIKey != nil && *in.APIKey != "" {
		site, e := s.st.GetSite(siteID)
		if e != nil {
			return nil, e
		}
		if site.ActiveAPIKeyID == nil {
			return nil, domain.NewError("invalid_config", "site has no active key")
		}
		old, e := s.st.GetSiteAPIKey(siteID, *site.ActiveAPIKeyID)
		if e != nil {
			return nil, e
		}
		if secure.KeyFingerprint(old) != secure.KeyFingerprint(*in.APIKey) {
			changed[*site.ActiveAPIKeyID] = true
		}
	}
	return changed, nil
}
