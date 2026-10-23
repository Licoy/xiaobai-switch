package service

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"time"
)

func (s *Service) Apply(ctx context.Context, previewID string) (result domain.ApplyResult, err error) {
	if e := checkContext(ctx); e != nil {
		return result, e
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
		return result, publicError(e)
	}
	pnd, ok := s.pending[previewID]
	delete(s.pending, previewID)
	if !ok || !time.Now().Before(pnd.Expires) {
		return result, domain.NewError("preview_expired", "preview is missing or expired")
	}
	v, e := s.load(pnd.Request)
	if e != nil {
		return result, publicError(e)
	}
	if revision(v) != pnd.Revision {
		return result, domain.NewError("config_changed", "site, key, model cache, preset, binding or settings changed after preview")
	}
	plan, e := s.build(v)
	if e != nil {
		return result, e
	}
	if fileRevision(plan) != pnd.FilesRevision {
		return result, domain.NewError("config_changed", "target configuration changed after preview")
	}
	held, e := s.acquire(plan)
	if e != nil {
		return result, publicError(e, v.Key)
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	fresh, e := s.load(pnd.Request)
	if e != nil {
		return result, e
	}
	if revision(fresh) != pnd.Revision {
		return result, domain.NewError("config_changed", "application state changed while acquiring locks")
	}
	if e = checkContext(ctx); e != nil {
		return result, e
	}
	recordID := domain.NewID()
	plan.Binding.ApplyRecordID = &recordID
	keyID := v.APIKeyID
	plan.Binding.APIKey.ID = &keyID
	for _, key := range v.Site.APIKeys {
		if key.ID == keyID {
			label, prefix := key.Label, key.KeyPrefix
			plan.Binding.APIKey.Label = &label
			plan.Binding.APIKey.Prefix = &prefix
		}
	}
	touched := domain.TouchedKeys{}
	for _, f := range plan.Changes {
		if f.Existed {
			touched.Paths = append(touched.Paths, f.Path)
		} else {
			touched.CreatedPaths = append(touched.CreatedPaths, f.Path)
		}
	}
	touched.EnvKeys = plan.Binding.ManagedEnvKeys
	dir, e := s.transact(plan, serviceJournal{Operation: "apply", RecordID: recordID, BeforeBinding: v.Binding}, func(dir string) error {
		at := time.Now().UnixMilli()
		record := domain.ApplyRecordDTO{ID: recordID, SiteID: &v.Site.ID, SiteNameSnapshot: v.Site.Name, Target: string(plan.Target), ModelID: v.Request.ModelID, ProviderID: plan.Binding.ProviderID, Status: "applied", BackupDir: &dir, AppliedAt: at, SiteAPIKeyID: &keyID, SiteAPIKeyLabelSnapshot: plan.Binding.APIKey.Label, SiteAPIKeyPrefixSnapshot: plan.Binding.APIKey.Prefix}
		preset := domain.TargetPreset{SiteID: v.Site.ID, APIKeyID: keyID, Target: plan.Target, Options: v.Request, UpdatedAt: at}
		return s.st.CommitApplication(plan.Binding, record, touched, preset)
	})
	if e != nil && !isCommitted(e) {
		return result, publicError(e, v.Key)
	}
	notify = true
	result = domain.ApplyResult{SiteID: v.Site.ID, ModelID: v.Request.ModelID, AppliedAt: time.Now().UnixMilli(), Results: []domain.ApplyTargetResult{{Target: plan.Target, Ok: true, Status: domain.Applied, BackupPaths: []string{dir}, Message: "applied"}}}
	if e != nil {
		return result, publicError(e, v.Key)
	}
	if e = s.prune(plan.Target, v.Settings.MaxBackupCopies); e != nil {
		return result, committedWarning("apply", plan.Target, result, e)
	}
	return result, nil
}
