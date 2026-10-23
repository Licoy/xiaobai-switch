package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/siteicon"
)

// TestPageShots renders every page offscreen for visual review; it runs only with XBS_SHOTS set to an output directory
func TestPageShots(t *testing.T) {
	dir := os.Getenv("XBS_SHOTS")
	if dir == "" {
		t.Skip("XBS_SHOTS not set")
	}
	s := testShell(t)
	for _, seed := range []struct{ name, url string }{{"Example Relay", "https://relay.example.test/v1"}, {"Backup Gateway", "https://gw.example.test/v1"}, {"Local Proxy", "http://127.0.0.1:8317/v1"}} {
		s.editSite(nil)
		s.Editor.Name = seed.name
		s.Editor.URLs = seed.url + "\nhttps://alt.example.test/v1"
		s.Editor.Key = "sk-test-not-real-0000"
		if err := s.saveSite(); err != nil {
			t.Fatal(err)
		}
		site := s.site()
		models := []domain.SiteModelDTO{}
		for _, id := range []string{"claude-sonnet-5-5", "claude-opus-5-5", "gpt-6", "gpt-6-mini"} {
			models = append(models, domain.SiteModelDTO{ModelID: id})
		}
		if err := s.Store.ReplaceModels(site.ID, value(site.ActiveAPIKeyID), models); err != nil {
			t.Fatal(err)
		}
		if err := s.Store.SetSelectedModel(site.ID, value(site.ActiveAPIKeyID), "claude-sonnet-5-5"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.reload(); err != nil {
		t.Fatal(err)
	}
	s.selectSite(s.Sites[0].ID)
	// the first site stands in for one whose favicon was found
	origin, _ := siteicon.Origin(s.Sites[0].BaseURL)
	s.SiteIcons = map[string]*siteIcon{s.Sites[0].ID: {Origin: origin, Src: logo}}
	shot := func(name string, view func(*ui.Context), w, h int, dark bool) {
		tester := ui.NewTester(view, w, h)
		tester.SetScale(2)
		tester.SetDark(dark)
		tester.Frame()
		tester.Frame()
		if dark {
			name += "-dark"
		}
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tester.Image()); err != nil {
			t.Fatal(err)
		}
	}
	for _, dark := range []bool{false, true} {
		for _, r := range routes {
			s.Current = r.ID
			shot(r.ID, s.View, 1100, 720, dark)
		}
		h := &host{shell: s}
		shot("tray", h.trayView, 420, 680, dark)
		s.Current = "sites"
		s.editSite(s.site())
		shot("site-editor", s.View, 1100, 720, dark)
		s.Editor = nil
		s.Notice = s.text("preset_saved")
		shot("prompt", s.View, 1100, 720, dark)
		s.clearPrompt()
		s.confirmDanger(s.text("delete"), "Example Relay", func() error { return nil })
		shot("confirm", s.View, 1100, 720, dark)
		s.Confirm = nil
	}
}
