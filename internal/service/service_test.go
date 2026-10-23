package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/clidetect"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/store"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixtureService(t *testing.T) (*Service, *store.Store, domain.SiteDTO) {
	t.Helper()
	root := t.TempDir()
	st, e := store.Open(filepath.Join(root, ".xiaobai-switch"))
	if e != nil {
		t.Fatal(e)
	}
	settings := domain.DefaultSettings()
	settings.CodexEnvInjectMode = "file_only"
	settings.ProxyMode = "none"
	if e = st.SaveSettings(settings); e != nil {
		t.Fatal(e)
	}
	empty := clidetect.ProbeEnv{}
	svc, e := New(st, Options{Root: filepath.Join(root, ".xiaobai-switch"), Home: root, CLIEnv: &empty})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { svc.Close(); st.Close() })
	site, e := svc.CreateSite(context.Background(), domain.CreateSiteInput{Name: "fixture", BaseURL: "https://example.test", APIKey: "fixture-private-key-long"})
	if e != nil {
		t.Fatal(e)
	}
	if e = st.SetSelectedModel(site.ID, *site.ActiveAPIKeyID, "fixture-model"); e != nil {
		t.Fatal(e)
	}
	site, e = st.GetSite(site.ID)
	if e != nil {
		t.Fatal(e)
	}
	return svc, st, site
}
func applyRequest(site domain.SiteDTO, target domain.TargetKind) domain.ApplyRequest {
	return domain.ApplyRequest{SiteID: site.ID, APIKeyID: *site.ActiveAPIKeyID, Target: target, ModelID: "fixture-model"}
}
func applyFixture(t *testing.T, s *Service, site domain.SiteDTO, target domain.TargetKind) domain.ApplyResult {
	t.Helper()
	p, e := s.PrepareApply(context.Background(), applyRequest(site, target))
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Apply(context.Background(), p.ID)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestFourTargetsPreviewApplyBindingPresetAndStatus(t *testing.T) {
	s, st, site := fixtureService(t)
	for _, target := range targets {
		t.Run(string(target), func(t *testing.T) {
			req := applyRequest(site, target)
			p, e := s.PrepareApply(context.Background(), req)
			if e != nil {
				t.Fatal(e)
			}
			raw, _ := json.Marshal(p)
			if strings.Contains(string(raw), "fixture-private-key-long") {
				t.Fatal("preview exposes key")
			}
			for _, path := range p.Files {
				if _, e = os.Stat(path); !os.IsNotExist(e) {
					t.Fatal("preview touched target")
				}
			}
			r, e := s.Apply(context.Background(), p.ID)
			if e != nil || len(r.Results) != 1 || !r.Results[0].Ok {
				t.Fatalf("apply %v %v", r, e)
			}
			binding, e := st.GetBinding(target)
			if e != nil || binding == nil || binding.APIKey.ID == nil || *binding.APIKey.ID != *site.ActiveAPIKeyID {
				t.Fatalf("binding %v %v", binding, e)
			}
			preset, e := st.GetTargetPreset(site.ID, *site.ActiveAPIKeyID, target)
			if e != nil || preset == nil || preset.Options.ModelID != "fixture-model" {
				t.Fatalf("preset %v %v", preset, e)
			}
			states, e := s.Status(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, state := range states {
				if state.Target == target {
					found = true
					if state.Status != domain.Applied {
						t.Fatalf("status %v", state)
					}
				}
			}
			if !found {
				t.Fatal("target absent")
			}
			if _, e = s.Apply(context.Background(), p.ID); e == nil {
				t.Fatal("preview reusable")
			}
		})
	}
}
func TestPreviewInvalidatesAfterFileOrDatabaseChanges(t *testing.T) {
	for _, change := range []string{"file", "settings", "key", "cache", "preset"} {
		t.Run(change, func(t *testing.T) {
			s, st, site := fixtureService(t)
			p, e := s.PrepareApply(context.Background(), applyRequest(site, domain.ClaudeCode))
			if e != nil {
				t.Fatal(e)
			}
			switch change {
			case "file":
				if e = os.MkdirAll(filepath.Dir(p.Files[0]), 0700); e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(p.Files[0], []byte(`{"model":"external"}`), 0600); e != nil {
					t.Fatal(e)
				}
			case "settings":
				v, _ := st.GetSettings()
				v.ForceExclusiveClaudeAuthKey = true
				e = st.SaveSettings(v)
			case "key":
				secret := "updated-key"
				_, e = st.UpdateSiteAPIKey(site.ID, *site.ActiveAPIKeyID, domain.UpdateSiteApiKeyInput{APIKey: &secret})
			case "cache":
				e = st.SetSelectedModel(site.ID, *site.ActiveAPIKeyID, "another-model")
			case "preset":
				e = st.SaveTargetPreset(domain.TargetPreset{SiteID: site.ID, APIKeyID: *site.ActiveAPIKeyID, Target: domain.ClaudeCode, Options: applyRequest(site, domain.ClaudeCode), UpdatedAt: 1})
			}
			if e != nil {
				t.Fatal(e)
			}
			if _, e = s.Apply(context.Background(), p.ID); e == nil {
				t.Fatal("stale preview applied")
			}
			b, e := st.GetBinding(domain.ClaudeCode)
			if e != nil || b != nil {
				t.Fatal("stale apply created binding")
			}
		})
	}
}
func TestStrictSelectionValidationAndPreviewCancellation(t *testing.T) {
	s, st, site := fixtureService(t)
	req := applyRequest(site, domain.ClaudeCode)
	req.APIKeyID = "foreign-key"
	if _, e := s.PrepareApply(context.Background(), req); e == nil {
		t.Fatal("cross-site key accepted")
	}
	req = applyRequest(site, domain.ClaudeCode)
	req.ModelID = "uncached"
	if _, e := s.PrepareApply(context.Background(), req); e == nil {
		t.Fatal("uncached model accepted")
	}
	p, e := s.PrepareApply(context.Background(), applyRequest(site, domain.ClaudeCode))
	if e != nil {
		t.Fatal(e)
	}
	if e = s.CancelPreview(context.Background(), p.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Apply(context.Background(), p.ID); e == nil {
		t.Fatal("cancelled preview applied")
	}
	off := false
	if _, e = st.UpdateSite(site.ID, domain.UpdateSiteInput{Enabled: &off}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.PrepareApply(context.Background(), applyRequest(site, domain.ClaudeCode)); e == nil {
		t.Fatal("disabled site accepted")
	}
}
func TestDatabaseFailureRollsTargetFilesBack(t *testing.T) {
	s, st, site := fixtureService(t)
	settings, _ := st.GetSettings()
	home, e := s.targetHome(domain.ClaudeCode, settings)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(home, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(home, "settings.json")
	original := []byte(`{"model":"original","keep":true}`)
	if e = os.WriteFile(path, original, 0600); e != nil {
		t.Fatal(e)
	}
	p, e := s.PrepareApply(context.Background(), applyRequest(site, domain.ClaudeCode))
	if e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", filepath.Join(s.options.Root, "xiaobai-switch.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec(`CREATE TRIGGER fail_apply BEFORE INSERT ON apply_records BEGIN SELECT RAISE(FAIL,'injected commit failure'); END`); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Apply(context.Background(), p.ID); e == nil {
		t.Fatal("DB failure reported success")
	}
	got, e := os.ReadFile(path)
	if e != nil || string(got) != string(original) {
		t.Fatalf("target not rolled back %s %v", got, e)
	}
	binding, e := st.GetBinding(domain.ClaudeCode)
	if e != nil || binding != nil {
		t.Fatal("failed commit created binding")
	}
}
func TestOnChangeCanReloadSharedServiceWithoutDeadlock(t *testing.T) {
	s, _, _ := fixtureService(t)
	var calls atomic.Int32
	completed := make(chan struct{}, 4)
	s.options.OnChange = func() {
		calls.Add(1)
		if _, e := s.Status(context.Background()); e != nil {
			t.Error(e)
		}
		completed <- struct{}{}
	}
	if _, e := s.CreateSite(context.Background(), domain.CreateSiteInput{Name: "second", BaseURL: "https://second.test", APIKey: "second-secret"}); e != nil {
		t.Fatal(e)
	}
	for range 2 {
		select {
		case <-completed:
		case <-time.After(time.Second):
			t.Fatal("shared callback did not complete")
		}
	}
	if calls.Load() < 2 {
		t.Fatal("CLI completion was not published")
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
}
