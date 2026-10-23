package adapters

import (
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"path/filepath"
	"strings"
)

func EncodeAuthLiteral(s string) string {
	s = strings.ReplaceAll(s, "$", "$$")
	if strings.HasPrefix(s, "!") {
		s = "$" + s
	}
	return s
}
func DecodeAuthLiteral(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && i+1 < len(s) && (s[i+1] == '$' || s[i+1] == '!') {
			i++
		}
		out.WriteByte(s[i])
	}
	return out.String()
}
func buildAgent(r Request, p *Plan) error {
	writeAll, models, thinking := r.Pi.WriteAllModels, r.Pi.CatalogModels, r.Pi.Thinking
	if r.Target == domain.Prime {
		writeAll, models, thinking = r.Prime.WriteAllModels, r.Prime.CatalogModels, r.Prime.Thinking
	}
	if r.Site.Protocol != domain.OpenAICompatible && r.Site.Protocol != domain.Anthropic {
		return errors.New("validation_failed: unsupported site protocol")
	}
	if err := domain.ValidateThinking(thinking, r.Target, r.Site.Protocol, r.ModelID); err != nil {
		return err
	}
	id := domain.ProviderIDForSite(r.Site.ID)
	p.Binding.ProviderID = &id
	codex := r.Site.Protocol == domain.OpenAICompatible
	base, err := normalizedURL(r.Site.BaseURL, codex)
	if err != nil {
		return err
	}
	api := "anthropic-messages"
	if codex {
		api = "openai-completions"
	}
	items, err := agentModels(r, models, writeAll, thinking)
	if err != nil {
		return err
	}
	provider := map[string]any{"name": r.Site.Name, "baseUrl": base, "api": api, "models": items}
	if r.Target == domain.Prime {
		provider["apiKey"] = "from-auth-json"
	}
	m, err := openEditor(p, filepath.Join(r.Home, "models.json"), "models")
	if err != nil {
		return err
	}
	if err = pruneManaged(m, []string{"providers"}, id); err != nil {
		return err
	}
	if err = m.write([]string{"providers", id}, provider, false); err != nil {
		return err
	}
	m.finish()
	a, err := openEditor(p, filepath.Join(r.Home, "auth.json"), "strict")
	if err != nil {
		return err
	}
	if err = pruneManaged(a, nil, id); err != nil {
		return err
	}
	if err = a.write([]string{id}, map[string]any{"type": "api_key", "key": EncodeAuthLiteral(r.APIKey)}, false); err != nil {
		return err
	}
	a.finish()
	s, err := openEditor(p, filepath.Join(r.Home, "settings.json"), "strict")
	if err != nil {
		return err
	}
	for _, f := range []struct {
		k string
		v any
	}{{"defaultProvider", id}, {"defaultModel", r.ModelID}} {
		if err = s.write([]string{f.k}, f.v, false); err != nil {
			return err
		}
	}
	if thinking.DefaultLevel != nil {
		if err = s.write([]string{"defaultThinkingLevel"}, *thinking.DefaultLevel, false); err != nil {
			return err
		}
		if r.Target == domain.Pi {
			key := id + "/" + r.ModelID
			if _, ok := s.doc.get([]string{"modelThinkingLevels", key}); ok {
				if err = s.write([]string{"modelThinkingLevels", key}, nil, true); err != nil {
					return err
				}
			}
		}
	}
	s.finish()
	carryBaselines(r.BeforeBinding, p)
	return nil
}
func pruneManaged(e *editor, parent []string, keep string) error {
	v, exists := e.doc.get(parent)
	if !exists {
		return nil
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return errors.New("invalid_config: providers must be object")
	}
	for k := range obj {
		if strings.HasPrefix(k, "xiaobai_") && k != keep {
			path := append(append([]string(nil), parent...), k)
			if err := e.write(path, nil, true); err != nil {
				return err
			}
			e.plan.Binding.ExpectedFields["baseline:"+e.name+":"+joinPath(path)] = missing
		}
	}
	return nil
}
func agentModels(r Request, catalog []domain.CatalogModel, all bool, thinking domain.ThinkingWrite) ([]any, error) {
	source := []domain.CatalogModel{}
	if all {
		source = append(source, catalog...)
	}
	source = append(source, domain.CatalogModel{ModelID: r.ModelID, DisplayName: r.ModelID})
	seen := map[string]bool{}
	items := []any{}
	for _, model := range source {
		id := strings.TrimSpace(model.ModelID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		item := map[string]any{"id": id, "input": []string{"text", "image"}}
		if name := strings.TrimSpace(model.DisplayName); name != "" && name != id {
			item["name"] = name
		}
		cfg := thinking.Models[id]
		if cfg.Reasoning {
			item["reasoning"] = true
			extended := map[string]string{}
			for k, v := range cfg.ThinkingLevelMap {
				if k == "max" || k == "xhigh" {
					extended[k] = v
				}
			}
			if thinking.Extended.Xhigh != nil {
				if _, ok := extended["xhigh"]; !ok {
					extended["xhigh"] = *thinking.Extended.Xhigh
				}
			}
			if thinking.Extended.Max != nil {
				if _, ok := extended["max"]; !ok {
					extended["max"] = *thinking.Extended.Max
				}
			}
			allowed := r.Site.Protocol == domain.OpenAICompatible || (r.Target == domain.Pi && cfg.ForceAdaptiveThinking)
			if len(extended) > 0 && allowed {
				item["thinkingLevelMap"] = extended
			}
			if r.Target == domain.Pi && r.Site.Protocol == domain.Anthropic && cfg.ForceAdaptiveThinking {
				item["compat"] = map[string]any{"forceAdaptiveThinking": true}
			}
		}
		items = append(items, item)
	}

	return items, nil
}
func carryBaselines(before *domain.TargetBinding, p *Plan) {
	if before == nil {
		return
	}
	for k := range p.Binding.ExpectedFields {
		if strings.HasPrefix(k, "baseline:") {
			if v, ok := before.ExpectedFields[k]; ok {
				p.Binding.ExpectedFields[k] = v
			}
		}
	}
}
