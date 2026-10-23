package service

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/secure"
	"strings"
	"time"
)

type rewriteGuard struct {
	SiteRevision        string
	CheckModels         bool
	BaseURL, KeyID, Key string
	RequireActive       bool
	Connection          *adapters.ConnectionOptions
}

func (s *Service) rewriteBinding(ctx context.Context, b domain.TargetBinding, base, key string) error {
	return s.rewriteManaged(ctx, b, rewriteGuard{BaseURL: base, Key: key})
}
func (s *Service) rewriteManaged(ctx context.Context, b domain.TargetBinding, g rewriteGuard) (err error) {
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
	current, e := s.st.GetBinding(b.Target)
	if e != nil {
		return e
	}
	if current == nil || revision(current) != revision(b) {
		return domain.NewError("config_changed", "binding changed before rewrite")
	}
	if b.SiteID == nil {
		return domain.NewError("invalid_binding", "rewrite requires bound site")
	}
	if e = s.validateBinding(&b); e != nil {
		return e
	}
	var p adapters.Plan
	if g.Connection != nil {
		p, e = adapters.Reconfigure(b, *g.Connection)
	} else {
		p, e = adapters.Rewrite(b, g.BaseURL, g.Key)
	}
	if e != nil {
		return publicError(e, g.Key)
	}
	held, e := s.acquire(p)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	site, e := s.st.GetSite(*b.SiteID)
	if e != nil {
		return e
	}
	if g.SiteRevision != "" && g.SiteRevision != revision([]any{site.BaseURL, site.Protocol, site.ClaudeAuthKeyStyle, site.Capabilities}) {
		return domain.NewError("config_changed", "site connection settings changed before rewrite")
	}
	if site.BaseURL != g.BaseURL {
		return domain.NewError("config_changed", "site route changed before rewrite")
	}
	if g.RequireActive && (site.ActiveAPIKeyID == nil || *site.ActiveAPIKeyID != g.KeyID) {
		return domain.NewError("config_changed", "active key changed before rewrite")
	}
	keyID := g.KeyID
	if keyID == "" && b.APIKey.ID != nil {
		keyID = *b.APIKey.ID
	}
	if keyID == "" && site.ActiveAPIKeyID != nil {
		keyID = *site.ActiveAPIKeyID
	}
	if keyID == "" {
		return domain.NewError("invalid_binding", "rewrite key is missing")
	}
	if g.Key != "" {
		currentKey, e := s.st.GetSiteAPIKey(site.ID, keyID)
		if e != nil {
			return e
		}
		if secure.KeyFingerprint(currentKey) != secure.KeyFingerprint(g.Key) {
			return domain.NewError("config_changed", "key material changed before rewrite")
		}
	}
	current, e = s.st.GetBinding(b.Target)
	if e != nil {
		return e
	}
	if current == nil || revision(current) != revision(b) {
		return domain.NewError("config_changed", "binding changed while acquiring rewrite locks")
	}
	if g.CheckModels {
		models, e := s.st.ListModelsForKey(site.ID, keyID)
		if e != nil {
			return e
		}
		ids := map[string]bool{}
		for _, m := range models {
			ids[m.ModelID] = true
		}
		refs, e := adapters.ReferencedModels(b)
		if e != nil {
			return e
		}
		for _, model := range refs {
			trimmed := strings.TrimSpace(model)
			without := trimmed
			if strings.HasSuffix(strings.ToLower(trimmed), "[1m]") {
				without = strings.TrimSpace(trimmed[:len(trimmed)-4])
			}
			if !ids[trimmed] && !ids[without] {
				return domain.NewError("model_unavailable", "current model or Claude alias is unavailable on the selected key")
			}
		}
	}
	id := domain.NewID()
	p.Binding.ApplyRecordID = &id
	if g.Key != "" {
		p.Binding.APIKey.ID = &keyID
	}
	for _, k := range site.APIKeys {
		if g.Key != "" && k.ID == keyID {
			label, prefix := k.Label, k.KeyPrefix
			p.Binding.APIKey.Label = &label
			p.Binding.APIKey.Prefix = &prefix
		}
	}
	_, e = s.transact(p, serviceJournal{Operation: "rewrite", RecordID: id, BeforeBinding: &b}, func(dir string) error {
		record := domain.ApplyRecordDTO{ID: id, SiteID: &site.ID, SiteNameSnapshot: site.Name, Target: string(b.Target), ModelID: b.ModelID, ProviderID: p.Binding.ProviderID, Status: "applied", BackupDir: &dir, AppliedAt: time.Now().UnixMilli(), SiteAPIKeyID: p.Binding.APIKey.ID, SiteAPIKeyLabelSnapshot: p.Binding.APIKey.Label, SiteAPIKeyPrefixSnapshot: p.Binding.APIKey.Prefix}
		return s.st.CommitRewrite(p.Binding, record, domain.TouchedKeys{})
	})
	if e == nil {
		notify = true
	}
	return publicError(e, g.Key)
}
