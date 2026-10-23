package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

const siteListWidth = 272

func (s *Shell) sitesView(c *ui.Context) {
	ui.Row(c).Grow(1).FillWidth().Padding(6, 24, 0, 24).Gap(16).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(siteListWidth).Shrink(0).Gap(10).Padding(0, 0, 16, 0).Children(func() { s.siteList(c) })
		ui.Scroll(c.Key("site-detail")).Grow(1).Basis(0).Children(func() {
			ui.Column(c).FillWidth().Gap(14).Padding(0, 0, 24, 0).Children(func() {
				if site := s.site(); site != nil {
					s.siteDetail(c, *site)
				} else {
					Panel(c).Children(func() {
						EmptyState(c, icApps, TintIndigo, s.text("start"), s.text("start_detail")).Children(func() {
							if AccentPillButton(c, icAdd, s.text("add_site"), false).Clicked() {
								s.editSite(nil)
							}
						})
					})
				}
			})
		})
	})
}
func (s *Shell) siteList(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(8).Padding(0, 12).Height(34).Radius(10).Background(pal(t).Card).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icSearch).Size(15, 15).TextColor(t.TextMuted)
		ui.TextInput(c.Key("search"), &s.Search).Placeholder(s.text("search")).Grow(1).Background(ui.Color{}).Border(0, ui.Color{}).Padding(0)
	})
	ui.Row(c.Key("site-filter")).FillWidth().Children(func() {
		PillTabs(c, &s.SiteFilter, s.text("filter_all"), s.text("enabled"), s.text("disabled"))
	})
	ui.Scroll(c.Key("site-list")).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			shown := 0
			for _, site := range s.Sites {
				if !strings.Contains(strings.ToLower(site.Name+" "+site.BaseURL), strings.ToLower(s.Search)) {
					continue
				}
				if (s.SiteFilter == 1 && !site.Enabled) || (s.SiteFilter == 2 && site.Enabled) {
					continue
				}
				shown++
				s.siteRow(c, site)
			}
			if shown == 0 {
				ui.Text(c, s.text("no_sites_match")).FontSize(12).TextColor(t.TextMuted).Padding(16, 8)
			}
		})
	})
}
func (s *Shell) siteRow(c *ui.Context, site domain.SiteDTO) {
	t := c.Theme()
	sel := site.ID == s.Selected
	b := ui.ButtonBase(c.Key(site.ID)).FillWidth().Padding(9, 10).Gap(10).Radius(12).Label(site.Name).AlignItems(ui.Center).FocusRing(false)
	switch {
	case sel:
		b.Background(t.Accent.Alpha(0.12))
	case b.Hovered():
		b.Background(pal(t).Hover)
	default:
		b.Background(ui.Color{})
	}
	b.Children(func() {
		s.siteAvatar(c, site, 34)
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			col := t.Text
			if sel {
				col = t.Accent
			}
			ui.Text(c, site.Name).FontSize(13).FontWeight(600).TextColor(col).SingleLine()
			ui.Text(c, hostOf(site.BaseURL)).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
		dot := t.Success
		if !site.Enabled {
			dot = TintGray.C(t).Alpha(0.6)
		}
		ui.Box(c).Size(7, 7).Radius(4).Background(dot).Tooltip(map[bool]string{true: s.text("enabled"), false: s.text("disabled")}[site.Enabled])
	})
	if b.Clicked() {
		s.selectSite(site.ID)
	}
}
func (s *Shell) siteDetail(c *ui.Context, site domain.SiteDTO) {
	s.siteHero(c, site)
	if columns(s.contentWidth(c)-siteListWidth-16, 280, 2) == 2 {
		ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Children(func() { s.quotaPanel(c, site) })
			ui.Column(c).Grow(1).Basis(0).Children(func() { s.routePanel(c, site) })
		})
	} else {
		ui.Column(c).FillWidth().Children(func() { s.quotaPanel(c, site) })
		ui.Column(c).FillWidth().Children(func() { s.routePanel(c, site) })
	}
	s.keysPanel(c, site)
	s.modelsPanel(c, site)
	s.applyToPanel(c, site)
	if notes := value(site.Notes); notes != "" {
		Panel(c).Children(func() {
			PanelTitle(c, icEdit, TintGray, s.text("notes"), "", false, nil)
			ui.Text(c, notes).FontSize(13).Wrap().Selectable()
		})
	}
	Banner(c, icLock, TintOrange.C(c.Theme()), s.text("secret_notice"))
}
func (s *Shell) siteHero(c *ui.Context, site domain.SiteDTO) {
	t := c.Theme()
	index := 0
	for i := range s.Sites {
		if s.Sites[i].ID == site.ID {
			index = i
		}
	}
	Panel(c).Children(func() {
		ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
			s.siteAvatar(c, site, 48)
			ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, site.Name).FontSize(18).FontWeight(700).SingleLine()
					Pill(c, s.protocolLabel(site.Protocol), TintIndigo.Text(t))
				})
				ui.Text(c, site.BaseURL).FontSize(12).TextColor(t.TextMuted).SingleLine().Selectable()
			})
			enabled := site.Enabled
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if site.Enabled {
					DotPill(c, s.text("enabled"), t.Success)
				} else {
					DotPill(c, s.text("disabled"), TintGray.C(t))
				}
				if ui.Switch(c.Key("site-enabled"), &enabled).Label(s.text("enabled")).Changed() {
					s.enable(site.ID, enabled)
				}
			})
		})
		model := value(site.SelectedModelID)
		if model == "" {
			model = "—"
		}
		Metrics(c,
			Metric{Label: s.text("default_model"), Value: model, Color: TintPurple.Text(t)},
			Metric{Label: s.text("models"), Value: fmt.Sprintf("%d", len(s.Models))},
			Metric{Label: s.text("api_keys"), Value: fmt.Sprintf("%d", len(site.APIKeys))},
			Metric{Label: s.text("routes"), Value: fmt.Sprintf("%d", len(site.BaseURLs))},
		)
		Divider(c)
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			if PillButton(c, icEdit, s.text("edit"), false).Clicked() {
				s.editSite(&site)
			}
			if PillButton(c, icUp, "", index == 0).Label(s.text("move_up")).Tooltip(s.text("move_up")).Clicked() {
				s.reorder(site.ID, -1)
			}
			if PillButton(c, icDown, "", index == len(s.Sites)-1).Label(s.text("move_down")).Tooltip(s.text("move_down")).Clicked() {
				s.reorder(site.ID, 1)
			}
			ui.Box(c).Grow(1)
			if DangerPillButton(c, icDelete, s.text("delete"), s.Busy).Clicked() {
				s.deleteSite(site)
			}
		})
	})
}
func (s *Shell) protocolLabel(protocol string) string {
	if protocol == "anthropic" {
		return "Anthropic"
	}
	return "OpenAI"
}
func (s *Shell) keysPanel(c *ui.Context, site domain.SiteDTO) {
	t := c.Theme()
	Panel(c).Children(func() {
		PanelTitle(c, icKey, TintYellow, s.text("api_keys"), fmt.Sprintf("%d", len(site.APIKeys)), false, func() {
			if PillButton(c, icAdd, s.text("add_key"), false).Clicked() {
				s.editKey(site, domain.SiteApiKeySummary{})
			}
		})
		ui.Column(c).FillWidth().Gap(0).Padding(2, 12).Radius(12).Background(pal(t).Inset).Children(func() {
			for i, key := range site.APIKeys {
				if i > 0 {
					Divider(c)
				}
				s.keyRow(c, site, key)
			}
		})
	})
}
func (s *Shell) keyRow(c *ui.Context, site domain.SiteDTO, key domain.SiteApiKeySummary) {
	t := c.Theme()
	ui.Row(c.Key(key.ID)).FillWidth().Gap(10).Padding(8, 0).AlignItems(ui.Center).Children(func() {
		k := TintGray
		if key.IsActive {
			k = TintYellow
		}
		Tile(c, icKey, k, 28, false)
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, key.Label).FontSize(13).FontWeight(600).SingleLine()
				if key.IsActive {
					DotPill(c, s.text("active"), t.Success)
				}
			})
			mono(c, key.KeyPrefix).TextColor(t.TextMuted)
		})
		if !key.IsActive && PillButton(c, icCheck, s.text("activate"), s.Busy).Clicked() {
			s.activateKey(site.ID, key.ID)
		}
		if PillButton(c, icEdit, "", false).Label(s.text("edit_key")).Tooltip(s.text("edit_key")).Clicked() {
			s.editKey(site, key)
		}
		if DangerPillButton(c, icDelete, "", len(site.APIKeys) < 2).Label(s.text("delete_key")).Tooltip(s.text("delete_key")).Clicked() {
			s.confirmDanger(s.text("delete_key"), key.Label+" · "+key.KeyPrefix, func() error {
				s.serviceChange(func(ctx context.Context) error { return s.Service.DeleteSiteAPIKey(ctx, site.ID, key.ID) })
				return nil
			})
		}
	})
}
func (s *Shell) modelsPanel(c *ui.Context, site domain.SiteDTO) {
	t := c.Theme()
	Panel(c).Children(func() {
		PanelTitle(c, icModel, TintPurple, s.text("models"), fmt.Sprintf("%d", len(s.Models)), false, func() {
			if BusyPillButton(c, icDownload, s.text("fetch_models"), false, s.Busy).Clicked() {
				s.fetchModels(site)
			}
			if PillButton(c, icPulse, s.text("probe_key"), s.Busy).Clicked() {
				s.probeKey(site)
			}
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).Basis(0).Gap(8).Padding(0, 12).Height(32).Radius(10).Background(pal(t).Inset).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icSearch).Size(14, 14).TextColor(t.TextMuted)
				ui.TextInput(c.Key("model_search"), &s.ModelSearch).Placeholder(s.text("search_models")).Grow(1).Background(ui.Color{}).Border(0, ui.Color{}).Padding(0)
			})
			ui.Row(c).Grow(1).Basis(0).Gap(8).Padding(0, 4, 0, 12).Height(32).Radius(10).Background(pal(t).Inset).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c.Key("model_id"), &s.ModelInput).Placeholder(s.text("model_id")).Grow(1).Background(ui.Color{}).Border(0, ui.Color{}).Padding(0)
				if PillButton(c, icAdd, s.text("add_model"), strings.TrimSpace(s.ModelInput) == "").Clicked() {
					id := strings.TrimSpace(s.ModelInput)
					s.mutate(func() error { return s.Service.SetSelectedModel(s.Context, site.ID, value(site.ActiveAPIKeyID), id) })
					s.ModelInput = ""
				}
			})
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			if s.BatchCancel != nil {
				ui.Text(c, fmt.Sprintf("%s %d / %d", s.text("batch_probe"), s.BatchProgress, s.BatchTotal)).FontSize(12).TextColor(t.TextMuted).SingleLine()
				ui.Column(c).Grow(1).Children(func() { Bar(c, float64(s.BatchProgress)/float64(max(1, s.BatchTotal)), TintPurple.C(t)) })
				if PillButton(c, icClose, s.text("cancel"), false).Clicked() {
					s.BatchCancel()
				}
				return
			}
			if PillButton(c, icFlash, s.text("batch_probe"), s.Busy || len(s.Models) == 0).Clicked() {
				s.batchProbe(site)
			}
			ui.Box(c).Grow(1)
			if DangerPillButton(c, icDelete, s.text("clear_models"), len(s.Models) == 0).Clicked() {
				s.confirmDanger(s.text("clear_models"), site.Name, func() error {
					if err := s.Service.ClearModels(s.Context, site.ID); err != nil {
						return err
					}
					return s.reload()
				})
			}
		})
		if site.LastModelFetchError != nil {
			Banner(c, icWarning, t.Warning, s.safeErrorDetail(*site.LastModelFetchError))
		}
		if len(s.Models) == 0 {
			EmptyState(c, icModel, TintPurple, s.text("no_models"), s.text("no_models_detail"))
			return
		}
		ui.Column(c).FillWidth().Gap(0).Padding(2, 12).Radius(12).Background(pal(t).Inset).Children(func() {
			first := true
			for _, model := range s.Models {
				if !strings.Contains(strings.ToLower(model.ModelID+" "+model.DisplayName), strings.ToLower(s.ModelSearch)) {
					continue
				}
				if !first {
					Divider(c)
				}
				first = false
				s.modelRow(c, site, model)
			}
		})
	})
}
func (s *Shell) modelRow(c *ui.Context, site domain.SiteDTO, model domain.SiteModelDTO) {
	t := c.Theme()
	isDefault := model.ModelID == value(site.SelectedModelID)
	result, probed := s.ModelProbes[probeResultKey(site, model.ModelID)]
	ui.Column(c.Key(model.ID)).FillWidth().Gap(4).Padding(7, 0).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, model.ModelID).FontSize(13).FontWeight(500).SingleLine().Grow(1).Basis(0).Selectable()
			if isDefault {
				Pill(c, s.text("default"), t.Accent)
			}
			if model.IsManual {
				Pill(c, s.text("manual"), TintGray.C(t))
			}
			if probed {
				if result.Ok {
					DotPill(c, fmt.Sprintf("%d ms", result.LatencyMS), latencyColor(t, result.LatencyMS))
				} else {
					DotPill(c, s.text("error_probe_failed"), t.Danger)
				}
			}
			if !isDefault && PillButton(c, icStar, "", false).Label(s.text("set_default")).Tooltip(s.text("set_default")).Clicked() {
				s.mutate(func() error {
					return s.Service.SetSelectedModel(s.Context, site.ID, value(site.ActiveAPIKeyID), model.ModelID)
				})
			}
			if PillButton(c, icPulse, "", s.Busy).Label(s.text("probe")).Tooltip(s.text("probe")).Clicked() {
				s.probeModel(site, model.ModelID)
			}
			if DangerPillButton(c, icClose, "", false).Label(s.text("remove")).Tooltip(s.text("remove")).Clicked() {
				s.mutate(func() error { return s.Service.DeleteModel(s.Context, site.ID, model.ModelID) })
			}
		})
		if probed && result.Error != nil {
			ui.Text(c, s.safeErrorDetail(*result.Error)).FontSize(11).TextColor(t.Danger).MaxLines(2)
		}
	})
}
func (s *Shell) applyToPanel(c *ui.Context, site domain.SiteDTO) {
	t := c.Theme()
	Panel(c).Children(func() {
		PanelTitle(c, icSend, TintBlue, s.text("apply_to"), "", false, nil)
		n := columns(s.contentWidth(c)-siteListWidth-52, 120, 4)
		grid(c, n, len(targetKinds), 10, func(i int) {
			target := targetKinds[i]
			r := routeByID(routeForTarget(target))
			d := s.draft(target)
			bound := d.Binding != nil && value(d.Binding.SiteID) == site.ID
			b := ui.ButtonBase(c.Key("apply-" + r.ID)).FillWidth().Label(targetLabel(target)).Padding(12).Gap(8).Radius(12).Background(pal(t).Inset).FocusRing(false).Disabled(!site.Enabled)
			if b.Hovered() {
				b.Background(r.Tint.C(t).Alpha(0.12))
			}
			b.Children(func() {
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					targetTile(c, target, 30)
					ui.Text(c, targetLabel(target)).FontSize(13).FontWeight(600).SingleLine()
					if bound {
						DotPill(c, s.text("in_use"), t.Success)
					} else {
						ui.Text(c, s.text("configure")).FontSize(11).TextColor(t.TextMuted)
					}
				})
			})
			if b.Clicked() {
				s.hydrateDraft(d, site)
				s.navigate(r.ID)
			}
		})
		if !site.Enabled {
			ui.Text(c, s.text("site_disabled_apply")).FontSize(11).TextColor(t.TextMuted)
		}
	})
}
func countMessage(label string, n int) string { return fmt.Sprintf("%s · %d", label, n) }
