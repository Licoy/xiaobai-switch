package adapters

import (
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/envinject"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"path/filepath"
)

func buildCodex(r Request, p *Plan) error {
	if !filepath.IsAbs(r.EnvPath) {
		return errors.New("validation_failed: absolute Codex env path required")
	}
	e, err := openEditor(p, filepath.Join(r.Home, "config.toml"), "toml")
	if err != nil {
		return err
	}
	id := domain.ProviderIDForSite(r.Site.ID)
	key := domain.EnvKeyForSite(r.Site.ID)
	p.Binding.ProviderID = &id
	p.Binding.ExpectedFields["meta:capability_source"] = string(r.Codex.CapabilitySource)
	p.Binding.ManagedEnvKeys = []string{key}
	base, err := normalizedURL(r.Site.BaseURL, true)
	if err != nil {
		return err
	}
	name := r.Site.Name
	if r.Codex.RemoteCompaction {
		name = "OpenAI"
	}
	search := "disabled"
	if r.Codex.WebSearch {
		search = "cached"
	}
	fields := []struct {
		path []string
		v    any
	}{{[]string{"model"}, r.ModelID}, {[]string{"model_provider"}, id}, {[]string{"model_providers", id, "name"}, name}, {[]string{"model_providers", id, "base_url"}, base}, {[]string{"model_providers", id, "env_key"}, key}, {[]string{"model_providers", id, "wire_api"}, "responses"}, {[]string{"web_search"}, search}, {[]string{"tools", "view_image"}, r.Codex.ImageUnderstanding}, {[]string{"features", "image_generation"}, r.Codex.ImageGeneration}}
	if r.Codex.ReasoningEffort != nil {
		fields = append(fields, struct {
			path []string
			v    any
		}{[]string{"model_reasoning_effort"}, string(*r.Codex.ReasoningEffort)})
	}
	if r.Codex.WriteAllModels {
		path := filepath.Join(r.Home, "xiaobai-model-catalog.json")
		catalog := r.Codex.CatalogModels
		if len(catalog) == 0 {
			catalog = []domain.CatalogModel{{ModelID: r.ModelID, DisplayName: r.ModelID}}
		}
		content, err := catalogJSON(catalog, r.Site.Name, r.Codex.ImageUnderstanding)
		if err != nil {
			return err
		}
		p.Binding.ExpectedFields["meta:catalog_hash"] = fingerprint(string(content))
		c, err := fileops.Snapshot(path, content)
		if err != nil {
			return err
		}
		p.Changes = append(p.Changes, c)
		p.Binding.ManagedPaths = append(p.Binding.ManagedPaths, path)
		fields = append(fields, struct {
			path []string
			v    any
		}{[]string{"model_catalog_json"}, path})
	}
	for _, f := range fields {
		if err = e.write(f.path, f.v, false); err != nil {
			return err
		}
	}
	e.finish()
	mode := r.EnvMode
	if mode == "" {
		mode = "file_only"
	}
	injection, err := envinject.Build(envinject.Request{Mode: mode, Platform: r.Platform, Home: r.ShellHome, EnvPath: r.EnvPath, Key: key, Value: r.APIKey})
	if err != nil {
		return err
	}
	p.Injection = &injection
	p.Binding.ExpectedFields["meta:env_mode"] = injection.Mode
	for _, c := range injection.Files {
		p.Changes = append(p.Changes, c)
		p.Binding.ManagedPaths = append(p.Binding.ManagedPaths, c.Path)
	}
	carryBaselines(r.BeforeBinding, p)
	return nil
}
func catalogJSON(models []domain.CatalogModel, site string, vision bool) ([]byte, error) {
	items := []any{}
	modalities := []string{"text"}
	if vision {
		modalities = append(modalities, "image")
	}
	for i, m := range models {
		levels := []any{}
		for _, v := range []struct{ effort, description string }{{"low", "Fast responses with lighter reasoning"}, {"medium", "Balances speed and reasoning depth"}, {"high", "Greater reasoning depth for complex problems"}, {"xhigh", "Extra high reasoning depth"}} {
			levels = append(levels, map[string]any{"effort": v.effort, "description": v.description})
		}
		items = append(items, map[string]any{"slug": m.ModelID, "display_name": m.DisplayName, "description": "From XiaoBaiSwitch · " + site, "context_window": 128000, "max_context_window": 128000, "visibility": "list", "supported_in_api": true, "input_modalities": modalities, "priority": i + 1, "default_reasoning_level": "medium", "supported_reasoning_levels": levels})
	}
	b, e := json.MarshalIndent(map[string]any{"models": items}, "", "  ")
	return append(b, '\n'), e
}
