package app

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"time"
)

func (s *Shell) probeRoutes(site domain.SiteDTO) {
	settings := s.Settings
	s.task(func() (func(), error) {
		client, err := netx.NewClient(settings, 10*time.Second)
		if err != nil {
			return nil, err
		}
		defer client.CloseIdleConnections()
		results, err := netx.ProbeURLs(s.Context, client, site.BaseURLs)
		if err != nil {
			return nil, err
		}
		return func() { s.RouteProbes[site.ID] = results; s.RouteProbeTimes[site.ID] = time.Now() }, nil
	})
}
func (s *Shell) routePanel(c *ui.Context, site domain.SiteDTO) {
	t := c.Theme()
	Panel(c).Grow(1).Children(func() {
		PanelTitle(c, icRoute, TintTeal, s.text("routes"), "", false, func() {
			if BusyPillButton(c, icPulse, s.text("probe_routes"), false, s.Busy).Clicked() {
				s.probeRoutes(site)
			}
		})
		route := site.BaseURL
		options := []choice{}
		for _, u := range site.BaseURLs {
			options = append(options, choice{u, u})
		}
		field(c, s.text("route"), func() {
			if choose(c.Key("route"), &route, options).Changed() {
				s.routeChange(site.ID, route)
			}
		})
		results := map[string]domain.UrlProbeResult{}
		for _, r := range s.RouteProbes[site.ID] {
			results[r.URL] = r
		}
		ui.Column(c).FillWidth().Gap(0).Padding(2, 12).Radius(12).Background(pal(t).Inset).Children(func() {
			for i, u := range site.BaseURLs {
				if i > 0 {
					Divider(c)
				}
				ui.Column(c.Key(u)).FillWidth().Gap(3).Padding(7, 0).Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						col := TintGray.C(t)
						if u == site.BaseURL {
							col = TintTeal.C(t)
						}
						ui.Box(c).Size(7, 7).Radius(4).Background(col)
						ui.Text(c, hostOf(u)).FontSize(12).SingleLine().Grow(1).Basis(0).Tooltip(u)
						if r, ok := results[u]; ok {
							if r.Ok {
								DotPill(c, fmt.Sprintf("%d ms", r.LatencyMS), latencyColor(t, r.LatencyMS))
							} else {
								DotPill(c, s.text("operation_failed"), t.Danger)
							}
						}
					})
					if r, ok := results[u]; ok && r.Error != nil {
						ui.Text(c, s.safeErrorDetail(*r.Error)).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
					}
				})
			}
		})
		if fetched, ok := s.RouteProbeTimes[site.ID]; ok {
			label := s.text("updated")
			if time.Since(fetched) > time.Duration(s.Settings.RouteProbeTtlMinutes)*time.Minute {
				label = s.text("probe_expired")
			}
			ui.Text(c, label+": "+fetched.Format("2006-01-02 15:04:05")).FontSize(11).TextColor(t.TextMuted)
		}
	})
}
func latencyColor(t *ui.Theme, ms uint64) ui.Color {
	switch {
	case ms < 800:
		return t.Success
	case ms < 2000:
		return t.Warning
	}
	return t.Danger
}
func (s *Shell) urlPreviewView(c *ui.Context, text string, open *bool) {
	disclosure(c, "url-preview", icLinks, s.text("url_preview"), open, func() {
		urls := lines(text)
		if len(urls) == 0 {
			ui.Text(c, "—").FontSize(12).TextColor(c.Theme().TextMuted)
			return
		}
		preview, err := netx.NormalizeURL(urls[0])
		if err != nil {
			Banner(c, icWarning, c.Theme().Warning, s.errorText(err))
			return
		}
		optionRows(c, func(row func(label, detail string, control func())) {
			for _, item := range [][2]string{{"Claude Code", preview.ClaudeBaseURL}, {"Codex", preview.CodexBaseURL}, {s.text("models"), preview.ModelsURL}} {
				row(item[0], "", func() { mono(c, item[1]).Selectable().SingleLine().MaxWidth(320) })
			}
		})
	})
}
