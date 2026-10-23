package adapters

import (
	"bytes"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func request(t *testing.T, target domain.TargetKind) Request {
	t.Helper()
	root := t.TempDir()
	return Request{Target: target, Site: domain.SiteRow{ID: "12345678-abcd-ef00", Name: "Test", BaseURL: "https://example.test/v1/messages", Protocol: domain.OpenAICompatible, ClaudeAuthKeyStyle: domain.AnthropicAuthToken}, APIKey: "fixture-secret-'$!", ModelID: "test-model", Home: root, EnvPath: filepath.Join(root, "codex.env")}
}
func apply(t *testing.T, p Plan) {
	t.Helper()
	root := t.TempDir()
	if _, e := Execute(p, ExecuteOptions{BackupDir: filepath.Join(root, "backup"), LocksDir: filepath.Join(root, "locks")}); e != nil {
		t.Fatal(e)
	}
}
func fixture(t *testing.T, home, name, content string) {
	t.Helper()
	if e := os.WriteFile(filepath.Join(home, name), []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
}
func jsonFile(t *testing.T, path string) map[string]any {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var out map[string]any
	if e = json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestFourTargetsApplyStateRewriteRevert(t *testing.T) {
	for _, target := range []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime} {
		t.Run(string(target), func(t *testing.T) {
			r := request(t, target)
			p, e := Build(r)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = os.Stat(filepath.Join(r.Home, "settings.json")); !os.IsNotExist(e) {
				t.Fatal("preview touched live files")
			}
			raw, _ := json.Marshal(p)
			if bytes.Contains(raw, []byte(r.APIKey)) {
				t.Fatal("preview leaks auth key")
			}
			apply(t, p)
			state, e := ReadState(&p.Binding)
			if e != nil || state.Status != domain.Applied {
				t.Fatalf("state %v %v", state, e)
			}
			raw, _ = json.Marshal(state)
			if bytes.Contains(raw, []byte(r.APIKey)) {
				t.Fatal("state leaks auth key")
			}
			rewrite, e := Rewrite(p.Binding, "https://route.test/anthropic", "next-secret")
			if e != nil {
				t.Fatal(e)
			}
			apply(t, rewrite)
			state, e = ReadState(&rewrite.Binding)
			if e != nil || state.Status != domain.Applied {
				t.Fatalf("rewrite state %v %v", state, e)
			}
			undo, e := Revert(rewrite.Binding)
			if e != nil {
				t.Fatal(e)
			}
			apply(t, undo)
			state, e = ReadState(&rewrite.Binding)
			if e != nil || state.Status != domain.Stale {
				t.Fatalf("undo state %v %v", state, e)
			}
		})
	}
}
func TestClaudePreservesEnvAndUsesTopLevelModel(t *testing.T) {
	r := request(t, domain.ClaudeCode)
	fixture(t, r.Home, "settings.json", `{"permissions":{"allow":["Read"]},"env":{"KEEP":"user","ANTHROPIC_MODEL":"old","CLAUDE_CODE_EFFORT_LEVEL":"max","ANTHROPIC_API_KEY":"user-different-key"}}`)
	opus, haiku, effort := "opus", "haiku", domain.ClaudeEffortLevel("high")
	r.Claude = domain.ClaudeApplyOptions{OpusModelID: &opus, HaikuModelID: &haiku, EffortLevel: &effort, Use1mContext: true}
	p, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	apply(t, p)
	root := jsonFile(t, filepath.Join(r.Home, "settings.json"))
	env := root["env"].(map[string]any)
	if root["model"] != "test-model[1m]" || root["effortLevel"] != "high" || env["ANTHROPIC_DEFAULT_OPUS_MODEL"] != "opus[1m]" || env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] != "haiku" || env["KEEP"] != "user" || env["ANTHROPIC_API_KEY"] != "user-different-key" {
		t.Fatalf("unexpected config %v", root)
	}
	if _, ok := env["ANTHROPIC_MODEL"]; ok {
		t.Fatal("model override persisted")
	}
	if env["ANTHROPIC_BASE_URL"] != "https://example.test/v1" {
		t.Fatal("v1 removed")
	}
}
func TestCodexTOMLCommentsMultilineArrayInlineTables(t *testing.T) {
	for _, input := range []string{"# top\nmodel = \"old\" # model comment\ncustom = [\n \"one\", # item\n \"two\",\n]\n[model_providers.other]\nbase_url = \"https://other\" # other comment\n", "# inline\nmodel_providers = { other = { name = \"keep\" } }\ntools = { custom = true }\nfeatures = {}\n"} {
		t.Run(input[:8], func(t *testing.T) {
			r := request(t, domain.Codex)
			fixture(t, r.Home, "config.toml", input)
			r.Codex.WriteAllModels = true
			r.Codex.ImageUnderstanding = true
			r.Codex.CatalogModels = []domain.CatalogModel{{ModelID: "one", DisplayName: "One"}, {ModelID: "two", DisplayName: "Two"}}
			p, e := Build(r)
			if e != nil {
				t.Fatal(e)
			}
			apply(t, p)
			raw, e := os.ReadFile(filepath.Join(r.Home, "config.toml"))
			if e != nil {
				t.Fatal(e)
			}
			for _, want := range []string{"# ", "other", "responses", "xiaobai_12345678abcd"} {
				if !strings.Contains(string(raw), want) {
					t.Fatalf("lost %s", want)
				}
			}
			if strings.Contains(input, "# item") && !strings.Contains(string(raw), "# item") {
				t.Fatal("array comment lost")
			}
			catalog := jsonFile(t, filepath.Join(r.Home, "xiaobai-model-catalog.json"))
			if len(catalog["models"].([]any)) != 2 {
				t.Fatal("catalog incomplete")
			}
		})
	}
}
func TestAgentPreservesCommentsOAuthUnknownFields(t *testing.T) {
	for _, target := range []domain.TargetKind{domain.Pi, domain.Prime} {
		t.Run(string(target), func(t *testing.T) {
			r := request(t, target)
			fixture(t, r.Home, "models.json", "{\n// provider note\n\"providers\":{\"other\":{\"models\":[],\"unknown\":true},\"xiaobai_old\":{}},\n\"custom\":1,\n}\n")
			fixture(t, r.Home, "auth.json", `{"other":{"type":"oauth","refresh":"keep"},"extra":true}`)
			fixture(t, r.Home, "settings.json", `{"defaultProvider":"official","defaultModel":"original","custom":true}`)
			p, e := Build(r)
			if e != nil {
				t.Fatal(e)
			}
			apply(t, p)
			raw, _ := os.ReadFile(filepath.Join(r.Home, "models.json"))
			if !strings.Contains(string(raw), "// provider note") || !strings.Contains(string(raw), "unknown") || strings.Contains(string(raw), "xiaobai_old") {
				t.Fatal("models merge damaged source")
			}
			a := jsonFile(t, filepath.Join(r.Home, "auth.json"))
			if a["other"].(map[string]any)["refresh"] != "keep" {
				t.Fatal("OAuth lost")
			}
			id := *p.Binding.ProviderID
			d, _ := parseJSON(raw, "models")
			key, has := d.get([]string{"providers", id, "apiKey"})
			if target == domain.Prime && (!has || key != "from-auth-json") {
				t.Fatal("Prime auth pointer missing")
			}
			if target == domain.Pi && has {
				t.Fatal("Pi unexpectedly got Prime apiKey")
			}
			undo, e := Revert(p.Binding)
			if e != nil {
				t.Fatal(e)
			}
			apply(t, undo)
			s := jsonFile(t, filepath.Join(r.Home, "settings.json"))
			if s["defaultProvider"] != "official" || s["defaultModel"] != "original" {
				t.Fatal("baseline not restored")
			}
			a = jsonFile(t, filepath.Join(r.Home, "auth.json"))
			if _, exists := a[id]; exists {
				t.Fatal("managed auth not removed")
			}
		})
	}
}
func TestUnsupportedJSONCSyntaxFailsWithoutMutation(t *testing.T) {
	for _, body := range []string{`{/* no block */"providers":{}}`, `{providers:{}}`, `{'providers':{}}`, `{"providers":{} "other":1}`, `{"providers":{},"n":0x12}`, `{"providers":{},"n":+2}`} {
		t.Run(body, func(t *testing.T) {
			r := request(t, domain.Pi)
			fixture(t, r.Home, "models.json", body)
			if _, e := Build(r); e == nil {
				t.Fatal("unsupported syntax accepted")
			}
			raw, _ := os.ReadFile(filepath.Join(r.Home, "models.json"))
			if string(raw) != body {
				t.Fatal("failed preview changed config")
			}
		})
	}
}
func TestUserEditsSurviveRevertAndRepeatedApplyKeepsBaseline(t *testing.T) {
	r := request(t, domain.ClaudeCode)
	fixture(t, r.Home, "settings.json", `{"model":"original","env":{"KEEP":"yes"}}`)
	p, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	apply(t, p)
	r.ModelID = "second"
	r.BeforeBinding = &p.Binding
	p2, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	apply(t, p2)
	undo, e := Revert(p2.Binding)
	if e != nil {
		t.Fatal(e)
	}
	apply(t, undo)
	root := jsonFile(t, filepath.Join(r.Home, "settings.json"))
	if root["model"] != "original" {
		t.Fatal("second apply overwrote original baseline")
	}
	p, e = Build(r)
	if e != nil {
		t.Fatal(e)
	}
	apply(t, p)
	root = jsonFile(t, filepath.Join(r.Home, "settings.json"))
	root["model"] = "user-after"
	raw, _ := json.Marshal(root)
	fixture(t, r.Home, "settings.json", string(raw))
	undo, e = Revert(p.Binding)
	if e != nil {
		t.Fatal(e)
	}
	apply(t, undo)
	root = jsonFile(t, filepath.Join(r.Home, "settings.json"))
	if root["model"] != "user-after" {
		t.Fatal("user later edit destroyed")
	}
}
