package service

import (
	"context"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"github.com/licoy/xiaobai-switch/internal/secure"
	"os"
	"path/filepath"
	"time"
)

type snapshot struct {
	Hydrated
	Request        domain.ApplyRequest
	Key            string `json:"-"`
	KeyFingerprint string
}

func (s *Service) load(req domain.ApplyRequest) (snapshot, error) {
	out := snapshot{Request: req}
	if !req.Target.Valid() {
		return out, domain.NewError("validation_failed", "unknown target")
	}
	var e error
	out.Site, e = s.st.GetSite(req.SiteID)
	if e != nil {
		return out, e
	}
	if !out.Site.Enabled {
		return out, domain.NewError("validation_failed", "site is disabled")
	}
	keyID := req.APIKeyID
	if keyID == "" && out.Site.ActiveAPIKeyID != nil {
		keyID = *out.Site.ActiveAPIKeyID
	}
	var selected *domain.SiteApiKeySummary
	for i := range out.Site.APIKeys {
		if out.Site.APIKeys[i].ID == keyID {
			selected = &out.Site.APIKeys[i]
			break
		}
	}
	if selected == nil {
		return out, domain.NewError("validation_failed", "key does not belong to site")
	}
	out.APIKeyID = keyID
	out.Request.APIKeyID = keyID
	out.Key, e = s.st.GetSiteAPIKey(req.SiteID, keyID)
	if e != nil {
		return out, e
	}
	out.KeyFingerprint = secure.KeyFingerprint(out.Key)
	out.Models, e = s.st.ListModelsForKey(req.SiteID, keyID)
	if e != nil {
		return out, e
	}
	if out.Request.ModelID == "" && selected.SelectedModelID != nil {
		out.Request.ModelID = *selected.SelectedModelID
	}
	found := false
	for _, m := range out.Models {
		if m.SiteID != req.SiteID || m.APIKeyID != keyID {
			return out, domain.NewError("invalid_config", "model cache key ownership mismatch")
		}
		found = found || m.ModelID == out.Request.ModelID
	}
	if !found {
		return out, domain.NewError("validation_failed", "selected model is not cached for this site key")
	}
	out.Settings, e = s.st.GetSettings()
	if e != nil {
		return out, e
	}
	out.Preset, e = s.st.GetTargetPreset(req.SiteID, keyID, req.Target)
	if e != nil {
		return out, e
	}
	if domain.IsThinkingTarget(req.Target) {
		out.Thinking, e = s.st.GetThinkingPreset(req.SiteID, req.Target)
		if e != nil {
			return out, e
		}
	}
	out.Binding, e = s.st.GetBinding(req.Target)
	return out, e
}
func revision(v any) string { raw, _ := json.Marshal(v); return secure.KeyFingerprint(string(raw)) }
func fileRevision(p adapters.Plan) string {
	parts := []any{}
	for _, f := range p.Changes {
		parts = append(parts, []any{f.Path, f.Existed, secure.KeyFingerprint(string(f.Before))})
	}
	return revision(parts)
}
func (s *Service) build(v snapshot) (adapters.Plan, error) {
	home, e := s.targetHome(v.Request.Target, v.Settings)
	if e != nil {
		return adapters.Plan{}, e
	}
	if e = s.validateTargetFiles(v.Request.Target, home); e != nil {
		return adapters.Plan{}, e
	}
	if e = s.validateBinding(v.Binding); e != nil {
		return adapters.Plan{}, e
	}
	env, e := s.path("env", "codex.env")
	if e != nil {
		return adapters.Plan{}, e
	}
	r := adapters.Request{Target: v.Request.Target, Site: domain.SiteRow{ID: v.Site.ID, Name: v.Site.Name, BaseURL: v.Site.BaseURL, Protocol: domain.SiteProtocol(v.Site.Protocol), ClaudeAuthKeyStyle: domain.ClaudeAuthKeyStyle(v.Site.ClaudeAuthKeyStyle)}, APIKey: v.Key, ModelID: v.Request.ModelID, Home: home, EnvPath: env, EnvMode: v.Settings.CodexEnvInjectMode, ShellHome: s.options.Home, Platform: s.options.Platform, BeforeBinding: v.Binding, ForceExclusive: v.Settings.ForceExclusiveClaudeAuthKey, Claude: v.Request.Claude, Codex: v.Request.Codex, Pi: v.Request.Pi, Prime: v.Request.Prime}
	catalog := []domain.CatalogModel{}
	for _, m := range v.Models {
		catalog = append(catalog, domain.CatalogModel{ModelID: m.ModelID, DisplayName: m.DisplayName})
	}
	r.Codex.CatalogModels = catalog
	r.Pi.CatalogModels = catalog
	r.Prime.CatalogModels = catalog
	if r.Codex.CapabilitySource != "custom" {
		r.Codex.CapabilitySource = domain.CapabilitySite
		r.Codex.RemoteCompaction = v.Site.Capabilities["codex-compact"]
		r.Codex.ImageUnderstanding = v.Site.Capabilities["codex-vision"]
		r.Codex.ImageGeneration = v.Site.Capabilities["codex-imagegen"]
		r.Codex.WebSearch = v.Site.Capabilities["codex-search"]
	}
	if emptyThinking(r.Pi.Thinking) {
		r.Pi.Thinking = v.Thinking.ToWrite()
	}
	if emptyThinking(r.Prime.Thinking) {
		r.Prime.Thinking = v.Thinking.ToWrite()
	}
	p, e := adapters.Build(r)
	if e != nil {
		return p, publicError(e, v.Key)
	}
	for _, f := range p.Changes {
		if _, e = s.validatePath(f.Path); e != nil {
			return p, e
		}
	}
	return p, nil
}
func emptyThinking(w domain.ThinkingWrite) bool {
	return w.DefaultLevel == nil && len(w.Models) == 0 && w.Extended.Xhigh == nil && w.Extended.Max == nil
}
func (s *Service) targetHome(t domain.TargetKind, v domain.AppSettings) (string, error) {
	var override *string
	var parts []string
	switch t {
	case domain.ClaudeCode:
		override = v.ClaudeHomeOverride
		parts = []string{".claude"}
	case domain.Codex:
		override = v.CodexHomeOverride
		parts = []string{".codex"}
	case domain.Pi:
		override = v.PiAgentDirOverride
		parts = []string{".pi", "agent"}
	case domain.Prime:
		override = v.PrimeAgentDirOverride
		parts = []string{".prime", "agent"}
	default:
		return "", domain.NewError("validation_failed", "unknown target")
	}
	if override != nil && *override != "" {
		return paths.ValidatePath(*override)
	}
	actual, e := paths.HomeDir()
	if e != nil {
		return "", e
	}
	if filepath.Clean(actual) == filepath.Clean(s.options.Home) && os.Getenv("XIAOBAI_DEV_HOME") == "" {
		switch t {
		case domain.ClaudeCode:
			return paths.ResolveClaudeHome("")
		case domain.Codex:
			return paths.ResolveCodexHome("")
		case domain.Pi:
			return paths.ResolvePiAgentDir("")
		case domain.Prime:
			return paths.ResolvePrimeAgentDir("")
		}
	}
	return paths.ValidatePath(filepath.Join(append([]string{s.options.Home}, parts...)...))
}

func (s *Service) PrepareApply(ctx context.Context, req domain.ApplyRequest) (Preview, error) {
	if e := checkContext(ctx); e != nil {
		return Preview{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expirePreviews()
	v, e := s.load(req)
	if e != nil {
		return Preview{}, publicError(e)
	}
	p, e := s.build(v)
	if e != nil {
		return Preview{}, publicError(e, v.Key)
	}
	id := domain.NewID()
	s.pending[id] = pending{Request: v.Request, Revision: revision(v), FilesRevision: fileRevision(p), Expires: time.Now().Add(5 * time.Minute)}
	out := Preview{ID: id, Target: req.Target, SiteID: req.SiteID, ModelID: v.Request.ModelID, Files: []string{}, Summary: map[string]string{"site": v.Site.Name, "model": v.Request.ModelID}, Warnings: []string{"credentials_written_to_target"}}
	for _, f := range p.Changes {
		out.Files = append(out.Files, f.Path)
	}
	return out, nil
}
func (s *Service) CancelPreview(ctx context.Context, id string) error {
	if e := checkContext(ctx); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, id)
	return nil
}
func (s *Service) expirePreviews() {
	for id, p := range s.pending {
		if !time.Now().Before(p.Expires) {
			delete(s.pending, id)
		}
	}
}
func (s *Service) Hydrate(ctx context.Context, siteID, keyID string, target domain.TargetKind) (Hydrated, error) {
	if e := checkContext(ctx); e != nil {
		return Hydrated{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	site, e := s.st.GetSite(siteID)
	if e != nil {
		return Hydrated{}, e
	}
	if keyID == "" && site.ActiveAPIKeyID != nil {
		keyID = *site.ActiveAPIKeyID
	}
	found := false
	for _, k := range site.APIKeys {
		found = found || k.ID == keyID
	}
	if !found {
		return Hydrated{}, domain.NewError("validation_failed", "key does not belong to site")
	}
	out := Hydrated{Site: site, APIKeyID: keyID}
	out.Models, e = s.st.ListModelsForKey(siteID, keyID)
	if e != nil {
		return out, e
	}
	out.Preset, e = s.st.GetTargetPreset(siteID, keyID, target)
	if e != nil {
		return out, e
	}
	if domain.IsThinkingTarget(target) {
		out.Thinking, e = s.st.GetThinkingPreset(siteID, target)
		if e != nil {
			return out, e
		}
	}
	out.Binding, e = s.st.GetBinding(target)
	if e != nil {
		return out, e
	}
	out.Settings, e = s.st.GetSettings()
	return out, e
}
func (s *Service) TargetPreset(ctx context.Context, siteID, keyID string, target domain.TargetKind) (*domain.TargetPreset, error) {
	h, e := s.Hydrate(ctx, siteID, keyID, target)
	return h.Preset, e
}
