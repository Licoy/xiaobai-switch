package adapters

import (
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/envinject"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"strings"
)

func Rewrite(b domain.TargetBinding, baseURL, apiKey string) (Plan, error) {
	b = normalizeBinding(b)
	p := Plan{Target: b.Target, Binding: b}
	p.Binding.ExpectedFields = map[string]string{}
	for k, v := range b.ExpectedFields {
		p.Binding.ExpectedFields[k] = v
	}
	if b.SiteID != nil {
		p.SiteID = *b.SiteID
	}
	docs, e := bindingDocs(b)
	if e != nil {
		return p, e
	}
	var doc *editor
	var basePath, keyPath []string
	codex := false
	id := ""
	if b.ProviderID != nil {
		id = *b.ProviderID
	}
	switch b.Target {
	case domain.ClaudeCode:
		doc = docs["settings.json"]
		basePath = []string{"env", "ANTHROPIC_BASE_URL"}
		for _, key := range b.ManagedEnvKeys {
			if strings.Contains(key, "API_KEY") || strings.Contains(key, "AUTH_TOKEN") {
				keyPath = []string{"env", key}
				break
			}
		}
	case domain.Codex:
		doc = docs["config.toml"]
		basePath = []string{"model_providers", id, "base_url"}
		codex = true
	case domain.Pi, domain.Prime:
		doc = docs["models.json"]
		basePath = []string{"providers", id, "baseUrl"}
		if doc == nil {
			return p, errors.New("invalid_binding: missing models document")
		}
		api, _ := doc.doc.get([]string{"providers", id, "api"})
		codex = api == "openai-completions"
	default:
		return p, errors.New("validation_failed: unknown target")
	}
	if doc == nil {
		return p, errors.New("invalid_binding: missing config document")
	}
	if baseURL != "" {
		base, e := normalizedURL(baseURL, codex)
		if e != nil {
			return p, e
		}
		if e = doc.doc.set(basePath, base, false); e != nil {
			return p, e
		}
	}
	if apiKey != "" {
		p.Binding.KeyFingerprint = fingerprint(apiKey)
		switch b.Target {
		case domain.ClaudeCode:
			if len(keyPath) == 0 {
				return p, errors.New("invalid_binding: no managed auth key")
			}
			if e = doc.doc.set(keyPath, apiKey, false); e != nil {
				return p, e
			}
		case domain.Pi, domain.Prime:
			a := docs["auth.json"]
			if a == nil {
				return p, errors.New("invalid_binding: missing auth")
			}
			if e = a.doc.set([]string{id, "key"}, EncodeAuthLiteral(apiKey), false); e != nil {
				return p, e
			}
		}
	}
	if baseURL != "" {
		if e = updateWrittenExpectation(&p.Binding, doc, basePath); e != nil {
			return p, e
		}
	}
	if apiKey != "" {
		if b.Target == domain.ClaudeCode {
			if e = updateWrittenExpectation(&p.Binding, doc, keyPath); e != nil {
				return p, e
			}
		} else if b.Target == domain.Pi || b.Target == domain.Prime {
			if e = updateWrittenExpectation(&p.Binding, docs["auth.json"], []string{id, "key"}); e != nil {
				return p, e
			}
		}
	}
	for _, d := range docs {
		d.change.After = d.doc.bytes()
		p.Changes = append(p.Changes, d.change)
	}
	if b.Target == domain.Codex && apiKey != "" {
		if e = rewriteEnv(&p, b, apiKey, false); e != nil {
			return p, e
		}
		if b.ExpectedFields["meta:env_mode"] == "user_env" && len(b.ManagedEnvKeys) > 0 {
			p.Injection = &envinject.Plan{Mode: "user_env", UserKey: b.ManagedEnvKeys[0], UserValue: apiKey, UserExpectedFingerprint: b.KeyFingerprint}
		}
	}
	return p, nil
}
func appendChange(p *Plan, c fileops.Change) { p.Changes = append(p.Changes, c) }
