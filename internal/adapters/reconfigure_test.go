package adapters

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReconfigureKeepsModelsCatalogThinkingAndCustomCapabilities(t *testing.T) {
	for _, target := range []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime} {
		t.Run(string(target), func(t *testing.T) {
			r := request(t, target)
			r.Codex.WriteAllModels = true
			r.Codex.CapabilitySource = domain.CapabilityCustom
			r.Codex.WebSearch = true
			p, e := Build(r)
			if e != nil {
				t.Fatal(e)
			}
			apply(t, p)
			site := r.Site
			site.BaseURL = "https://new-route.test"
			caps := domain.SiteCapabilities{"codex-search": false}
			changed, e := Reconfigure(p.Binding, ConnectionOptions{Site: site, Capabilities: &caps})
			if e != nil {
				t.Fatal(e)
			}
			apply(t, changed)
			for _, c := range changed.Changes {
				if filepath.Base(c.Path) == "codex.env" || filepath.Base(c.Path) == "auth.json" {
					continue
				}
				if !strings.Contains(string(c.After), r.ModelID) {
					t.Fatal("connection change discarded model configuration")
				}
			}
			if target == domain.Codex {
				raw, _ := os.ReadFile(filepath.Join(r.Home, "config.toml"))
				if !strings.Contains(string(raw), `web_search = "cached"`) || !strings.Contains(string(raw), "model_catalog_json") {
					t.Fatal("site capabilities overrode custom target capabilities or catalog")
				}
			}
		})
	}
}
func TestInvalidTargetEffortRejectedAndClaudeLegacyMaxNormalized(t *testing.T) {
	r := request(t, domain.ClaudeCode)
	invalid := domain.ClaudeEffortLevel("invalid")
	r.Claude.EffortLevel = &invalid
	if _, e := Build(r); e == nil {
		t.Fatal("invalid Claude effort accepted")
	}
	legacy := domain.ClaudeEffortLevel("max")
	r.Claude.EffortLevel = &legacy
	p, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(p.Changes[0].After), `"effortLevel": "xhigh"`) {
		t.Fatal("legacy max was not normalized")
	}
	r = request(t, domain.Codex)
	wrong := domain.CodexReasoningEffort("none")
	r.Codex.ReasoningEffort = &wrong
	if _, e = Build(r); e == nil {
		t.Fatal("invalid Codex effort accepted")
	}
}
