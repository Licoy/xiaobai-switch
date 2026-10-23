package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/clidetect"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeAuthIsPerTargetDraftAndCancellationWritesNothing(t *testing.T) {
	s, st, site := fixtureService(t)
	req := applyRequest(site, domain.ClaudeCode)
	style := domain.AnthropicAPIKey
	req.Claude.AuthKeyStyle = &style
	p, e := s.PrepareApply(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.CancelPreview(context.Background(), p.ID); e != nil {
		t.Fatal(e)
	}
	current, _ := st.GetSite(site.ID)
	if current.ClaudeAuthKeyStyle != "anthropic_auth_token" {
		t.Fatal("preview changed site auth default")
	}
	for _, path := range p.Files {
		if _, e = os.Stat(path); !os.IsNotExist(e) {
			t.Fatal("cancelled preview wrote target")
		}
	}
	p, e = s.PrepareApply(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Apply(context.Background(), p.ID); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(p.Files[0])
	if !strings.Contains(string(raw), "ANTHROPIC_API_KEY") || strings.Contains(string(raw), "ANTHROPIC_AUTH_TOKEN") {
		t.Fatal("target auth override not applied")
	}
	current, _ = st.GetSite(site.ID)
	if current.ClaudeAuthKeyStyle != "anthropic_auth_token" {
		t.Fatal("apply changed shared auth default")
	}
	preset, e := st.GetTargetPreset(site.ID, *site.ActiveAPIKeyID, domain.ClaudeCode)
	if e != nil || preset.Options.Claude.AuthKeyStyle == nil || *preset.Options.Claude.AuthKeyStyle != style {
		t.Fatal("per-target auth preference not saved")
	}
}
func TestKeyRewritePreservesLegacyOptionsAndExternalChangesWithoutPreset(t *testing.T) {
	s, st, site := fixtureService(t)
	applyFixture(t, s, site, domain.ClaudeCode)
	b, _ := st.GetBinding(domain.ClaudeCode)
	path := b.ManagedPaths[0]
	legacy := `{"model":"user-after[1m]","effortLevel":"high","env":{"ANTHROPIC_BASE_URL":"https://example.test","ANTHROPIC_AUTH_TOKEN":"fixture-private-key-long","ANTHROPIC_DEFAULT_OPUS_MODEL":"opus-model[1m]","KEEP":"user"}}`
	if e := os.WriteFile(path, []byte(legacy), 0600); e != nil {
		t.Fatal(e)
	}
	b.ExpectedFields = map[string]string{"ANTHROPIC_BASE_URL": "https://example.test", "model": "fixture-model", "auth_env_key": "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_DEFAULT_OPUS_MODEL": "opus-model[1m]", "effortLevel": "high"}
	if e := st.UpsertBinding(*b); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", filepath.Join(s.options.Root, "xiaobai-switch.db"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("DELETE FROM target_presets"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"user-after"},{"id":"opus-model"}]}`))
	}))
	defer server.Close()
	if _, e = st.UpdateSite(site.ID, domain.UpdateSiteInput{BaseURL: &server.URL}); e != nil {
		t.Fatal(e)
	}
	site, e = st.AddSiteAPIKey(site.ID, domain.AddSiteApiKeyInput{APIKey: "new-key-secret"})
	if e != nil {
		t.Fatal(e)
	}
	newID := ""
	for _, key := range site.APIKeys {
		if key.ID != *site.ActiveAPIKeyID {
			newID = key.ID
		}
	}
	out, e := s.SwitchKey(context.Background(), site.ID, newID)
	if e != nil || len(out.Results) != 1 || !out.Results[0].Ok {
		t.Fatalf("rewrite %v %v", out, e)
	}
	raw, _ := os.ReadFile(path)
	for _, keep := range []string{"user-after[1m]", "opus-model[1m]", "high", "KEEP", "new-key-secret"} {
		if !strings.Contains(string(raw), keep) {
			t.Fatalf("lost %s", keep)
		}
	}
	current, _ := st.GetBinding(domain.ClaudeCode)
	if current.APIKey.ID == nil || *current.APIKey.ID != newID {
		t.Fatal("key binding snapshot not updated")
	}
	preset, e := st.GetTargetPreset(site.ID, newID, domain.ClaudeCode)
	if e != nil || preset != nil {
		t.Fatal("rewrite fabricated default preset")
	}
}
func TestRewriteRejectsStaleRouteAndActiveKeyGuard(t *testing.T) {
	s, st, site := fixtureService(t)
	applyFixture(t, s, site, domain.ClaudeCode)
	b, _ := st.GetBinding(domain.ClaudeCode)
	before, _ := os.ReadFile(b.ManagedPaths[0])
	route := "https://newer-route.test"
	if _, e := st.UpdateSite(site.ID, domain.UpdateSiteInput{BaseURL: &route}); e != nil {
		t.Fatal(e)
	}
	if e := s.rewriteBinding(context.Background(), *b, "https://example.test", ""); e == nil {
		t.Fatal("stale route overwrote newer site route")
	}
	if e := s.rewriteManaged(context.Background(), *b, rewriteGuard{BaseURL: route, KeyID: "inactive-key", Key: "secret", RequireActive: true}); e == nil {
		t.Fatal("inactive key passed guard")
	}
	after, _ := os.ReadFile(b.ManagedPaths[0])
	if string(before) != string(after) {
		t.Fatal("failed guard modified target")
	}
}
func TestCommittedPruneFailureReturnsSuccessWithWarning(t *testing.T) {
	s, st, site := fixtureService(t)
	s.writeJournal = func(path string, j serviceJournal) error {
		if e := saveServiceJournal(path, j); e != nil {
			return e
		}
		if j.Phase == "committed" {
			bad := filepath.Join(s.options.Root, "backups", "codex", "invalid")
			if e := os.MkdirAll(bad, 0700); e != nil {
				return e
			}
			return os.WriteFile(filepath.Join(bad, "transaction.json"), []byte("invalid"), 0600)
		}
		return nil
	}
	p, e := s.PrepareApply(context.Background(), applyRequest(site, domain.ClaudeCode))
	if e != nil {
		t.Fatal(e)
	}
	result, e := s.Apply(context.Background(), p.ID)
	var app *domain.AppError
	if !errors.As(e, &app) || app.Code != "committed_with_warning" || len(result.Results) != 1 || !result.Results[0].Ok {
		t.Fatalf("commit outcome hidden %v %v", result, e)
	}
	binding, e := st.GetBinding(domain.ClaudeCode)
	if e != nil || binding == nil {
		t.Fatal("successful application not committed")
	}
}
func TestUnfinalizedCommitBlocksClearUntilRecoveryMarkerCanBeWritten(t *testing.T) {
	s, st, site := fixtureService(t)
	s.writeJournal = func(path string, j serviceJournal) error {
		if j.Phase == "committed" {
			return errors.New("injected journal finalization failure")
		}
		return saveServiceJournal(path, j)
	}
	p, e := s.PrepareApply(context.Background(), applyRequest(site, domain.ClaudeCode))
	if e != nil {
		t.Fatal(e)
	}
	result, e := s.Apply(context.Background(), p.ID)
	if !isCommitted(e) || len(result.Results) != 1 || !result.Results[0].Ok {
		t.Fatalf("committed boundary lost %v %v", result, e)
	}
	binding, e := st.GetBinding(domain.ClaudeCode)
	if e != nil || binding == nil {
		t.Fatal("binding not committed")
	}
	if e = s.Revert(context.Background(), domain.ClaudeCode); e == nil {
		t.Fatal("clear deleted unresolved commit evidence")
	}
	record, e := st.GetRecord(*binding.ApplyRecordID)
	if e != nil || record == nil {
		t.Fatal("unresolved commit evidence disappeared")
	}
	s.writeJournal = saveServiceJournal
	empty := clidetect.ProbeEnv{}
	reopened, e := New(st, Options{Root: s.options.Root, Home: s.options.Home, CLIEnv: &empty})
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if e = reopened.Revert(context.Background(), domain.ClaudeCode); e != nil {
		t.Fatal(e)
	}
}
func TestRolledBackOuterJournalCannotHidePendingFiles(t *testing.T) {
	s, st, _ := fixtureService(t)
	dir := filepath.Join(s.options.Root, "backups", "claude_code", "100")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	live := filepath.Join(s.options.Home, "fixture-live")
	if e := os.WriteFile(live, []byte("new"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "000.bak"), []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(map[string]any{"state": "prepared", "files": []any{map[string]any{"path": live, "backup": "000.bak", "existed": true, "applied": true}}})
	if e := os.WriteFile(filepath.Join(dir, "transaction.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	if e := saveServiceJournal(dir+".service.json", serviceJournal{Target: domain.ClaudeCode, Operation: "apply", Phase: "rolled_back"}); e != nil {
		t.Fatal(e)
	}
	empty := clidetect.ProbeEnv{}
	reopened, e := New(st, Options{Root: s.options.Root, Home: s.options.Home, CLIEnv: &empty})
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	got, _ := os.ReadFile(live)
	if string(got) != "old" {
		t.Fatal("outer terminal state hid file rollback")
	}
}
func TestBackupMetadataLegacyAndCurrentApplication(t *testing.T) {
	s, _, site := fixtureService(t)
	applyFixture(t, s, site, domain.ClaudeCode)
	backups, e := s.ListBackups(context.Background())
	if e != nil || len(backups) != 1 {
		t.Fatalf("list %v %v", backups, e)
	}
	if backups[0].SiteNameSnapshot == nil || *backups[0].SiteNameSnapshot != site.Name || backups[0].ModelID == nil || *backups[0].ModelID != "fixture-model" {
		t.Fatal("new backup metadata used previous binding")
	}
	legacy := filepath.Join(s.options.Root, "backups", "pi", "123")
	if e = os.MkdirAll(legacy, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(legacy, "models.json"), []byte(`{"providers":{}}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e = fileops.AtomicWrite(filepath.Join(legacy, "xiaobai-backup.json"), []byte(`{"version":1,"target":"pi","createdAt":123,"siteName":"legacy-site","modelId":"legacy-model","applyRecordId":"old-record","files":[]}`)); e != nil {
		t.Fatal(e)
	}
	backups, e = s.ListBackups(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, b := range backups {
		if b.ID == "pi-123" {
			found = true
			if b.SiteNameSnapshot == nil || *b.SiteNameSnapshot != "legacy-site" || b.ModelID == nil || *b.ModelID != "legacy-model" {
				t.Fatal("legacy metadata lost")
			}
		}
	}
	if !found {
		t.Fatal("legacy backup absent")
	}
}
