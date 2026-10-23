package service

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeleteSiteRetainsOrphansAndCleanupFailureKeepsSite(t *testing.T) {
	t.Run("keep", func(t *testing.T) {
		s, st, site := fixtureService(t)
		applyFixture(t, s, site, domain.ClaudeCode)
		b, _ := st.GetBinding(domain.ClaudeCode)
		before, _ := os.ReadFile(b.ManagedPaths[0])
		if e := s.DeleteSite(context.Background(), site.ID); e != nil {
			t.Fatal(e)
		}
		b, e := st.GetBinding(domain.ClaudeCode)
		if e != nil || b == nil || !b.Orphan || b.SiteID != nil {
			t.Fatal("delete did not atomically orphan binding")
		}
		after, _ := os.ReadFile(b.ManagedPaths[0])
		if string(before) != string(after) {
			t.Fatal("default site deletion altered target")
		}
	})
	t.Run("cleanup-failure", func(t *testing.T) {
		s, st, site := fixtureService(t)
		applyFixture(t, s, site, domain.ClaudeCode)
		b, _ := st.GetBinding(domain.ClaudeCode)
		if e := os.WriteFile(b.ManagedPaths[0], []byte("broken JSON"), 0600); e != nil {
			t.Fatal(e)
		}
		result, e := s.DeleteSiteWithCleanup(context.Background(), site.ID)
		if e == nil || result.Deleted || len(result.Results) != 1 || result.Results[0].Ok {
			t.Fatal("partial cleanup reported site deleted")
		}
		if _, e = st.GetSite(site.ID); e != nil {
			t.Fatal("failed cleanup deleted site")
		}
	})
}
func TestRouteChoiceCanLeaveAppliedTargetsUnchanged(t *testing.T) {
	s, st, site := fixtureService(t)
	urls := []string{"https://example.test", "https://route.test"}
	if _, e := st.UpdateSite(site.ID, domain.UpdateSiteInput{BaseURLs: &urls}); e != nil {
		t.Fatal(e)
	}
	applyFixture(t, s, site, domain.ClaudeCode)
	b, _ := st.GetBinding(domain.ClaudeCode)
	before, _ := os.ReadFile(b.ManagedPaths[0])
	result, e := s.SwitchRouteWithOptions(context.Background(), site.ID, "https://route.test", false)
	if e != nil || result.Site.BaseURL != "https://route.test" || len(result.Results) != 0 {
		t.Fatalf("route choice %v %v", result, e)
	}
	after, _ := os.ReadFile(b.ManagedPaths[0])
	if string(before) != string(after) {
		t.Fatal("station-only route change rewrote target")
	}
}
func TestLegacyBackupRestoreIgnoresUntrustedOriginalPath(t *testing.T) {
	s, st, _ := fixtureService(t)
	backup := filepath.Join(s.options.Root, "backups", "claude_code", "1")
	if e := os.MkdirAll(backup, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(backup, "settings.json"), []byte(`{"model":"restored"}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(backup, ".origins.json"), []byte(`{"settings.json":"/untrusted-destination/config"}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e := s.RestoreBackup(context.Background(), "claude_code-1"); e != nil {
		t.Fatal(e)
	}
	settings, _ := st.GetSettings()
	home, _ := s.targetHome(domain.ClaudeCode, settings)
	raw, e := os.ReadFile(filepath.Join(home, "settings.json"))
	if e != nil || !strings.Contains(string(raw), "restored") {
		t.Fatal("backup not mapped to current allowed target")
	}
}
