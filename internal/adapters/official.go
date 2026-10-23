package adapters

import (
	"encoding/json"
	"errors"
	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/envinject"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"path/filepath"
	"strings"
)

type RestoreRequest struct {
	Target        domain.TargetKind
	Home, EnvPath string
	Binding       *domain.TargetBinding
}

func RestoreOfficial(r RestoreRequest) (Plan, error) {
	p := Plan{Target: r.Target}
	if r.Binding != nil {
		normalized := normalizeBinding(*r.Binding)
		r.Binding = &normalized
		p.Binding = *r.Binding
		if r.Binding.SiteID != nil {
			p.SiteID = *r.Binding.SiteID
		}
	}
	expected := make(map[string]string, len(p.Binding.ExpectedFields))
	for key, value := range p.Binding.ExpectedFields {
		expected[key] = value
	}
	p.Binding.ExpectedFields = expected
	if !filepath.IsAbs(r.Home) {
		return p, errors.New("validation_failed: absolute home required")
	}
	switch r.Target {
	case domain.ClaudeCode:
		e, err := openEditor(&p, filepath.Join(r.Home, "settings.json"), "strict")
		if err != nil {
			return p, err
		}
		if !e.change.Existed {
			return p, nil
		}
		for _, key := range []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_FABLE_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL", "CLAUDE_CODE_EFFORT_LEVEL", "ANTHROPIC_CUSTOM_HEADERS", "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY"} {
			if err = e.doc.set([]string{"env", key}, nil, true); err != nil {
				return p, err
			}
		}
		for _, key := range []string{"model", "effortLevel", "apiKeyHelper"} {
			if err = e.doc.set([]string{key}, nil, true); err != nil {
				return p, err
			}
		}
		e.change.After = e.doc.bytes()
		appendChange(&p, e.change)
	case domain.Codex:
		if err := officialCodex(r, &p); err != nil {
			return p, err
		}
	case domain.Pi, domain.Prime:
		if err := officialAgent(r, &p); err != nil {
			return p, err
		}
	default:
		return p, errors.New("validation_failed: unknown target")
	}
	return p, nil
}
func officialAgent(r RestoreRequest, p *Plan) error {
	for _, name := range []string{"models.json", "auth.json", "settings.json"} {
		mode := "strict"
		if name == "models.json" {
			mode = "models"
		}
		e, err := openEditor(p, filepath.Join(r.Home, name), mode)
		if err != nil {
			return err
		}
		if !e.change.Existed {
			continue
		}
		switch name {
		case "models.json":
			err = pruneManaged(e, []string{"providers"}, "")
		case "auth.json":
			err = pruneManaged(e, nil, "")
		case "settings.json":
			v, _ := e.doc.get([]string{"defaultProvider"})
			if strings.HasPrefix(asString(v), "xiaobai_") {
				for _, key := range []string{"defaultProvider", "defaultModel"} {
					if err = e.doc.set([]string{key}, nil, true); err != nil {
						return err
					}
				}
				if r.Binding != nil {
					for _, key := range []string{"defaultProvider", "defaultModel", "defaultThinkingLevel"} {
						bindingKey := "settings.json:" + key
						want, ok := r.Binding.ExpectedFields[bindingKey]
						if !ok {
							continue
						}
						live, exists := e.doc.get([]string{key})
						if key == "defaultThinkingLevel" && !expectedEqual(live, exists, want) {
							continue
						}
						baseline := r.Binding.ExpectedFields["baseline:"+bindingKey]
						if baseline != "" && baseline != missing {
							var x any
							if err = json.Unmarshal([]byte(baseline), &x); err != nil {
								return err
							}
							if err = e.doc.set([]string{key}, x, false); err != nil {
								return err
							}
						} else if key == "defaultThinkingLevel" {
							if err = e.doc.set([]string{key}, nil, true); err != nil {
								return err
							}
						}
					}
				}
			}
		}
		if err != nil {
			return err
		}
		e.change.After = e.doc.bytes()
		appendChange(p, e.change)
	}
	return nil
}
func officialCodex(r RestoreRequest, p *Plan) error {
	e, err := openEditor(p, filepath.Join(r.Home, "config.toml"), "toml")
	if err != nil {
		return err
	}
	keys := map[string]bool{}
	t := e.doc.(*tomlDoc)
	var managed []*tomledit.Entry
	t.doc.Scan(func(k parser.Key, entry *tomledit.Entry) bool {
		if len(k) > 1 && k[0] == "model_providers" && strings.HasPrefix(k[1], "xiaobai_") {
			managed = append(managed, entry)
			if len(k) == 3 && k[2] == "env_key" {
				v, _ := t.get([]string(k))
				keys[asString(v)] = true
			}
		}
		return true
	})
	for i := len(managed) - 1; i >= 0; i-- {
		managed[i].Remove()
	}
	if err = t.set([]string{"openai_base_url"}, nil, true); err != nil {
		return err
	}
	active, _ := t.get([]string{"model_provider"})
	if strings.HasPrefix(asString(active), "xiaobai_") {
		for _, key := range []string{"model_provider", "model", "model_reasoning_effort", "model_catalog_json"} {
			if err = t.set([]string{key}, nil, true); err != nil {
				return err
			}
		}
	}
	if r.Binding != nil {
		for _, key := range r.Binding.ManagedEnvKeys {
			keys[key] = true
		}
		for _, path := range [][]string{{"web_search"}, {"tools", "view_image"}, {"features", "image_generation"}} {
			k := "config.toml:" + strings.Join(path, "/")
			want, ok := r.Binding.ExpectedFields[k]
			if !ok {
				continue
			}
			v, exists := t.get(path)
			if expectedEqual(v, exists, want) {
				if err = t.set(path, nil, true); err != nil {
					return err
				}
			}
		}
	}
	if e.change.Existed {
		e.change.After = t.bytes()
		appendChange(p, e.change)
	}
	if r.Binding != nil {
		if err = removeOwnedCatalog(p, *r.Binding); err != nil {
			return err
		}
		if r.Binding.ExpectedFields["meta:env_mode"] == "user_env" && len(r.Binding.ManagedEnvKeys) > 0 {
			p.Injection = &envinject.Plan{Mode: "user_env", UserKey: r.Binding.ManagedEnvKeys[0], UserRemove: true, UserExpectedFingerprint: r.Binding.KeyFingerprint}
		}
	}
	if r.EnvPath != "" {
		c, err := fileops.Snapshot(r.EnvPath, nil)
		if err != nil {
			return err
		}
		if c.Existed {
			c.After = c.Before
			for key := range keys {
				if !envinject.ValidKey(key) {
					continue
				}
				c.After, err = envinject.Upsert(c.After, key, "", true)
				if err != nil {
					return err
				}
			}
			appendChange(p, c)
		}
	}
	return nil
}
