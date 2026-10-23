package service

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

type connectionChange struct {
	Input       domain.UpdateSiteInput
	ChangedKeys map[string]bool
}

func (s *Service) syncConnections(ctx context.Context, site domain.SiteDTO, change connectionChange) ([]domain.ApplyTargetResult, error) {
	in := change.Input
	s.mu.Lock()
	bindings, e := s.st.ListBindings()
	s.mu.Unlock()
	if e != nil {
		return nil, e
	}
	out := []domain.ApplyTargetResult{}
	for _, b := range bindings {
		if b.SiteID == nil || *b.SiteID != site.ID || b.Orphan {
			continue
		}
		keyID := ""
		if b.APIKey.ID != nil {
			keyID = *b.APIKey.ID
		} else if site.ActiveAPIKeyID != nil {
			keyID = *site.ActiveAPIKeyID
		}
		s.mu.Lock()
		key, e := s.st.GetSiteAPIKey(site.ID, keyID)
		s.mu.Unlock()
		row := domain.ApplyTargetResult{Target: b.Target, Status: domain.Failed}
		if e == nil {
			o := adapters.ConnectionOptions{Site: domain.SiteRow{ID: site.ID, Name: site.Name, BaseURL: site.BaseURL, Protocol: domain.SiteProtocol(site.Protocol)}, APIKey: "", Capabilities: in.Capabilities, ProtocolChanged: in.Protocol != nil}
			if change.ChangedKeys[keyID] || (b.Target == domain.ClaudeCode && in.ClaudeAuthKeyStyle != nil) {
				o.APIKey = key
			}
			if in.ClaudeAuthKeyStyle != nil {
				style := domain.ClaudeAuthKeyStyle(*in.ClaudeAuthKeyStyle)
				o.AuthStyle = &style
			}
			e = s.rewriteManaged(ctx, b, rewriteGuard{BaseURL: site.BaseURL, KeyID: keyID, Key: o.APIKey, Connection: &o, SiteRevision: revision([]any{site.BaseURL, site.Protocol, site.ClaudeAuthKeyStyle, site.Capabilities}), CheckModels: change.ChangedKeys[keyID]})
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
				row.Message = "connection_updated"
			}
		}
		out = append(out, row)
	}
	return out, nil
}
func (s *Service) syncURL(ctx context.Context, site domain.SiteDTO) ([]domain.ApplyTargetResult, error) {
	s.mu.Lock()
	bindings, e := s.st.ListBindings()
	s.mu.Unlock()
	if e != nil {
		return nil, e
	}
	out := []domain.ApplyTargetResult{}
	for _, b := range bindings {
		if b.SiteID == nil || *b.SiteID != site.ID || b.Orphan {
			continue
		}
		row := domain.ApplyTargetResult{Target: b.Target, Status: domain.Failed}
		if e = s.rewriteBinding(ctx, b, site.BaseURL, ""); e != nil {
			row.Message = publicError(e).Error()
		} else {
			row.Ok = true
			row.Status, e = s.bindingStatus(b.Target)
			if e != nil {
				row.Ok = false
				row.Status = domain.Failed
				row.Message = publicError(e).Error()
			}
		}
		out = append(out, row)
	}
	return out, nil
}
