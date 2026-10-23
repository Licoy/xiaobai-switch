package service

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func (s *Service) Revert(ctx context.Context, target domain.TargetKind) error {
	return s.restore(ctx, restoreOptions{Target: target})
}
func (s *Service) RestoreOfficial(ctx context.Context, target domain.TargetKind) error {
	return s.restore(ctx, restoreOptions{Target: target, Official: true})
}
func (s *Service) CleanupOrphan(ctx context.Context, target domain.TargetKind) error {
	return s.restore(ctx, restoreOptions{Target: target, OrphanCleanup: true})
}

type restoreOptions struct {
	Target                  domain.TargetKind
	Official, OrphanCleanup bool
	ExpectedSiteID          string
}

func (s *Service) restore(ctx context.Context, options restoreOptions) (err error) {
	target, official := options.Target, options.Official
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
		return publicError(e)
	}
	if !target.Valid() {
		return domain.NewError("validation_failed", "unknown target")
	}
	binding, e := s.st.GetBinding(target)
	if e != nil {
		return e
	}
	if options.ExpectedSiteID != "" && (binding == nil || binding.SiteID == nil || *binding.SiteID != options.ExpectedSiteID) {
		return domain.NewError("config_changed", "target binding changed before site cleanup")
	}
	if options.OrphanCleanup {
		if binding == nil {
			if !domain.IsThinkingTarget(target) {
				return nil
			}
			official = true
		} else {
			official = false
		}
	}
	if e = s.validateBinding(binding); e != nil {
		return e
	}
	settings, e := s.st.GetSettings()
	if e != nil {
		return e
	}
	var plan adapters.Plan
	if official {
		home, e := s.targetHome(target, settings)
		if e != nil {
			return e
		}
		env, e := s.path("env", "codex.env")
		if e != nil {
			return e
		}
		if e = s.validateTargetFiles(target, home); e != nil {
			return e
		}
		plan, e = adapters.RestoreOfficial(adapters.RestoreRequest{Target: target, Home: home, EnvPath: env, Binding: binding})
		if e != nil {
			return publicError(e)
		}
	} else {
		if binding == nil {
			return domain.NewError("not_found", "target has no tracked application")
		}
		plan, e = adapters.Revert(*binding)
		if e != nil {
			return publicError(e)
		}
	}
	locks, e := s.acquire(plan)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, locks.Close()) }()
	_, e = s.transact(plan, serviceJournal{Operation: "clear", BeforeBinding: binding, NoDB: binding == nil}, func(string) error {
		recordID := ""
		if binding != nil && binding.ApplyRecordID != nil {
			recordID = *binding.ApplyRecordID
		}
		return s.st.ClearApplication(target, recordID)
	})
	if e != nil && !isCommitted(e) {
		return publicError(e)
	}
	notify = true
	if e != nil {
		return publicError(e)
	}
	return committedWarning("restore", target, nil, s.prune(target, settings.MaxBackupCopies))
}
