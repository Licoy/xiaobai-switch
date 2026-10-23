package adapters

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"path/filepath"
	"strings"
)

func modelContext(id string, on bool) string {
	id = strings.TrimSpace(id)
	if on && !strings.HasSuffix(strings.ToLower(id), "[1m]") {
		id += "[1m]"
	}
	return id
}
func buildClaude(r Request, p *Plan) error {
	e, err := openEditor(p, filepath.Join(r.Home, "settings.json"), "strict")
	if err != nil {
		return err
	}
	base, err := normalizedURL(r.Site.BaseURL, false)
	if err != nil {
		return err
	}
	p.Binding.ExpectedFields["meta:auth_source"] = "site"
	auth := r.Site.ClaudeAuthKeyStyle
	if r.Claude.AuthKeyStyle != nil {
		auth = *r.Claude.AuthKeyStyle
		p.Binding.ExpectedFields["meta:auth_source"] = "custom"
	}
	if auth == "" {
		auth = domain.AnthropicAuthToken
	}
	if auth != domain.AnthropicAPIKey && auth != domain.AnthropicAuthToken {
		return domain.NewError("validation_failed", "unknown Claude auth style")
	}
	fields := []struct {
		path   []string
		value  any
		remove bool
	}{{[]string{"env", "ANTHROPIC_BASE_URL"}, base, false}, {[]string{"env", auth.EnvKey()}, r.APIKey, false}, {[]string{"env", "ANTHROPIC_MODEL"}, nil, true}, {[]string{"env", "CLAUDE_CODE_EFFORT_LEVEL"}, nil, true}, {[]string{"model"}, modelContext(r.ModelID, r.Claude.Use1mContext), false}}
	other, exists := e.doc.get([]string{"env", auth.OtherEnvKey()})
	if r.ForceExclusive || (exists && (other == r.APIKey || (r.BeforeBinding != nil && fingerprint(asString(other)) == r.BeforeBinding.KeyFingerprint))) {
		fields = append(fields, struct {
			path   []string
			value  any
			remove bool
		}{[]string{"env", auth.OtherEnvKey()}, nil, true})
	}
	for _, f := range fields {
		if err = e.write(f.path, f.value, f.remove); err != nil {
			return err
		}
	}
	for _, a := range []struct {
		name    string
		id      *string
		context bool
	}{{"FABLE", r.Claude.FableModelID, false}, {"OPUS", r.Claude.OpusModelID, r.Claude.Use1mContext}, {"SONNET", r.Claude.SonnetModelID, r.Claude.Use1mContext}, {"HAIKU", r.Claude.HaikuModelID, false}} {
		k := "ANTHROPIC_DEFAULT_" + a.name + "_MODEL"
		if a.id != nil && strings.TrimSpace(*a.id) != "" {
			err = e.write([]string{"env", k}, modelContext(*a.id, a.context), false)
		} else if managed(r.BeforeBinding, k) {
			err = e.write([]string{"env", k}, nil, true)
		}
		if err != nil {
			return err
		}
	}
	if r.Claude.EffortLevel != nil {
		level := string(*r.Claude.EffortLevel)
		if level == "max" {
			level = "xhigh"
		}
		err = e.write([]string{"effortLevel"}, level, false)
	} else if r.BeforeBinding != nil {
		if _, ok := r.BeforeBinding.ExpectedFields["settings.json:effortLevel"]; ok {
			err = e.write([]string{"effortLevel"}, nil, true)
		}
	}
	if err != nil {
		return err
	}
	p.Binding.ManagedEnvKeys = []string{"ANTHROPIC_BASE_URL", auth.EnvKey()}
	for _, k := range []string{"FABLE", "OPUS", "SONNET", "HAIKU"} {
		name := "ANTHROPIC_DEFAULT_" + k + "_MODEL"
		if _, ok := p.Binding.ExpectedFields["settings.json:env/"+name]; ok {
			p.Binding.ManagedEnvKeys = append(p.Binding.ManagedEnvKeys, name)
		}
	}
	e.finish()
	return nil
}
func managed(b *domain.TargetBinding, key string) bool {
	if b == nil {
		return false
	}
	for _, k := range b.ManagedEnvKeys {
		if k == key {
			return true
		}
	}
	return false
}
func asString(v any) string { s, _ := v.(string); return s }
