package app

import (
	"bytes"
	"context"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"github.com/licoy/xiaobai-switch/internal/siteicon"
)

// siteIcon is the icon of a site for one origin; Src stays nil while loading and when the site has none
type siteIcon struct {
	Origin  string
	Src     ui.ImageSource
	Loading bool
}

// loadSiteIcon reads the cached icon of a site at origin, or looks it up and caches what it finds
func loadSiteIcon(ctx context.Context, cache siteicon.Cache, fetch func(context.Context, domain.AppSettings, string, func([]byte) bool) siteicon.Result, settings domain.AppSettings, id, origin string) ui.ImageSource {
	if data, hit := cache.Load(id, origin); hit {
		return decodeSiteIcon(data)
	}
	if fetch == nil {
		return nil
	}
	result := fetch(ctx, settings, origin, func(b []byte) bool { return decodeSiteIcon(b) != nil })
	// an origin that never answered is tried again next launch instead of being remembered as iconless
	if result.Reached && ctx.Err() == nil {
		_ = cache.Save(id, origin, result.Icon)
	}
	return decodeSiteIcon(result.Icon)
}

// fetchSiteIcon looks the icon up over the network with the proxy settings, as the Tauri fetch commands did
func fetchSiteIcon(ctx context.Context, settings domain.AppSettings, origin string, usable func([]byte) bool) siteicon.Result {
	client, err := netx.NewClient(settings, 8*time.Second)
	if err != nil {
		return siteicon.Result{}
	}
	defer client.CloseIdleConnections()
	return siteicon.Resolve(ctx, client, origin, usable)
}

func decodeSiteIcon(data []byte) ui.ImageSource {
	switch {
	case len(data) == 0:
		return nil
	case siteicon.IsICO(data):
		if img, err := siteicon.DecodeICO(data); err == nil {
			return ui.NewBitmap(img)
		}
		return nil
	case bytes.Contains(data[:min(len(data), 512)], []byte("<svg")):
		if svg, err := ui.ParseSVG(data); err == nil {
			return svg
		}
		return nil
	}
	if bm, err := ui.DecodeBitmap(data); err == nil {
		return bm
	}
	return nil
}

func (s *Shell) iconCache() siteicon.Cache {
	return siteicon.Cache{Dir: filepath.Join(s.Config.DataDir, "cache", "site-icons")}
}

// siteIcon returns the icon state of a site, starting to load it from the cache or the network the first time
func (s *Shell) siteIcon(site domain.SiteDTO) *siteIcon {
	origin, ok := siteicon.Origin(site.BaseURL)
	if !ok {
		return nil
	}
	if icon := s.SiteIcons[site.ID]; icon != nil && icon.Origin == origin {
		return icon
	}
	if s.SiteIcons == nil {
		s.SiteIcons = map[string]*siteIcon{}
	}
	icon := &siteIcon{Origin: origin, Loading: true}
	s.SiteIcons[site.ID] = icon
	if s.FetchIcon == nil {
		icon.Src, icon.Loading = loadSiteIcon(s.Context, s.iconCache(), nil, s.Settings, site.ID, origin), false
		return icon
	}
	cache, settings, fetch, ctx, id := s.iconCache(), s.Settings, s.FetchIcon, s.Context, site.ID
	go func() {
		src := loadSiteIcon(ctx, cache, fetch, settings, id, origin)
		mygo.RunOnMain(func() {
			if s.SiteIcons[id] != icon {
				return
			}
			icon.Src, icon.Loading = src, false
			if s.Invalidate != nil {
				s.Invalidate()
			}
		})
	}()
	return icon
}

// forgetSiteIcon drops the icon of a site so it is looked up again, as editing a site did in the Tauri version
func (s *Shell) forgetSiteIcon(siteID string) {
	delete(s.SiteIcons, siteID)
	_ = s.iconCache().Invalidate(siteID)
}

// siteAvatar shows the icon of a site on a light tile, a spinner while it loads, or its letter when it has none
func (s *Shell) siteAvatar(c *ui.Context, site domain.SiteDTO, size float32) ui.Element {
	icon := s.siteIcon(site)
	if icon == nil || (!icon.Loading && icon.Src == nil) {
		return letterTile(c, site.Name, site.ID, size)
	}
	t := c.Theme()
	tile := ui.Row(c).Size(size, size).Shrink(0).Radius(size * 0.28).Justify(ui.Center).AlignItems(ui.Center).Tooltip(hostOf(site.BaseURL))
	if icon.Loading {
		return tile.Background(pal(t).Inset).Children(func() {
			ui.Spinner(c).Size(size*0.42, size*0.42).Label(site.Name)
		})
	}
	inset := size * 0.14
	return tile.Background(ui.RGB(255, 255, 255)).Border(1, pal(t).Line).Children(func() {
		ui.Image(c, icon.Src).Size(size-2*inset, size-2*inset).Label(site.Name)
	})
}
