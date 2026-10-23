package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/service"
)

func TestClaudePrivateAuthAndLiveModelHydration(t *testing.T) {
	s := testShell(t)
	site := fixtureSite(t, s)
	d := s.draft(domain.ClaudeCode)
	d.Auth = string(domain.AnthropicAPIKey)
	if err := s.saveDraft(d); err != nil {
		t.Fatal(err)
	}
	s.hydrateDraft(d, site)
	if d.Auth != string(domain.AnthropicAPIKey) {
		t.Fatal("private authentication preset lost")
	}
	preview, err := s.Service.PrepareApply(s.Context, s.buildRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Service.Apply(s.Context, preview.ID); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Config.Home, ".claude", "settings.json")
	if err = os.WriteFile(path, []byte(`{"model":"external-model","env":{"ANTHROPIC_API_KEY":"fixture-not-real"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s.acceptStatuses(rows)
	delete(s.Drafts, domain.ClaudeCode)
	actual := s.draft(domain.ClaudeCode)
	if actual.Request.ModelID != "external-model" || actual.Auth != string(domain.AnthropicAPIKey) {
		t.Fatalf("live model/auth did not win: %s %s", actual.Request.ModelID, actual.Auth)
	}
}
func TestDetectionCompletionReplacesRefreshingAndCoalesces(t *testing.T) {
	s := testShell(t)
	s.Current = "claude"
	d := s.draft(domain.ClaudeCode)
	s.acceptStatuses([]service.TargetStatus{{Target: domain.ClaudeCode, Status: domain.NotApplied, CLIRefreshing: true}})
	tester := ui.NewTester(s.View, 1100, 720)
	if !tester.HasText(s.text("cli_detecting")) {
		t.Fatal("missing detecting state")
	}
	s.acceptStatuses([]service.TargetStatus{{Target: domain.ClaudeCode, Status: domain.NotApplied, Installed: false, CLIRefreshing: false}})
	tester.Frame()
	if tester.HasText(s.text("cli_detecting")) || d.Status.CLIRefreshing {
		t.Fatal("detecting state stuck")
	}
	s.StatusRefreshing = true
	s.queueStatusRefresh()
	if !s.StatusRefreshPending {
		t.Fatal("completion event lost while refresh in flight")
	}
	s.StatusRefreshing = false
	s.StatusRefreshPending = false
}
func TestPendingRestoreBlocksWritesAndDevelopmentUpdates(t *testing.T) {
	s := testShell(t)
	site := fixtureSite(t, s)
	if s.Updates.Snapshot().Enabled {
		t.Fatal("development updater enabled")
	}
	s.RestoreReady = true
	next := s.Settings
	next.AutoStart = !next.AutoStart
	s.saveSettings(next)
	actual, err := s.Store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if actual.AutoStart == next.AutoStart {
		t.Fatal("settings changed after restore queued")
	}
	called := false
	s.mutate(func() error { called = true; return nil })
	s.task(func() (func(), error) { called = true; return nil, nil })
	if called {
		t.Fatal("write ran after restore queued")
	}
	if err := s.saveDraft(s.draft(domain.ClaudeCode)); err == nil {
		t.Fatal("preset write accepted")
	}
	if quotaKey(site) == "" {
		t.Fatal("missing quota identity")
	}
	tester := ui.NewTester(s.View, 1100, 720)
	if !tester.HasText(s.text("restart_now")) {
		t.Fatal("restore restart missing")
	}
}

func TestLoginHiddenLegacyPreference(t *testing.T) {
	settings := domain.AppSettings{StartInTray: true}
	if !loginHidden(settings) {
		t.Fatal("legacy login hidden preference lost")
	}
	settings.LaunchAtLoginHidden = pointer(false)
	if loginHidden(settings) {
		t.Fatal("explicit login visibility did not override legacy")
	}
}
