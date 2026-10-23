package adapters

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/secure"
	"strconv"
)

// Summarize returns only selected configuration facts and key prefixes.
func Summarize(target domain.TargetKind, files map[string][]byte) (map[string]string, error) {
	out := map[string]string{}
	if target == domain.Codex {
		if raw, ok := files["config.toml"]; ok {
			d, e := parseTOML(raw)
			if e != nil {
				return nil, e
			}
			for _, key := range []string{"model", "model_provider", "model_reasoning_effort", "web_search"} {
				v, ok := d.get([]string{key})
				if ok {
					out[key] = asString(v)
				}
			}
			provider := out["model_provider"]
			for _, key := range []string{"name", "base_url", "env_key", "wire_api"} {
				v, ok := d.get([]string{"model_providers", provider, key})
				if ok {
					out[key] = asString(v)
				}
			}
		}
		return out, nil
	}
	if target == domain.ClaudeCode {
		raw, ok := files["settings.json"]
		if !ok {
			return out, nil
		}
		d, e := parseJSON(raw, "strict")
		if e != nil {
			return nil, e
		}
		for _, key := range []string{"model", "effortLevel"} {
			if v, ok := d.get([]string{key}); ok {
				out[key] = asString(v)
			}
		}
		if env, ok := d.get([]string{"env"}); ok {
			if values, ok := env.(map[string]any); ok {
				for _, key := range []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL", "ANTHROPIC_DEFAULT_FABLE_MODEL"} {
					if v, ok := values[key]; ok {
						value := asString(v)
						if isSecret([]string{key}) {
							value = secure.KeyPrefix(value)
						}
						out[key] = value
					}
				}
			}
		}
		return out, nil
	}
	if !domain.IsThinkingTarget(target) {
		return nil, domain.NewError("validation_failed", "unknown target")
	}
	provider := ""
	if raw, ok := files["settings.json"]; ok {
		d, e := parseJSON(raw, "strict")
		if e != nil {
			return nil, e
		}
		for _, key := range []string{"defaultProvider", "defaultModel", "defaultThinkingLevel"} {
			if v, ok := d.get([]string{key}); ok {
				out[key] = asString(v)
			}
		}
		provider = out["defaultProvider"]
	}
	if raw, ok := files["models.json"]; ok {
		d, e := parseJSON(raw, "models")
		if e != nil {
			return nil, e
		}
		if provider == "" {
			if values, ok := d.get([]string{"providers"}); ok {
				if obj, ok := values.(map[string]any); ok {
					for key := range obj {
						if len(key) >= 8 && key[:8] == "xiaobai_" {
							provider = key
							break
						}
					}
				}
			}
		}
		for _, key := range []string{"name", "baseUrl", "api"} {
			if v, ok := d.get([]string{"providers", provider, key}); ok {
				out[key] = asString(v)
			}
		}
		if v, ok := d.get([]string{"providers", provider, "models"}); ok {
			if list, ok := v.([]any); ok {
				out["models"] = strconv.Itoa(len(list))
			}
		}
	}
	if raw, ok := files["auth.json"]; ok {
		d, e := parseJSON(raw, "strict")
		if e != nil {
			return nil, e
		}
		if v, ok := d.get([]string{provider, "key"}); ok {
			out["apiKey"] = secure.KeyPrefix(DecodeAuthLiteral(asString(v)))
		}
	}
	return out, nil
}

// ReferencedModels mirrors key-switch compatibility rules, including Claude's live aliases.
func ReferencedModels(b domain.TargetBinding) ([]string, error) {
	if b.Target != domain.ClaudeCode {
		return []string{b.ModelID}, nil
	}
	docs, e := bindingDocs(b)
	if e != nil {
		return nil, e
	}
	d := docs["settings.json"]
	if d == nil {
		return nil, domain.NewError("invalid_binding", "Claude settings path missing")
	}
	out := []string{}
	model := ""
	for _, path := range [][]string{{"env", "ANTHROPIC_MODEL"}, {"model"}} {
		if v, ok := d.doc.get(path); ok && asString(v) != "" {
			model = asString(v)
			break
		}
	}
	if model == "" {
		model = b.ModelID
	}
	out = append(out, model)
	for _, key := range []string{"ANTHROPIC_DEFAULT_FABLE_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
		if v, ok := d.doc.get([]string{"env", key}); ok && asString(v) != "" {
			out = append(out, asString(v))
		} else if value, ok := b.ExpectedFields[key]; ok && value != "" {
			out = append(out, value)
		}
	}
	return out, nil
}
