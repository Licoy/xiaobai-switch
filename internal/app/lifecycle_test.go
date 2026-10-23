package app

import (
	"context"
	"encoding/json"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func fixtureSite(t *testing.T, s *Shell) domain.SiteDTO {
	t.Helper()
	s.editSite(nil)
	s.Editor.Name = "Fixture"
	s.Editor.URLs = "https://example.test/v1"
	s.Editor.Key = "fixture-not-a-real-key"
	if err := s.saveSite(); err != nil {
		t.Fatal(err)
	}
	site := *s.site()
	if err := s.Service.SetSelectedModel(context.Background(), site.ID, value(site.ActiveAPIKeyID), "fixture-model"); err != nil {
		t.Fatal(err)
	}
	if err := s.reload(); err != nil {
		t.Fatal(err)
	}
	return *s.site()
}
func TestPreviewCancellationDoesNotWrite(t *testing.T) {
	s := testShell(t)
	site := fixtureSite(t, s)
	d := s.draft(domain.ClaudeCode)
	d.Auth = string(domain.AnthropicAPIKey)
	before, err := s.Store.GetSite(site.ID)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := s.Store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.Service.PrepareApply(context.Background(), s.buildRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Service.CancelPreview(context.Background(), preview.ID); err != nil {
		t.Fatal(err)
	}
	after, err := s.Store.GetSite(site.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterSettings, err := s.Store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(settings, afterSettings) {
		t.Fatal("cancelled preview modified application state")
	}
	for _, path := range []string{filepath.Join(s.Config.Home, ".claude", "settings.json"), filepath.Join(s.Config.Home, ".codex", "config.toml")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("preview wrote target path %s: %v", path, err)
		}
	}
	binding, err := s.Store.GetBinding(domain.ClaudeCode)
	if err != nil || binding != nil {
		t.Fatalf("preview created binding: %v", err)
	}
	if _, err = s.Service.Apply(context.Background(), preview.ID); err == nil {
		t.Fatal("cancelled preview was applied")
	}
}
func TestThinkingAndSiteCapabilitiesRoundTrip(t *testing.T) {
	s := testShell(t)
	site := fixtureSite(t, s)
	enabled := domain.SiteCapabilities{"codex-vision": true, "codex-search": true}
	if _, err := s.Service.UpdateSite(context.Background(), site.ID, domain.UpdateSiteInput{Capabilities: &enabled}); err != nil {
		t.Fatal(err)
	}
	if err := s.reload(); err != nil {
		t.Fatal(err)
	}
	for _, target := range []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime} {
		s.Current = routeForTarget(target)
		tester := ui.NewTester(s.View, 1100, 720)
		if !tester.HasText(s.text("draft")) {
			t.Fatalf("missing draft for %s", target)
		}
	}
	d := s.draft(domain.Pi)
	d.GlobalXhigh = "high"
	d.GlobalMax = "high"
	d.ThinkingModels["fixture-model"] = domain.ModelThinkingConfig{Reasoning: true, ThinkingLevelMap: map[string]string{"max": "high"}}
	if err := s.saveDraft(d); err != nil {
		t.Fatal(err)
	}
	s.hydrateDraft(d, *s.site())
	if d.GlobalXhigh != "high" || d.GlobalMax != "high" || d.ThinkingModels["fixture-model"].ThinkingLevelMap["max"] != "high" {
		t.Fatal("thinking preset lost mappings")
	}
	request := s.buildRequest(d)
	request.Pi.Thinking.Models["fixture-model"].ThinkingLevelMap["max"] = "low"
	if d.ThinkingModels["fixture-model"].ThinkingLevelMap["max"] != "high" {
		t.Fatal("request aliases mutable UI draft")
	}
}
func TestTrayFitsDisplayAndFourCards(t *testing.T) {
	s := testShell(t)
	fixtureSite(t, s)
	h := &host{shell: s}
	tester := ui.NewTester(h.trayView, 420, 680)
	for _, target := range []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime} {
		if s.Drafts[target] == nil {
			t.Fatalf("missing card state for %s", target)
		}
	}
	if !tester.HasText("Claude Code") {
		t.Fatal("tray did not draw first card")
	}
	if err := tester.Click("Codex"); err != nil || h.trayTarget != 1 {
		t.Fatal("tray tab did not switch target")
	}
	for _, area := range []mygo.Rectangle{{X: 0, Y: 25, Width: 1440, Height: 875}, {X: -1920, Y: 0, Width: 1920, Height: 1080}, {X: 0, Y: 0, Width: 800, Height: 480}} {
		b := clampPanelBounds(mygo.Rectangle{X: 9000, Y: -9000, Width: 420, Height: 680}, area)
		if b.Height > area.Height*85/100 || b.X < area.X || b.Y < area.Y || b.X+b.Width > area.X+area.Width || b.Y+b.Height > area.Y+area.Height {
			t.Fatalf("panel escaped work area: %+v in %+v", b, area)
		}
	}
}
func TestCatalogHasStaticUIKeys(t *testing.T) {
	for _, language := range []string{"zh-CN", "en-US"} {
		s := testShell(t)
		s.Settings.Language = language
		s.saveSettings(s.Settings)
		if s.Error != "" {
			t.Fatal(s.Error)
		}
		fixtureSite(t, s)
		for _, route := range routes {
			s.Current = route.ID
			ui.NewTester(s.View, 1100, 720)
		}
		data, err := json.Marshal(s.Settings)
		if err != nil || len(data) == 0 {
			t.Fatal(err)
		}
	}
}
