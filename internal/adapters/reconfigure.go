package adapters

import (
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"path/filepath"
	"strings"
)

type ConnectionOptions struct {
	Site            domain.SiteRow
	APIKey          string `json:"-"`
	AuthStyle       *domain.ClaudeAuthKeyStyle
	Capabilities    *domain.SiteCapabilities
	ProtocolChanged bool
}

// Reconfigure changes explicit connection settings while retaining live model options.
func Reconfigure(b domain.TargetBinding, o ConnectionOptions) (Plan, error) {
	b = normalizeBinding(b)
	p, e := Rewrite(b, "", o.APIKey)
	if e != nil {
		return p, e
	}
	id := ""
	if b.ProviderID != nil {
		id = *b.ProviderID
	}
	codex := b.Target == domain.Codex || (b.Target != domain.ClaudeCode && o.Site.Protocol == domain.OpenAICompatible)
	base, e := normalizedURL(o.Site.BaseURL, codex)
	if e != nil {
		return p, e
	}
	for i := range p.Changes {
		c := &p.Changes[i]
		name := filepath.Base(c.Path)
		if name == "codex.env" || name == "xiaobai-model-catalog.json" {
			continue
		}
		var d document
		if name == "config.toml" {
			d, e = parseTOML(c.After)
		} else if name == "models.json" {
			d, e = parseJSON(c.After, "models")
		} else if name == "settings.json" || name == "auth.json" {
			d, e = parseJSON(c.After, "strict")
		} else {
			continue
		}
		if e != nil {
			return p, e
		}
		ed := &editor{doc: d, change: *c, plan: &p, name: name}
		set := func(path []string, value any) error {
			covered := false
			key := name + ":" + joinPath(path)
			for owned := range p.Binding.ExpectedFields {
				covered = covered || owned == key || strings.HasPrefix(key, owned+"/")
			}
			if !covered {
				return ed.write(path, value, false)
			}
			if e := d.set(path, value, false); e != nil {
				return e
			}
			return updateWrittenExpectation(&p.Binding, ed, path)
		}
		switch b.Target {
		case domain.ClaudeCode:
			if name != "settings.json" {
				continue
			}
			if e = set([]string{"env", "ANTHROPIC_BASE_URL"}, base); e != nil {
				return p, e
			}
			if o.AuthStyle != nil && b.ExpectedFields["meta:auth_source"] != "custom" {
				style := *o.AuthStyle
				if style != domain.AnthropicAPIKey && style != domain.AnthropicAuthToken {
					return p, errors.New("validation_failed: unknown Claude auth style")
				}
				if o.APIKey == "" {
					return p, errors.New("validation_failed: auth style change requires bound key")
				}
				old := style.OtherEnvKey()
				if live, ok := d.get([]string{"env", old}); ok && (fingerprint(asString(live)) == b.KeyFingerprint || asString(live) == o.APIKey) {
					if e = ed.write([]string{"env", old}, nil, true); e != nil {
						return p, e
					}
				}
				if e = ed.write([]string{"env", style.EnvKey()}, o.APIKey, false); e != nil {
					return p, e
				}
				keys := []string{}
				for _, k := range p.Binding.ManagedEnvKeys {
					if k != old && k != style.EnvKey() {
						keys = append(keys, k)
					}
				}
				p.Binding.ManagedEnvKeys = append(keys, style.EnvKey())
			}
		case domain.Codex:
			if name != "config.toml" {
				continue
			}
			if e = set([]string{"model_providers", id, "base_url"}, base); e != nil {
				return p, e
			}
			if o.Capabilities != nil && b.ExpectedFields["meta:capability_source"] != "custom" {
				caps := *o.Capabilities
				search := "disabled"
				if caps["codex-search"] {
					search = "cached"
				}
				display := o.Site.Name
				if caps["codex-compact"] {
					display = "OpenAI"
				}
				for _, field := range []struct {
					path  []string
					value any
				}{{[]string{"web_search"}, search}, {[]string{"tools", "view_image"}, caps["codex-vision"]}, {[]string{"features", "image_generation"}, caps["codex-imagegen"]}, {[]string{"model_providers", id, "name"}, display}} {
					if e = set(field.path, field.value); e != nil {
						return p, e
					}
				}
			}
		case domain.Pi, domain.Prime:
			if name != "models.json" {
				continue
			}
			if o.ProtocolChanged {
				if modelList, ok := d.get([]string{"providers", id, "models"}); ok {
					if models, ok := modelList.([]any); ok {
						for _, model := range models {
							if obj, ok := model.(map[string]any); ok {
								if compat, ok := obj["compat"].(map[string]any); ok && compat["forceAdaptiveThinking"] == true && o.Site.Protocol != domain.Anthropic {
									return p, errors.New("reapply_required: adaptive model options require an explicit protocol preview")
								}
								if _, ok := obj["thinkingLevelMap"]; ok && o.Site.Protocol == domain.Anthropic && b.Target == domain.Prime {
									return p, errors.New("reapply_required: extended model options are incompatible with Prime Anthropic")
								}
							}
						}
					}
				}
			}
			api := "anthropic-messages"
			if o.Site.Protocol == domain.OpenAICompatible {
				api = "openai-completions"
			}
			if e = set([]string{"providers", id, "api"}, api); e != nil {
				return p, e
			}
			if e = set([]string{"providers", id, "baseUrl"}, base); e != nil {
				return p, e
			}
		}
		c.After = d.bytes()
	}
	carryBaselines(&b, &p)
	return p, validatePlan(p)
}
