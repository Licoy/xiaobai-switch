package app

import (
	"fmt"
	"net/url"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

var targetKinds = []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime}

func (s *Shell) contentWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	return w - sidebarWidth - 48
}
func (s *Shell) overviewView(c *ui.Context) {
	enabled, keys, applied := 0, 0, 0
	for _, site := range s.Sites {
		if site.Enabled {
			enabled++
		}
		keys += len(site.APIKeys)
	}
	for _, target := range targetKinds {
		if status, ok := s.Statuses[target]; ok && status.Status == domain.Applied {
			applied++
		}
	}
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		Stat(c, icApps, TintIndigo, s.text("stat_sites"), fmt.Sprintf("%d", len(s.Sites)))
		Stat(c, icToggle, TintGreen, s.text("stat_enabled"), fmt.Sprintf("%d", enabled))
		Stat(c, icKey, TintYellow, s.text("api_keys"), fmt.Sprintf("%d", keys))
		Stat(c, icSend, TintBlue, s.text("stat_applied"), fmt.Sprintf("%d / %d", applied, len(targetKinds)))
	})
	n := columns(s.contentWidth(c), 340, 2)
	grid(c, n, len(targetKinds), 14, func(i int) { s.targetCard(c, targetKinds[i]) })
	ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(3).Basis(0).Children(func() { s.overviewSites(c) })
		ui.Column(c).Grow(2).Basis(0).Children(func() { s.overviewBackups(c) })
	})
}
func (s *Shell) targetCard(c *ui.Context, target domain.TargetKind) {
	t := c.Theme()
	d := s.draft(target)
	r := routeByID(routeForTarget(target))
	b := ui.ButtonBase(c.Key("target-card-" + r.ID)).FillWidth().Label(targetLabel(target)).Padding(16).Gap(14).Radius(16).Background(pal(t).Card).FocusRing(false)
	if b.Hovered() {
		b.Background(pal(t).Card.Mix(r.Tint.C(t), 0.06))
	}
	b.Children(func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				targetTile(c, target, 38)
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, targetLabel(target)).FontSize(15).FontWeight(700).TextColor(r.Tint.Text(t)).SingleLine()
					cli := "CLI · —"
					if d.Status != nil {
						switch {
						case d.Status.CLIRefreshing:
							cli = s.text("cli_detecting")
						case d.Status.Installed:
							cli = s.text("cli_installed") + " " + value(d.Status.Version)
						default:
							cli = s.text("cli_missing")
						}
					}
					ui.Text(c, cli).FontSize(11).TextColor(t.TextMuted).SingleLine()
				})
				label, col := s.targetTone(target)
				DotPill(c, label, col)
			})
			site, model, key := "—", "—", "—"
			if d.Binding != nil {
				site, model = d.Binding.SiteNameSnapshot, d.Binding.ModelID
				if d.Binding.APIKey.Prefix != nil {
					key = *d.Binding.APIKey.Prefix
				}
			}
			ui.Column(c).FillWidth().Gap(2).Children(func() {
				ui.Text(c, s.text("current_model")).FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, model).FontSize(18).FontWeight(700).SingleLine()
			})
			Metrics(c, Metric{Label: s.text("site"), Value: site}, Metric{Label: "API key", Value: key})
		})
	})
	if b.Clicked() {
		s.navigate(r.ID)
	}
}
func (s *Shell) overviewSites(c *ui.Context) {
	t := c.Theme()
	Panel(c).Grow(1).Children(func() {
		if PanelTitle(c, icApps, TintIndigo, s.text("sites"), fmt.Sprintf("%d", len(s.Sites)), true, nil) {
			s.navigate("sites")
		}
		if len(s.Sites) == 0 {
			EmptyState(c, icApps, TintIndigo, s.text("start"), s.text("start_detail")).Children(func() {
				if AccentPillButton(c, icAdd, s.text("add_site"), false).Clicked() {
					s.navigate("sites")
					s.editSite(nil)
				}
			})
			return
		}
		for i, site := range s.Sites {
			if i == 5 {
				break
			}
			if i > 0 {
				Divider(c)
			}
			b := ui.ButtonBase(c.Key("ov-"+site.ID)).FillWidth().Label(site.Name).Padding(4, 0).Background(ui.Color{}).FocusRing(false)
			b.Children(func() {
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
					s.siteAvatar(c, site, 28)
					ui.Column(c).Grow(1).Basis(0).Gap(1).Children(func() {
						ui.Text(c, site.Name).FontSize(13).FontWeight(600).SingleLine()
						ui.Text(c, hostOf(site.BaseURL)).FontSize(11).TextColor(t.TextMuted).SingleLine()
					})
					if m := value(site.SelectedModelID); m != "" {
						ui.Text(c, m).FontSize(11).TextColor(t.TextMuted).SingleLine().MaxWidth(160)
					}
					if site.Enabled {
						DotPill(c, s.text("enabled"), t.Success)
					} else {
						DotPill(c, s.text("disabled"), TintGray.C(t))
					}
				})
			})
			if b.Clicked() {
				s.selectSite(site.ID)
				s.navigate("sites")
			}
		}
	})
}
func (s *Shell) overviewBackups(c *ui.Context) {
	t := c.Theme()
	o := s.BackupUI.Overview
	Panel(c).Grow(1).Children(func() {
		if PanelTitle(c, icHistory, TintGreen, s.text("backups"), "", true, nil) {
			s.navigate("backups")
		}
		latest := s.text("never")
		if o.LatestLocalBackupAt != nil {
			latest = time.UnixMilli(*o.LatestLocalBackupAt).Format("01-02 15:04")
		}
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, s.text("latest_backup")).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, latest).FontSize(22).FontWeight(700).TextColor(TintGreen.Text(t)).SingleLine()
		})
		webdav := s.text("webdav_not_configured")
		col := TintGray.C(t)
		if o.WebdavConfigured {
			webdav, col = s.text("webdav_configured"), t.Success
			if o.WebdavSync.Status == "failed" || o.WebdavSync.Error != nil {
				webdav, col = s.text("sync_failed"), t.Danger
			}
		}
		next := "—"
		if o.NextScheduledAt != nil {
			next = time.UnixMilli(*o.NextScheduledAt).Format("01-02 15:04")
		}
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "WebDAV").FontSize(12).TextColor(t.TextMuted).Grow(1)
			DotPill(c, webdav, col)
		})
		Metrics(c, Metric{Label: s.text("next_backup"), Value: next}, Metric{Label: s.text("max_backups"), Value: fmt.Sprintf("%d", s.Settings.MaxBackupCopies)})
	})
}
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}
