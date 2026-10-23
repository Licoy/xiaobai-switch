package adapters

import (
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"path/filepath"
	"strings"
)

type StatusRequest struct {
	Target        domain.TargetKind
	Home, EnvPath string
	Binding       *domain.TargetBinding
	Site          *domain.SiteRow
	APIKey        string `json:"-"`
}

// Inspect also detects orphaned managed configuration when no database binding exists.
func Inspect(r StatusRequest) (State, error) {
	if !r.Target.Valid() || !filepath.IsAbs(r.Home) {
		return State{}, errors.New("validation_failed: target and absolute home required")
	}
	if r.Binding != nil {
		if r.Binding.Target != r.Target {
			return State{}, errors.New("invalid_binding: target mismatch")
		}
		b := *r.Binding
		b.ManagedPaths = append([]string(nil), b.ManagedPaths...)
		for i, path := range b.ManagedPaths {
			name := filepath.Base(path)
			switch name {
			case "config.toml", "settings.json", "auth.json", "models.json":
				b.ManagedPaths[i] = filepath.Join(r.Home, name)
			case "codex.env":
				if r.EnvPath != "" {
					b.ManagedPaths[i] = r.EnvPath
				}
			}
		}
		state, e := ReadState(&b)
		if e != nil {
			return state, e
		}
		if r.Site == nil || b.Orphan {
			state.Status = domain.Orphan
			return state, nil
		}
		if r.APIKey != "" && fingerprint(r.APIKey) != b.KeyFingerprint {
			state.Status = domain.Stale
			state.Differences = append(state.Differences, "site_api_key")
		}
		if b.SiteID == nil || *b.SiteID != r.Site.ID {
			state.Status = domain.Stale
			state.Differences = append(state.Differences, "site_id")
		}
		normalized := normalizeBinding(b)
		codex := r.Target == domain.Codex || (r.Target != domain.ClaudeCode && r.Site.Protocol == domain.OpenAICompatible)
		base, e := normalizedURL(r.Site.BaseURL, codex)
		if e != nil {
			return state, e
		}
		name, path := "settings.json", []string{"env", "ANTHROPIC_BASE_URL"}
		if r.Target == domain.Codex && b.ProviderID != nil {
			name, path = "config.toml", []string{"model_providers", *b.ProviderID, "base_url"}
		}
		if (r.Target == domain.Pi || r.Target == domain.Prime) && b.ProviderID != nil {
			name, path = "models.json", []string{"providers", *b.ProviderID, "baseUrl"}
		}
		wantKey := name + ":" + joinPath(path)
		if want, ok := normalized.ExpectedFields[wantKey]; ok {
			if !expectedEqual(base, true, want) {
				state.Status = domain.Stale
				state.Differences = append(state.Differences, "site_base_url")
			}
		} else if r.Target == domain.Pi || r.Target == domain.Prime {
			providerKey := "models.json:providers/"
			if b.ProviderID != nil {
				providerKey += *b.ProviderID
			}
			if raw, ok := normalized.ExpectedFields[providerKey]; ok {
				d, e := parseJSON([]byte(raw), "strict")
				if e != nil {
					return state, e
				}
				v, _ := d.get([]string{"baseUrl"})
				if v != base {
					state.Status = domain.Stale
					state.Differences = append(state.Differences, "site_base_url")
				}
			}
		}
		return state, nil
	}
	state := State{Status: domain.NotApplied, Fields: map[string]string{}}
	p := Plan{}
	switch r.Target {
	case domain.ClaudeCode:
		e, err := openEditor(&p, filepath.Join(r.Home, "settings.json"), "strict")
		if err != nil {
			return state, err
		}
		_, base := e.doc.get([]string{"env", "ANTHROPIC_BASE_URL"})
		_, token := e.doc.get([]string{"env", "ANTHROPIC_AUTH_TOKEN"})
		_, key := e.doc.get([]string{"env", "ANTHROPIC_API_KEY"})
		if base && (token || key) {
			state.Status = domain.Orphan
		}
	case domain.Codex:
		e, err := openEditor(&p, filepath.Join(r.Home, "config.toml"), "toml")
		if err != nil {
			return state, err
		}
		t := e.doc.(*tomlDoc)
		if t.hasManagedProvider() {
			state.Status = domain.Orphan
		}
		active, _ := t.get([]string{"model_provider"})
		if strings.HasPrefix(asString(active), "xiaobai_") {
			state.Status = domain.Orphan
		}
	case domain.Pi, domain.Prime:
		e, err := openEditor(&p, filepath.Join(r.Home, "models.json"), "models")
		if err != nil {
			return state, err
		}
		providers, _ := e.doc.get([]string{"providers"})
		if obj, ok := providers.(map[string]any); ok {
			for key := range obj {
				if strings.HasPrefix(key, "xiaobai_") {
					state.Status = domain.Orphan
				}
			}
		}
	}
	return state, nil
}
