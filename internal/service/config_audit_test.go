package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/licoy/xiaobai-switch/internal/domain"
)

func TestAgentOfficialRestoreThinkingBaseline(t *testing.T) {
	for _, target := range []domain.TargetKind{domain.Pi, domain.Prime} {
		for _, baseline := range []string{"", "medium", "user-after"} {
			t.Run(string(target)+"/"+baseline, func(t *testing.T) {
				s, st, site := fixtureService(t)
				settings, err := st.GetSettings()
				if err != nil {
					t.Fatal(err)
				}
				home, err := s.targetHome(target, settings)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.MkdirAll(home, 0700); err != nil {
					t.Fatal(err)
				}
				initial := map[string]any{"theme": "dark", "defaultProvider": "official", "defaultModel": "official-model"}
				if baseline == "medium" {
					initial["defaultThinkingLevel"] = baseline
				}
				writeAuditJSON(t, filepath.Join(home, "settings.json"), initial)
				request := applyRequest(site, target)
				level := "high"
				thinking := domain.ThinkingWrite{DefaultLevel: &level, Models: map[string]domain.ModelThinkingConfig{"fixture-model": {Reasoning: true}}}
				if target == domain.Pi {
					request.Pi.Thinking = thinking
				} else {
					request.Prime.Thinking = thinking
				}
				preview, err := s.PrepareApply(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = s.Apply(context.Background(), preview.ID); err != nil {
					t.Fatal(err)
				}
				if baseline == "user-after" {
					current := readAuditJSON(t, filepath.Join(home, "settings.json"))
					current["defaultThinkingLevel"] = baseline
					writeAuditJSON(t, filepath.Join(home, "settings.json"), current)
				}
				if err = s.RestoreOfficial(context.Background(), target); err != nil {
					t.Fatal(err)
				}
				current := readAuditJSON(t, filepath.Join(home, "settings.json"))
				if baseline == "" {
					if _, exists := current["defaultThinkingLevel"]; exists {
						t.Fatal("official restore retained managed thinking level without baseline")
					}
				} else if current["defaultThinkingLevel"] != baseline {
					t.Fatal("official restore changed baseline or later user thinking level")
				}
				if current["theme"] != "dark" || current["defaultProvider"] != "official" || current["defaultModel"] != "official-model" {
					t.Fatal("official restore lost original defaults or unrelated setting")
				}
			})
		}
	}
}

func writeAuditJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
func readAuditJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFourTargetsOfficialRestoreWithoutBinding(t *testing.T) {
	for _, target := range targets {
		t.Run(string(target), func(t *testing.T) {
			s, st, site := fixtureService(t)
			applyFixture(t, s, site, target)
			if err := st.ClearApplication(target, ""); err != nil {
				t.Fatal(err)
			}
			if err := s.RestoreOfficial(context.Background(), target); err != nil {
				t.Fatal(err)
			}
			states, err := s.Status(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for _, state := range states {
				if state.Target == target && state.Status != domain.NotApplied {
					t.Fatalf("official restore left state %s", state.Status)
				}
			}
		})
	}
}

func TestFourTargetsReformattedConfigRewriteReconfigureRevert(t *testing.T) {
	for _, target := range targets {
		t.Run(string(target), func(t *testing.T) {
			s, st, site := fixtureService(t)
			settings, err := st.GetSettings()
			if err != nil {
				t.Fatal(err)
			}
			home, err := s.targetHome(target, settings)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.MkdirAll(home, 0700); err != nil {
				t.Fatal(err)
			}
			if target == domain.Codex {
				body := "model = \"official-model\"\n[model_providers.other]\nname = \"keep\" # provider note\n[mcp_servers.keep]\ncommand = \"fixture\"\n"
				if err = os.WriteFile(filepath.Join(home, "config.toml"), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				initial := map[string]any{"theme": "dark"}
				if target == domain.ClaudeCode {
					initial["model"] = "official-model"
					initial["env"] = map[string]any{"KEEP": "user"}
				} else {
					initial["defaultProvider"], initial["defaultModel"] = "official", "official-model"
					if err = os.WriteFile(filepath.Join(home, "models.json"), []byte("{\n// provider note\n\"providers\":{\"other\":{\"models\":[],\"unknown\":true}},\n\"custom\":1,\n}\n"), 0600); err != nil {
						t.Fatal(err)
					}
					writeAuditJSON(t, filepath.Join(home, "auth.json"), map[string]any{"official": map[string]any{"type": "oauth", "refresh": "fixture-refresh"}})
				}
				writeAuditJSON(t, filepath.Join(home, "settings.json"), initial)
			}
			applyFixture(t, s, site, target)
			route := "https://route.example.test"
			if _, err = s.UpdateSite(context.Background(), site.ID, domain.UpdateSiteInput{BaseURL: &route}); err != nil {
				t.Fatal(err)
			}
			caps := domain.SiteCapabilities{"codex-search": true}
			if _, err = s.UpdateSite(context.Background(), site.ID, domain.UpdateSiteInput{Capabilities: &caps}); err != nil {
				t.Fatal(err)
			}
			status, err := s.bindingStatus(target)
			if err != nil || status != domain.Applied {
				t.Fatalf("rewrite/reconfigure status %s: %v", status, err)
			}
			if err = s.Revert(context.Background(), target); err != nil {
				t.Fatal(err)
			}
			if target == domain.Codex {
				raw, err := os.ReadFile(filepath.Join(home, "config.toml"))
				if err != nil {
					t.Fatal(err)
				}
				for _, value := range []string{"official-model", "# provider note", "mcp_servers.keep"} {
					if !strings.Contains(string(raw), value) {
						t.Fatal("revert lost baseline or unrelated config")
					}
				}
			} else {
				current := readAuditJSON(t, filepath.Join(home, "settings.json"))
				if current["theme"] != "dark" {
					t.Fatal("revert lost unrelated setting")
				}
				if target == domain.ClaudeCode {
					if current["model"] != "official-model" || current["env"].(map[string]any)["KEEP"] != "user" {
						t.Fatal("revert lost Claude baseline")
					}
				} else {
					if current["defaultProvider"] != "official" || current["defaultModel"] != "official-model" {
						t.Fatal("revert lost agent baseline")
					}
					auth := readAuditJSON(t, filepath.Join(home, "auth.json"))
					if len(auth) != 1 || auth["official"].(map[string]any)["refresh"] != "fixture-refresh" {
						t.Fatal("revert damaged OAuth")
					}
					raw, err := os.ReadFile(filepath.Join(home, "models.json"))
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(raw), "// provider note") || !strings.Contains(string(raw), "unknown") || strings.Contains(string(raw), "xiaobai_") {
						t.Fatal("revert damaged other providers or retained managed provider")
					}
				}
			}
		})
	}
}
