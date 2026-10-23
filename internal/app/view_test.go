package app

import (
	"errors"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/buildinfo"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func testShell(t *testing.T) *Shell {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XIAOBAI_DEV_HOME", root)
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(config.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	shell, err := NewShell(config, db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := shell.closeManagers(); err != nil {
			t.Error(err)
		}
	})
	return shell
}
func TestStoreBackedNavigation(t *testing.T) {
	s := testShell(t)
	tester := ui.NewTester(s.View, 1100, 720)
	for _, r := range routes {
		if err := tester.Click(s.label(r)); err != nil {
			t.Fatalf("route %s: %v", r.ID, err)
		}
		if s.Current != r.ID {
			t.Fatalf("route %s, want %s", s.Current, r.ID)
		}
	}
	tester.SetDark(true)
	tester.Frame()
	if !tester.HasText("XiaoBaiSwitch " + buildinfo.Version) {
		t.Fatal("about content missing in dark theme")
	}
}
func TestSiteEditorAndSecretLifetime(t *testing.T) {
	s := testShell(t)
	s.editSite(nil)
	s.Editor.Name = "Local test"
	s.Editor.URLs = "https://example.test/v1\nhttps://backup.test/v1"
	s.Editor.Key = "test-secret-not-real"
	s.Editor.ExtraKeys = []string{"test-second-not-real"}
	if err := s.saveSite(); err != nil {
		t.Fatal(err)
	}
	if s.Editor != nil || len(s.Sites) != 1 || len(s.site().APIKeys) != 2 {
		t.Fatal("site or keys not persisted")
	}
	s.selectSite(s.Selected)
	if s.Notice != "" {
		t.Fatal("read operation claimed to save")
	}
	s.editSite(s.site())
	if s.Editor.Key != "" {
		t.Fatal("site editing eagerly decrypted the key")
	}
	s.Editor.Name = "Renamed"
	if err := s.saveSite(); err != nil {
		t.Fatal(err)
	}
	if s.site().Name != "Renamed" {
		t.Fatal("update not persisted")
	}
	s.editKey(*s.site(), s.site().APIKeys[0])
	if s.KeyEditor.Key != "" {
		t.Fatal("key editor eagerly decrypted the key")
	}
	tester := ui.NewTester(s.View, 1100, 720)
	tester.Key(0, ui.KeyEscape)
	if s.KeyEditor != nil {
		t.Fatal("dismissed key editor retained secret state")
	}
	site := s.site()
	if err := s.Store.SetSelectedModel(site.ID, value(site.ActiveAPIKeyID), "manual-model"); err != nil {
		t.Fatal(err)
	}
	if err := s.reload(); err != nil {
		t.Fatal(err)
	}
	if len(s.Models) != 1 || value(s.site().SelectedModelID) != "manual-model" {
		t.Fatal("manual model not stored")
	}
}
func TestSettingsFailureRollsBack(t *testing.T) {
	platformTheme := "system"
	s := testShell(t)
	before := s.Settings
	s.SettingsChanged = func(settings domain.AppSettings) error {
		platformTheme = settings.ThemeMode
		if settings.AutoStart {
			return errors.New("login item unavailable")
		}
		return nil
	}
	next := before
	next.AutoStart = true
	next.ThemeMode = "dark"
	s.saveSettings(next)
	actual, err := s.Store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if actual.AutoStart || s.Settings.AutoStart || s.Error == "" || platformTheme != before.ThemeMode {
		t.Fatal("failed native setting was persisted or masked")
	}
	next = before
	next.PrimaryColor = "invalid"
	s.saveSettings(next)
	if s.Settings.PrimaryColor != before.PrimaryColor || s.Error == "" {
		t.Fatal("invalid theme accepted")
	}
}
func TestDevelopmentHome(t *testing.T) {
	isolated, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XIAOBAI_DEV_HOME", isolated)
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !config.Development || config.UseRealHome || config.Home != isolated || config.DataDir != filepath.Join(isolated, ".xiaobai-switch") {
		t.Fatalf("invalid isolation: %+v", config)
	}
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{realHome, "/", "relative"} {
		t.Setenv("XIAOBAI_DEV_HOME", invalid)
		if _, err := LoadConfig(); err == nil {
			t.Errorf("accepted unsafe home %q", invalid)
		}
	}
}

func TestRealHomeDevelopmentDoesNotClaimIsolation(t *testing.T) {
	s := testShell(t)
	s.Config.UseRealHome = true
	s.Current = "paths"
	tester := ui.NewTester(s.View, 1100, 720)
	if !tester.HasText(s.text("real_home_development")) || !tester.HasText(s.text("real_home_development_notice")) {
		t.Fatal("real-home development notice missing")
	}
	if tester.HasText(s.text("development")) || tester.HasText(s.text("isolation")) {
		t.Fatal("real-home mode falsely claims isolation")
	}
}
