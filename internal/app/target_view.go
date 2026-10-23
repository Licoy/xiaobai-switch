package app

import (
	"context"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/service"
	"path/filepath"
)

type targetDraft struct {
	Request                                                    domain.ApplyRequest
	Auth, Effort, Reasoning, CapabilitySource, DefaultThinking string
	Fable, Opus, Sonnet, Haiku                                 string
	GlobalXhigh, GlobalMax                                     string
	Binding                                                    *domain.TargetBinding
	OptionsOpen, FieldsOpen                                    bool
	ThinkingModels                                             map[string]domain.ModelThinkingConfig
	Status                                                     *service.TargetStatus
	Models                                                     []domain.SiteModelDTO
}

func targetForRoute(route string) domain.TargetKind {
	if route == "claude" {
		return domain.ClaudeCode
	}
	return domain.TargetKind(route)
}
func routeForTarget(t domain.TargetKind) string {
	if t == domain.ClaudeCode {
		return "claude"
	}
	return string(t)
}
func (s *Shell) draft(target domain.TargetKind) *targetDraft {
	if d := s.Drafts[target]; d != nil {
		return d
	}
	d := &targetDraft{Request: domain.ApplyRequest{Target: target}, CapabilitySource: "site", ThinkingModels: map[string]domain.ModelThinkingConfig{}}
	s.Drafts[target] = d
	if status, ok := s.Statuses[target]; ok {
		copy := status
		d.Status = &copy
	}
	binding, err := s.Store.GetBinding(target)
	if err != nil {
		s.result(err)
		return d
	}
	if binding != nil && binding.SiteID != nil {
		for _, site := range s.Sites {
			if site.Enabled && site.ID == *binding.SiteID {
				s.hydrateDraft(d, site)
				return d
			}
		}
	}
	for _, site := range s.Sites {
		if site.Enabled {
			s.hydrateDraft(d, site)
			break
		}
	}
	return d
}
func (s *Shell) hydrateDraft(d *targetDraft, site domain.SiteDTO) {
	request := domain.ApplyRequest{SiteID: site.ID, APIKeyID: value(site.ActiveAPIKeyID), Target: d.Request.Target, ModelID: value(site.SelectedModelID)}
	preset, err := s.Store.GetTargetPreset(site.ID, request.APIKeyID, request.Target)
	if err != nil {
		s.result(err)
		return
	}
	if preset != nil {
		request = preset.Options
	}
	models, err := s.Store.ListModelsForKey(site.ID, request.APIKeyID)
	if err != nil {
		s.result(err)
		return
	}
	d.Models = models
	binding, err := s.Store.GetBinding(request.Target)
	if err != nil {
		s.result(err)
		return
	}
	d.Binding = binding
	if preset == nil && binding != nil && value(binding.SiteID) == site.ID {
		request.ModelID = binding.ModelID
	}
	d.Request = request
	d.Auth = site.ClaudeAuthKeyStyle
	if request.Claude.AuthKeyStyle != nil {
		d.Auth = string(*request.Claude.AuthKeyStyle)
	}
	d.Fable = value(request.Claude.FableModelID)
	d.Opus = value(request.Claude.OpusModelID)
	d.Sonnet = value(request.Claude.SonnetModelID)
	d.Haiku = value(request.Claude.HaikuModelID)
	d.Effort = string(value(request.Claude.EffortLevel))
	d.Reasoning = string(value(request.Codex.ReasoningEffort))
	d.CapabilitySource = string(request.Codex.CapabilitySource)
	if d.CapabilitySource == "" {
		d.CapabilitySource = "site"
	}
	d.ThinkingModels = map[string]domain.ModelThinkingConfig{}
	d.DefaultThinking = ""
	if domain.IsThinkingTarget(request.Target) {
		p, err := s.Store.GetThinkingPreset(site.ID, request.Target)
		if err != nil {
			s.result(err)
			return
		}
		d.ThinkingModels = p.Models
		d.DefaultThinking = value(p.DefaultLevel)
		d.GlobalXhigh = value(p.Extended.Xhigh)
		d.GlobalMax = value(p.Extended.Max)
	}
	s.hydrateLiveOptions(d)
}
func (s *Shell) targetView(c *ui.Context, target domain.TargetKind, compact bool) {
	d := s.draft(target)
	if compact {
		s.compactStatus(c, d)
		s.draftForm(c, d, true)
		return
	}
	s.targetHero(c, d)
	s.liveFields(c, d)
	Panel(c).Children(func() {
		r := routeByID(routeForTarget(target))
		PanelTitle(c, icMagic, r.Tint, s.text("draft"), "", false, nil)
		s.draftForm(c, d, false)
	})
}
func (s *Shell) draftForm(c *ui.Context, d *targetDraft, compact bool) {
	t := c.Theme()
	options := []choice{}
	for _, site := range s.Sites {
		if site.Enabled {
			options = append(options, choice{site.ID, site.Name})
		}
	}
	if len(options) == 0 {
		EmptyState(c, icApps, TintIndigo, s.text("no_enabled_site"), "").Children(func() {
			if !compact && AccentPillButton(c, icApps, s.text("sites"), false).Clicked() {
				s.navigate("sites")
			}
		})
		return
	}
	ids := []string{}
	for _, m := range d.Models {
		ids = append(ids, m.ModelID)
	}
	n := 2
	if compact {
		n = 1
	}
	grid(c, n, 2, 12, func(i int) {
		if i == 0 {
			field(c, s.text("select_site"), func() {
				if choose(c, &d.Request.SiteID, options).Changed() {
					for _, site := range s.Sites {
						if site.ID == d.Request.SiteID {
							s.hydrateDraft(d, site)
						}
					}
				}
			})
			return
		}
		field(c, s.text("select_model"), func() { ui.Combobox(c, &d.Request.ModelID, ids).FillWidth().Label(s.text("select_model")) })
	})
	if compact {
		disclosure(c, "options", icSettings, s.text("show_options"), &d.OptionsOpen, func() { s.targetOptions(c, d, d.Models) })
	} else {
		s.targetOptions(c, d, d.Models)
	}
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		if !compact {
			ui.Icon(c, icLock).Size(14, 14).TextColor(t.TextMuted)
			ui.Text(c, s.text("secret_notice")).FontSize(11).TextColor(t.TextMuted).Grow(1).Basis(0).Wrap()
		} else {
			ui.Box(c).Grow(1)
		}
		if PillButton(c, icSave, s.text("save_preset"), d.Request.SiteID == "").Clicked() {
			if s.result(s.saveDraft(d)) {
				s.Notice = s.text("preset_saved")
			}
		}
		if AccentPillButton(c, icEye, s.text("preview"), s.Busy || d.Request.ModelID == "").Clicked() {
			s.prepareApply(d, compact)
		}
	})
}
func (s *Shell) buildRequest(d *targetDraft) domain.ApplyRequest {
	r := d.Request
	auth := domain.ClaudeAuthKeyStyle(d.Auth)
	r.Claude.AuthKeyStyle = &auth
	r.Claude.FableModelID = optional(d.Fable)
	r.Claude.OpusModelID = optional(d.Opus)
	r.Claude.SonnetModelID = optional(d.Sonnet)
	r.Claude.HaikuModelID = optional(d.Haiku)
	r.Claude.EffortLevel = nil
	if d.Effort != "" {
		v := domain.ClaudeEffortLevel(d.Effort)
		r.Claude.EffortLevel = &v
	}
	r.Codex.ReasoningEffort = nil
	if d.Reasoning != "" {
		v := domain.CodexReasoningEffort(d.Reasoning)
		r.Codex.ReasoningEffort = &v
	}
	r.Codex.CapabilitySource = domain.CapabilitySource(d.CapabilitySource)
	catalog := []domain.CatalogModel{}
	models := d.Models
	for _, m := range models {
		catalog = append(catalog, domain.CatalogModel{ModelID: m.ModelID, DisplayName: m.DisplayName})
	}
	r.Codex.CatalogModels = catalog
	r.Pi.CatalogModels = catalog
	r.Prime.CatalogModels = catalog
	thinkingModels := map[string]domain.ModelThinkingConfig{}
	for id, cfg := range d.ThinkingModels {
		mapping := map[string]string{}
		for k, v := range cfg.ThinkingLevelMap {
			mapping[k] = v
		}
		cfg.ThinkingLevelMap = mapping
		thinkingModels[id] = cfg
	}
	thinking := domain.ThinkingWrite{DefaultLevel: optional(d.DefaultThinking), Extended: domain.ThinkingExtendedMap{Xhigh: optional(d.GlobalXhigh), Max: optional(d.GlobalMax)}, Models: thinkingModels}
	r.Pi.Thinking = thinking
	r.Prime.Thinking = thinking
	return r
}
func (s *Shell) saveDraft(d *targetDraft) error {
	if s.RestoreReady {
		return domain.NewError("restore_pending", s.text("restore_queued"))
	}
	r := s.buildRequest(d)
	if domain.IsThinkingTarget(r.Target) {
		if err := s.Service.SaveThinkingPreset(s.Context, domain.SiteThinkingPreset{SiteID: r.SiteID, Target: r.Target, DefaultLevel: optional(d.DefaultThinking), Extended: domain.ThinkingExtendedMap{Xhigh: optional(d.GlobalXhigh), Max: optional(d.GlobalMax)}, Models: d.ThinkingModels}); err != nil {
			return err
		}
	}
	return s.Service.SaveTargetPreset(s.Context, domain.TargetPreset{SiteID: r.SiteID, APIKeyID: r.APIKeyID, Target: r.Target, Options: r})
}
func (s *Shell) targetHero(c *ui.Context, d *targetDraft) {
	t := c.Theme()
	target := d.Request.Target
	r := routeByID(routeForTarget(target))
	Panel(c).Children(func() {
		ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
			targetTile(c, target, 48)
			ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
				ui.Text(c, targetLabel(target)).FontSize(18).FontWeight(700).TextColor(r.Tint.Text(t)).SingleLine()
				path := s.text("status_pending")
				if d.Status != nil {
					path = d.Status.ConfigPath
				}
				mono(c, path).TextColor(t.TextMuted).SingleLine().Selectable()
			})
		})
		site, model, key, cli := "—", "—", "—", "—"
		cliColor := t.TextMuted
		if d.Binding != nil {
			site, model = d.Binding.SiteNameSnapshot, d.Binding.ModelID
			if d.Binding.APIKey.Prefix != nil {
				key = *d.Binding.APIKey.Prefix
			}
		}
		if d.Status != nil {
			switch {
			case d.Status.CLIRefreshing:
				cli = s.text("cli_detecting")
			case d.Status.Installed:
				cli, cliColor = value(d.Status.Version), t.Success
				if cli == "" {
					cli = s.text("cli_installed")
				}
			default:
				cli, cliColor = s.text("cli_missing"), t.Warning
			}
		}
		Metrics(c, Metric{Label: s.text("site"), Value: site}, Metric{Label: s.text("current_model"), Value: model, Color: r.Tint.Text(t)}, Metric{Label: "API key", Value: key}, Metric{Label: "CLI", Value: cli, Color: cliColor})
		if d.Status != nil && d.Status.CLIError != nil {
			Banner(c, icWarning, t.Warning, s.text("cli_detection_error"))
		}
		if d.Status != nil {
			for _, difference := range d.Status.Differences {
				Banner(c, icWarning, t.Warning, difference)
			}
		}
		Divider(c)
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			if PillButton(c, icRevert, s.text("revert"), s.Busy).Clicked() {
				s.restoreTarget(target, false)
			}
			if PillButton(c, icRestart, s.text("restore_official"), s.Busy).Clicked() {
				s.restoreTarget(target, true)
			}
			if d.Status != nil && d.Status.Status == domain.Orphan && DangerPillButton(c, icDelete, s.text("cleanup_orphan"), s.Busy).Clicked() {
				s.confirmDanger(s.text("cleanup_orphan"), targetLabel(target), func() error {
					s.serviceChange(func(ctx context.Context) error { return s.Service.CleanupOrphan(ctx, target) })
					return nil
				})
			}
			ui.Box(c).Grow(1)
			if d.Status != nil && PillButton(c, icFolder, s.text("open_config_dir"), false).Clicked() {
				s.openDirectory(filepath.Dir(d.Status.ConfigPath))
			}
		})
	})
}

type choice struct{ ID, Label string }

// choose is a select showing labels for ids, in the look of the capsule controls
func choose(c *ui.Context, selected *string, options []choice) ui.Element {
	t := c.Theme()
	sel := ui.SelectBase(c, selected)
	label := ""
	for _, item := range options {
		if item.ID == *selected {
			label = item.Label
		}
	}
	bg := t.Text.Alpha(0.06)
	if sel.Trigger.Hovered() {
		bg = t.Text.Alpha(0.1)
	}
	sel.Trigger.FillWidth().Height(34).Padding(0, 10, 0, 12).Gap(8).Radius(10).Background(bg).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(13).SingleLine().Grow(1).Basis(0)
		ui.Icon(c, icDown).Size(14, 14).TextColor(t.TextMuted)
	})
	sel.Popup(func(panel ui.Element) {
		panel.MaxHeight(280).Padding(4).Gap(2).Background(t.Background).Border(1, t.Border).Radius(10).Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.18))
		for _, option := range options {
			item := sel.Item(option.ID).Padding(7, 10).Radius(7)
			col := t.Text
			switch {
			case item.Highlighted():
				item.Background(t.Accent)
				col = t.AccentText
			case option.ID == *selected:
				item.Background(t.Accent.Alpha(0.1))
			}
			item.Children(func() { ui.Text(c, option.Label).FontSize(13).TextColor(col).SingleLine() })
		}
	})
	return sel.Trigger
}
