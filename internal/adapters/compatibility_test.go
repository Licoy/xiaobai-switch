package adapters

import (
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyBindingsStateAndRevert(t *testing.T) {
	r := request(t, domain.ClaudeCode)
	p, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	apply(t, p)
	legacy := p.Binding
	legacy.ExpectedFields = map[string]string{"ANTHROPIC_BASE_URL": "https://example.test/v1", "model": r.ModelID, "auth_env_key": "ANTHROPIC_AUTH_TOKEN"}
	state, e := ReadState(&legacy)
	if e != nil || state.Status != domain.Applied {
		t.Fatalf("legacy %v %v", state, e)
	}
	undo, e := Revert(legacy)
	if e != nil {
		t.Fatal(e)
	}
	apply(t, undo)
	root := jsonFile(t, filepath.Join(r.Home, "settings.json"))
	if _, exists := root["model"]; exists {
		t.Fatal("legacy model not removed")
	}
	env := root["env"].(map[string]any)
	if _, exists := env["ANTHROPIC_AUTH_TOKEN"]; exists {
		t.Fatal("legacy credential not removed")
	}
}
func TestOfficialRestoreKeepsUserConfig(t *testing.T) {
	for _, target := range []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime} {
		t.Run(string(target), func(t *testing.T) {
			r := request(t, target)
			p, e := Build(r)
			if e != nil {
				t.Fatal(e)
			}
			apply(t, p)
			restore, e := RestoreOfficial(RestoreRequest{Target: target, Home: r.Home, EnvPath: r.EnvPath, Binding: &p.Binding})
			if e != nil {
				t.Fatal(e)
			}
			apply(t, restore)
			state, e := ReadState(&p.Binding)
			if e != nil || state.Status != domain.Stale {
				t.Fatalf("official still managed %v %v", state, e)
			}
		})
	}
}
func TestBrokenBindingReturnsErrorInsteadOfPanic(t *testing.T) {
	id := "xiaobai_fixture"
	b := domain.TargetBinding{Target: domain.Pi, ProviderID: &id}
	if _, e := Rewrite(b, "https://example.test", ""); e == nil {
		t.Fatal("missing models accepted")
	}
	s, e := ReadState(&b)
	if e != nil || s.Status != domain.Stale {
		t.Fatalf("empty schema reported applied %v %v", s, e)
	}
}
func TestURLMatchesLegacyFixtures(t *testing.T) {
	for _, row := range []struct{ raw, claude, codex string }{{"https://example.test", "https://example.test", "https://example.test/v1"}, {"https://example.test/v1", "https://example.test/v1", "https://example.test/v1"}, {"https://example.test/v1/messages", "https://example.test/v1", "https://example.test/v1"}, {"https://example.test/anthropic/", "https://example.test/anthropic", "https://example.test/anthropic/v1"}, {"https://example.test/v1?key=ignored#x", "https://example.test/v1", "https://example.test/v1"}} {
		a, e := normalizedURL(row.raw, false)
		b, e2 := normalizedURL(row.raw, true)
		if e != nil || e2 != nil || a != row.claude || b != row.codex {
			t.Fatalf("URL %s -> %s/%s %v %v", row.raw, a, b, e, e2)
		}
	}
}
func TestLegacyAgentThinkingRestoreUsesRustBindingFields(t *testing.T) {
	for _, target := range []domain.TargetKind{domain.Pi, domain.Prime} {
		t.Run(string(target), func(t *testing.T) {
			r := request(t, target)
			id := domain.ProviderIDForSite(r.Site.ID)
			modelKey := id + "/" + r.ModelID
			fixture(t, r.Home, "models.json", `{"providers":{"`+id+`":{"baseUrl":"https://example.test/v1","api":"openai-completions","models":[{"id":"test-model","reasoning":true}]},"other":{"models":[]}}}`)
			fixture(t, r.Home, "auth.json", `{"`+id+`":{"type":"api_key","key":"legacy-key"},"other":{"type":"oauth","refresh":"keep"}}`)
			fixture(t, r.Home, "settings.json", `{"defaultProvider":"`+id+`","defaultModel":"test-model","defaultThinkingLevel":"high","modelThinkingLevels":{"other/model":"low"},"keep":true}`)
			legacy := domain.TargetBinding{Target: target, SiteID: &r.Site.ID, ModelID: r.ModelID, ProviderID: &id, KeyFingerprint: fingerprint("legacy-key"), ManagedPaths: []string{filepath.Join(r.Home, "models.json"), filepath.Join(r.Home, "auth.json"), filepath.Join(r.Home, "settings.json")}, ExpectedFields: map[string]string{"base_url": "https://example.test/v1", "api": "openai-completions", "write_all_models": "false", string(target) + "_baseline_default_provider": "official", string(target) + "_baseline_default_model": "official-model", domain.BaselineThinkingLevel: "medium", domain.WroteThinkingLevel: "high"}}
			if target == domain.Pi {
				legacy.ExpectedFields[domain.ClearedModelThinkingKey] = modelKey
				legacy.ExpectedFields[domain.BaselineModelThinking] = "low"
			}
			state, e := ReadState(&legacy)
			if e != nil || state.Status != domain.Applied {
				t.Fatalf("legacy state %v %v", state, e)
			}
			p, e := Revert(legacy)
			if e != nil {
				t.Fatal(e)
			}
			apply(t, p)
			root := jsonFile(t, filepath.Join(r.Home, "settings.json"))
			if root["defaultProvider"] != "official" || root["defaultModel"] != "official-model" || root["defaultThinkingLevel"] != "medium" {
				t.Fatalf("legacy baseline not restored %v", root)
			}
			if target == domain.Pi {
				levels := root["modelThinkingLevels"].(map[string]any)
				if levels[modelKey] != "low" || levels["other/model"] != "low" {
					t.Fatal("per-model baseline not restored")
				}
			}
			auth := jsonFile(t, filepath.Join(r.Home, "auth.json"))
			if auth["other"].(map[string]any)["refresh"] != "keep" {
				t.Fatal("OAuth lost")
			}
		})
	}
}
func TestThinkingProtocolValidationAndNoModelNameGuessing(t *testing.T) {
	level, maxMapping := "max", "provider-max"
	for _, target := range []domain.TargetKind{domain.Pi, domain.Prime} {
		r := request(t, target)
		r.ModelID = "claude-opus-reasoning"
		p, e := Build(r)
		if e != nil {
			t.Fatal(e)
		}
		raw := string(p.Changes[0].After)
		if strings.Contains(raw, "\"reasoning\"") {
			t.Fatal("guessed thinking from model name")
		}
		w := domain.ThinkingWrite{DefaultLevel: &level, Extended: domain.ThinkingExtendedMap{Max: &maxMapping}, Models: map[string]domain.ModelThinkingConfig{r.ModelID: {Reasoning: true}}}
		if target == domain.Pi {
			r.Pi.Thinking = w
		} else {
			r.Prime.Thinking = w
		}
		if _, e = Build(r); e != nil {
			t.Fatal(e)
		}
		r.Site.Protocol = domain.Anthropic
		if _, e = Build(r); e == nil {
			t.Fatal("unsupported Anthropic max accepted")
		}
		if target == domain.Pi {
			w.Models[r.ModelID] = domain.ModelThinkingConfig{Reasoning: true, ForceAdaptiveThinking: true}
			r.Pi.Thinking = w
			if _, e = Build(r); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestLegacyCodexBindingMatchesRustFieldSet(t *testing.T) {
	r := request(t, domain.Codex)
	id := domain.ProviderIDForSite(r.Site.ID)
	key := domain.EnvKeyForSite(r.Site.ID)
	fixture(t, r.Home, "config.toml", "# user\nmodel = \"test-model\"\nmodel_provider = \""+id+"\"\nmodel_reasoning_effort = \"high\"\nweb_search = \"cached\"\n[tools]\nview_image = true\n[features]\nimage_generation = false\n[model_providers."+id+"]\nname = \"OpenAI\"\nbase_url = \"https://example.test/v1\"\nenv_key = \""+key+"\"\nwire_api = \"responses\"\n[model_providers.other]\nname = \"preserve\"\n")
	fixture(t, r.Home, "codex.env", "export "+key+"=\"legacy-key\"\nexport OTHER=\"keep\"\n")
	legacy := domain.TargetBinding{Target: domain.Codex, SiteID: &r.Site.ID, ProviderID: &id, ModelID: r.ModelID, ManagedPaths: []string{filepath.Join(r.Home, "config.toml"), r.EnvPath}, ManagedEnvKeys: []string{key}, KeyFingerprint: fingerprint("legacy-key"), ExpectedFields: map[string]string{"base_url": "https://example.test/v1", "env_key": key, "wire_api": "responses", "model_reasoning_effort": "high", "provider_display_name": "OpenAI", "remote_compaction": "on", "web_search": "cached", "tools_view_image": "true", "features_image_generation": "false", "capability_source": "custom"}}
	s, e := ReadState(&legacy)
	if e != nil || s.Status != domain.Applied {
		t.Fatalf("legacy Codex %v %v", s, e)
	}
	p, e := Revert(legacy)
	if e != nil {
		t.Fatal(e)
	}
	apply(t, p)
	raw, e := os.ReadFile(filepath.Join(r.Home, "config.toml"))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(raw), id) || !strings.Contains(string(raw), "preserve") {
		t.Fatal("managed provider remained or user provider lost")
	}
}
func TestRewriteDoesNotAdoptExternalUserChanges(t *testing.T) {
	for _, target := range []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime} {
		t.Run(string(target), func(t *testing.T) {
			r := request(t, target)
			p, e := Build(r)
			if e != nil {
				t.Fatal(e)
			}
			apply(t, p)
			if target == domain.Codex {
				raw, e := os.ReadFile(filepath.Join(r.Home, "config.toml"))
				if e != nil {
					t.Fatal(e)
				}
				fixture(t, r.Home, "config.toml", strings.Replace(string(raw), `model = "test-model"`, `model = "user-after"`, 1))
			} else if target == domain.ClaudeCode {
				root := jsonFile(t, filepath.Join(r.Home, "settings.json"))
				root["model"] = "user-after"
				raw, _ := json.Marshal(root)
				fixture(t, r.Home, "settings.json", string(raw))
			} else {
				raw, e := os.ReadFile(filepath.Join(r.Home, "models.json"))
				if e != nil {
					t.Fatal(e)
				}
				fixture(t, r.Home, "models.json", strings.Replace(string(raw), `"test-model"`, `"user-after"`, 1))
			}
			rewritten, e := Rewrite(p.Binding, "https://route.test", "")
			if e != nil {
				t.Fatal(e)
			}
			apply(t, rewritten)
			s, e := ReadState(&rewritten.Binding)
			if e != nil || s.Status != domain.Stale {
				t.Fatalf("adopted outside edit %v %v", s, e)
			}
			undo, e := Revert(rewritten.Binding)
			if e != nil {
				t.Fatal(e)
			}
			apply(t, undo)
			path := filepath.Join(r.Home, "settings.json")
			if target == domain.Codex {
				path = filepath.Join(r.Home, "config.toml")
			}
			if target == domain.Pi || target == domain.Prime {
				path = filepath.Join(r.Home, "models.json")
			}
			raw, _ := os.ReadFile(path)
			if !strings.Contains(string(raw), "user-after") {
				t.Fatal("rewrite made later revert delete user edit")
			}
		})
	}
}
func TestInspectDetectsOrphanAndCurrentSiteChanges(t *testing.T) {
	for _, target := range []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime} {
		t.Run(string(target), func(t *testing.T) {
			r := request(t, target)
			empty, e := Inspect(StatusRequest{Target: target, Home: r.Home, EnvPath: r.EnvPath})
			if e != nil || empty.Status != domain.NotApplied {
				t.Fatalf("empty %v %v", empty, e)
			}
			p, e := Build(r)
			if e != nil {
				t.Fatal(e)
			}
			apply(t, p)
			orphan, e := Inspect(StatusRequest{Target: target, Home: r.Home, EnvPath: r.EnvPath})
			if e != nil || orphan.Status != domain.Orphan {
				t.Fatalf("orphan %v %v", orphan, e)
			}
			site := r.Site
			site.BaseURL = "https://new-site.test"
			stale, e := Inspect(StatusRequest{Target: target, Home: r.Home, EnvPath: r.EnvPath, Binding: &p.Binding, Site: &site, APIKey: r.APIKey})
			if e != nil || stale.Status != domain.Stale {
				t.Fatalf("site change %v %v", stale, e)
			}
		})
	}
}
