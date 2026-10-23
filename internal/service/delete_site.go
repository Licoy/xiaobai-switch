package service

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

type DeleteSiteResult struct {
	Deleted bool                       `json:"deleted"`
	Results []domain.ApplyTargetResult `json:"results"`
}

// DeleteSite retains CLI configuration and atomically marks bindings orphaned.
func (s *Service) DeleteSite(ctx context.Context, id string) error {
	return s.deleteSite(ctx, id, false)
}
func (s *Service) deleteSite(ctx context.Context, id string, requireClean bool) (err error) {
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
	if e := s.recoverPending(); e != nil {
		return e
	}
	held, e := s.siteLock(id)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	if requireClean {
		bindings, e := s.st.ListBindings()
		if e != nil {
			return e
		}
		for _, b := range bindings {
			if b.SiteID != nil && *b.SiteID == id {
				return domain.NewError("config_changed", "site gained a target binding during cleanup")
			}
		}
	}
	if e = s.st.DeleteSite(id); e != nil {
		return e
	}
	notify = true
	return nil
}

// Each target cleanup is its own recoverable transaction; failures keep the site.
func (s *Service) DeleteSiteWithCleanup(ctx context.Context, id string) (DeleteSiteResult, error) {
	out := DeleteSiteResult{Results: []domain.ApplyTargetResult{}}
	if e := checkContext(ctx); e != nil {
		return out, e
	}
	s.mu.Lock()
	bindings, e := s.st.ListBindings()
	s.mu.Unlock()
	if e != nil {
		return out, e
	}
	for _, b := range bindings {
		if b.SiteID == nil || *b.SiteID != id {
			continue
		}
		e = s.restore(ctx, restoreOptions{Target: b.Target, ExpectedSiteID: id})
		row := domain.ApplyTargetResult{Target: b.Target, Status: domain.NotApplied, Ok: e == nil || isCommitted(e)}
		if e != nil {
			row.Message = publicError(e).Error()
			if !row.Ok {
				row.Status = domain.Failed
			}
			out.Results = append(out.Results, row)
			return out, e
		}
		row.Message = "target_cleaned"
		out.Results = append(out.Results, row)
	}
	if e = s.deleteSite(ctx, id, true); e != nil {
		return out, e
	}
	out.Deleted = true
	return out, nil
}
