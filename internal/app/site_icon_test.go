package app

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/siteicon"
)

func fixturePNG(t *testing.T) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	img.SetNRGBA(1, 1, color.NRGBA{12, 118, 248, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestSiteIconCachesFoundAndMissingButRetriesUnreachable(t *testing.T) {
	s := testShell(t)
	cache, ctx := s.iconCache(), context.Background()
	icon := fixturePNG(t)
	calls := 0
	found := func(_ context.Context, _ domain.AppSettings, origin string, usable func([]byte) bool) siteicon.Result {
		calls++
		if origin != "https://relay.example.test" || !usable(icon) || usable([]byte("not an image")) {
			t.Fatal("origin or decoder check wrong")
		}
		return siteicon.Result{Icon: icon, Reached: true}
	}
	if loadSiteIcon(ctx, cache, found, s.Settings, "site-a", "https://relay.example.test") == nil {
		t.Fatal("found icon not shown")
	}
	if loadSiteIcon(ctx, cache, found, s.Settings, "site-a", "https://relay.example.test") == nil || calls != 1 {
		t.Fatal("cached icon fetched again")
	}
	none := func(context.Context, domain.AppSettings, string, func([]byte) bool) siteicon.Result {
		calls++
		return siteicon.Result{Reached: true}
	}
	if loadSiteIcon(ctx, cache, none, s.Settings, "site-b", "https://relay.example.test") != nil {
		t.Fatal("missing icon invented")
	}
	if _, hit := cache.Load("site-b", "https://relay.example.test"); !hit {
		t.Fatal("missing icon not remembered")
	}
	offline := func(context.Context, domain.AppSettings, string, func([]byte) bool) siteicon.Result {
		return siteicon.Result{}
	}
	loadSiteIcon(ctx, cache, offline, s.Settings, "site-c", "https://relay.example.test")
	if _, hit := cache.Load("site-c", "https://relay.example.test"); hit {
		t.Fatal("unreachable origin remembered as iconless")
	}
}

func TestSiteAvatarShowsIconLetterOrSpinner(t *testing.T) {
	s := testShell(t)
	site := fixtureSite(t, s)
	origin, _ := siteicon.Origin(site.BaseURL)
	bm, err := ui.DecodeBitmap(fixturePNG(t))
	if err != nil {
		t.Fatal(err)
	}
	letter := string([]rune(site.Name)[0])
	draw := func(c *ui.Context) { s.siteAvatar(c, site, 34) }
	s.SiteIcons = map[string]*siteIcon{site.ID: {Origin: origin, Src: bm}}
	tester := ui.NewTester(draw, 100, 100)
	if _, ok := tester.Find(site.Name); !ok || hasExact(tester, letter) {
		t.Fatal("site icon not drawn")
	}
	s.SiteIcons[site.ID] = &siteIcon{Origin: origin}
	tester.Frame()
	if !hasExact(tester, letter) {
		t.Fatal("letter fallback missing")
	}
	s.forgetSiteIcon(site.ID)
	if s.SiteIcons[site.ID] != nil {
		t.Fatal("edited site kept its icon")
	}
}

func TestFetchedSiteIconRendersPixels(t *testing.T) {
	for _, fixture := range []struct {
		name, contentType string
		data              []byte
	}{
		{"png", "image/png", solidIconPNG(t)},
		{"svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="#0c76f8"/></svg>`)},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path == "/" {
					w.Header().Set("Content-Type", "text/html")
					w.Write([]byte(`<html><head><link rel="icon" href="/icon"></head></html>`))
					return
				}
				if r.URL.Path != "/icon" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", fixture.contentType)
				w.Write(fixture.data)
			}))
			defer server.Close()
			s := testShell(t)
			s.Settings.ProxyMode = "none"
			site := domain.SiteDTO{ID: "downloaded-icon", Name: "Downloaded icon", BaseURL: server.URL + "/v1"}
			writeLegacyIconCache(t, s.iconCache(), site.ID, server.URL, nil)

			if loadSiteIcon(context.Background(), s.iconCache(), fetchSiteIcon, s.Settings, site.ID, server.URL) == nil {
				t.Fatal("downloaded icon did not decode")
			}
			if requests.Load() != 2 {
				t.Fatalf("requests = %d, want homepage and icon", requests.Load())
			}
			tester := ui.NewTester(func(c *ui.Context) { s.siteAvatar(c, site, 80) }, 100, 100)
			img := tester.Image()
			pixel := color.NRGBAModel.Convert(img.At(40, 40)).(color.NRGBA)
			if pixel != (color.NRGBA{12, 118, 248, 255}) {
				t.Fatalf("avatar center pixel = %v, want downloaded blue icon", pixel)
			}
		})
	}
}

func solidIconPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{12, 118, 248, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestLegacyMissingSiteIconCacheIsRefreshed(t *testing.T) {
	s := testShell(t)
	cache := s.iconCache()
	origin := "https://relay.example.test"
	writeLegacyIconCache(t, cache, "legacy-site", origin, nil)
	calls := 0
	fetch := func(context.Context, domain.AppSettings, string, func([]byte) bool) siteicon.Result {
		calls++
		return siteicon.Result{Reached: true}
	}
	for i := 0; i < 2; i++ {
		loadSiteIcon(context.Background(), cache, fetch, s.Settings, "legacy-site", origin)
	}
	if calls != 1 {
		t.Fatalf("legacy missing icon fetched %d times, want one refresh then cached absence", calls)
	}
	icon := solidIconPNG(t)
	writeLegacyIconCache(t, cache, "positive-site", origin, icon)
	if loadSiteIcon(context.Background(), cache, fetch, s.Settings, "positive-site", origin) == nil || calls != 1 {
		t.Fatal("valid positive cache was not preserved")
	}
}

func writeLegacyIconCache(t *testing.T, cache siteicon.Cache, id, origin string, icon []byte) {
	t.Helper()
	if err := os.MkdirAll(cache.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"origin": origin, "icon": icon})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(cache.Dir, id+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
