package adapters

import (
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"strings"
)

func normalizeBinding(b domain.TargetBinding) domain.TargetBinding {
	for k := range b.ExpectedFields {
		if strings.Contains(k, ":") {
			return b
		}
	}
	old := b.ExpectedFields
	b.ExpectedFields = map[string]string{"meta:legacy": "true"}
	id := ""
	if b.ProviderID != nil {
		id = *b.ProviderID
	}
	add := func(name string, path []string, value any) {
		raw, _ := json.Marshal(value)
		b.ExpectedFields[name+":"+joinPath(path)] = string(raw)
	}
	baseline := func(name string, path []string, key string) {
		v, ok := old[key]
		if !ok {
			return
		}
		out := missing
		if v != missing {
			raw, _ := json.Marshal(v)
			out = string(raw)
		}
		b.ExpectedFields["baseline:"+name+":"+joinPath(path)] = out
	}
	switch b.Target {
	case domain.ClaudeCode:
		for k, v := range old {
			switch k {
			case "auth_env_key":
			case "model", "effortLevel":
				add("settings.json", []string{k}, v)
			default:
				if strings.HasPrefix(k, "ANTHROPIC_") || k == "CLAUDE_CODE_EFFORT_LEVEL" {
					add("settings.json", []string{"env", k}, v)
				}
			}
		}
		for _, key := range b.ManagedEnvKeys {
			if strings.Contains(key, "API_KEY") || strings.Contains(key, "AUTH_TOKEN") {
				b.ExpectedFields["settings.json:env/"+key] = "rawsha256:" + b.KeyFingerprint
			}
		}
	case domain.Codex:
		if source, ok := old["capability_source"]; ok {
			b.ExpectedFields["meta:capability_source"] = source
		}
		add("config.toml", []string{"model_provider"}, id)
		add("config.toml", []string{"model"}, b.ModelID)
		for k, v := range old {
			var path []string
			var val any = v
			switch k {
			case "base_url", "env_key", "wire_api":
				path = []string{"model_providers", id, k}
			case "provider_display_name":
				path = []string{"model_providers", id, "name"}
			case "model_reasoning_effort", "model_catalog_json", "web_search":
				path = []string{k}
			case "tools_view_image":
				path = []string{"tools", "view_image"}
				val = v == "true"
			case "features_image_generation":
				path = []string{"features", "image_generation"}
				val = v == "true"
			}
			if path != nil {
				add("config.toml", path, val)
			}
		}
	case domain.Pi, domain.Prime:
		add("settings.json", []string{"defaultProvider"}, id)
		add("settings.json", []string{"defaultModel"}, b.ModelID)
		add("auth.json", []string{id, "type"}, "api_key")
		b.ExpectedFields["auth.json:"+id+"/key"] = "authsha256:" + b.KeyFingerprint
		for k, v := range old {
			switch k {
			case "base_url":
				add("models.json", []string{"providers", id, "baseUrl"}, v)
			case "api":
				add("models.json", []string{"providers", id, "api"}, v)
			case domain.WroteThinkingLevel:
				add("settings.json", []string{"defaultThinkingLevel"}, v)
			}
		}
		if key, ok := old[domain.ClearedModelThinkingKey]; ok {
			path := []string{"modelThinkingLevels", key}
			b.ExpectedFields["settings.json:"+joinPath(path)] = missing
			baseline("settings.json", path, domain.BaselineModelThinking)
		}
		prefix := string(b.Target)
		baseline("settings.json", []string{"defaultProvider"}, prefix+"_baseline_default_provider")
		baseline("settings.json", []string{"defaultModel"}, prefix+"_baseline_default_model")
		baseline("settings.json", []string{"defaultThinkingLevel"}, domain.BaselineThinkingLevel)
	}
	return b
}
func joinPath(path []string) string {
	out := make([]string, len(path))
	for i, s := range path {
		out[i] = escapePointer(s)
	}
	return strings.Join(out, "/")
}
func splitPath(path string) []string {
	out := strings.Split(path, "/")
	for i, s := range out {
		out[i] = strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
	}
	return out
}
