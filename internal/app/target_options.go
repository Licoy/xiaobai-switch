package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func (s *Shell) targetOptions(c *ui.Context, d *targetDraft, models []domain.SiteModelDTO) {
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		switch d.Request.Target {
		case domain.ClaudeCode:
			s.claudeOptions(c, d)
		case domain.Codex:
			s.codexOptions(c, d)
		case domain.Pi, domain.Prime:
			s.thinkingOptions(c, d, models)
		}
	})
}

// optionRows is an inset list of setting rows inside a panel
func optionRows(c *ui.Context, rows func(row func(label, detail string, control func()))) {
	ui.Column(c).FillWidth().Gap(0).Padding(0, 14).Radius(12).Background(pal(c.Theme()).Inset).Children(func() {
		first := true
		rows(func(label, detail string, control func()) {
			if !first {
				Divider(c)
			}
			first = false
			settingRow(c, label, detail, control)
		})
	})
}
func (s *Shell) levelLabels(values []string) []string {
	labels := make([]string, len(values))
	for i, v := range values {
		labels[i] = v
		if v == "" {
			labels[i] = s.text("default")
		}
	}
	return labels
}
func (s *Shell) claudeOptions(c *ui.Context, d *targetDraft) {
	optionRows(c, func(row func(label, detail string, control func())) {
		row(s.text("auth_style"), s.text("auth_style_detail"), func() {
			stringTabs(c.Key("auth"), &d.Auth, []string{"anthropic_auth_token", "anthropic_api_key"}, "AUTH_TOKEN", "API_KEY")
		})
		efforts := []string{"", "low", "medium", "high", "xhigh"}
		row(s.text("effort"), "", func() { stringTabs(c.Key("effort"), &d.Effort, efforts, s.levelLabels(efforts)...) })
		row(s.text("one_million"), s.text("one_million_detail"), func() {
			ui.Switch(c.Key("1m"), &d.Request.Claude.Use1mContext).Label(s.text("one_million"))
		})
	})
	ui.Text(c, s.text("model_aliases")).FontSize(13).FontWeight(600)
	aliases := []struct {
		label string
		v     *string
	}{{"Fable", &d.Fable}, {"Opus", &d.Opus}, {"Sonnet", &d.Sonnet}, {"Haiku", &d.Haiku}}
	grid(c, 2, len(aliases), 12, func(i int) {
		item := aliases[i]
		field(c, item.label, func() {
			ui.TextInput(c.Key(item.label), item.v).FillWidth().Placeholder(s.text("alias_placeholder")).Label(item.label)
		})
	})
}
func (s *Shell) codexOptions(c *ui.Context, d *targetDraft) {
	if d.CapabilitySource == "site" {
		for _, site := range s.Sites {
			if site.ID == d.Request.SiteID {
				d.Request.Codex.RemoteCompaction = site.Capabilities["codex-compact"]
				d.Request.Codex.ImageUnderstanding = site.Capabilities["codex-vision"]
				d.Request.Codex.ImageGeneration = site.Capabilities["codex-imagegen"]
				d.Request.Codex.WebSearch = site.Capabilities["codex-search"]
			}
		}
	}
	optionRows(c, func(row func(label, detail string, control func())) {
		row(s.text("write_all_models"), s.text("write_all_models_detail"), func() {
			ui.Switch(c.Key("all"), &d.Request.Codex.WriteAllModels).Label(s.text("write_all_models"))
		})
		efforts := []string{"", "minimal", "low", "medium", "high", "xhigh"}
		row(s.text("effort"), "", func() { stringTabs(c.Key("reasoning"), &d.Reasoning, efforts, s.levelLabels(efforts)...) })
		row(s.text("capability_source"), "", func() {
			stringTabs(c.Key("source"), &d.CapabilitySource, []string{"site", "custom"}, s.text("capability_site"), s.text("capability_custom"))
		})
	})
	ui.Text(c, s.text("codex_capabilities")).FontSize(13).FontWeight(600)
	values := []*bool{&d.Request.Codex.RemoteCompaction, &d.Request.Codex.ImageUnderstanding, &d.Request.Codex.ImageGeneration, &d.Request.Codex.WebSearch}
	ui.Column(c).FillWidth().Disabled(d.CapabilitySource == "site").Children(func() {
		optionRows(c, func(row func(label, detail string, control func())) {
			for i, key := range capabilityKeys {
				row(s.text(key), key, func() { ui.Switch(c.Key(key), values[i]).Label(s.text(key)) })
			}
		})
	})
}
func (s *Shell) thinkingOptions(c *ui.Context, d *targetDraft, models []domain.SiteModelDTO) {
	t := c.Theme()
	writeAll := &d.Request.Pi.WriteAllModels
	if d.Request.Target == domain.Prime {
		writeAll = &d.Request.Prime.WriteAllModels
	}
	levels := []string{"", "off", "minimal", "low", "medium", "high", "xhigh", "max"}
	levelChoices := []choice{}
	for i, l := range s.levelLabels(levels) {
		levelChoices = append(levelChoices, choice{levels[i], l})
	}
	optionRows(c, func(row func(label, detail string, control func())) {
		row(s.text("write_all_models"), s.text("write_all_models_detail"), func() {
			ui.Switch(c.Key("all"), writeAll).Label(s.text("write_all_models"))
		})
		row(s.text("thinking_default"), "", func() {
			ui.Column(c).Width(180).Children(func() { choose(c.Key("default-thinking"), &d.DefaultThinking, levelChoices) })
		})
	})
	grid(c, 2, 2, 12, func(i int) {
		item := []struct {
			level string
			v     *string
		}{{"xhigh", &d.GlobalXhigh}, {"max", &d.GlobalMax}}[i]
		field(c, item.level+" · "+s.text("global_mapping"), func() { ui.TextInput(c.Key(item.level), item.v).FillWidth() })
	})
	if len(models) == 0 {
		return
	}
	ui.Text(c, s.text("model_thinking")).FontSize(13).FontWeight(600)
	protocol := ""
	for _, site := range s.Sites {
		if site.ID == d.Request.SiteID {
			protocol = site.Protocol
		}
	}
	for _, m := range models {
		cfg := d.ThinkingModels[m.ModelID]
		if cfg.ThinkingLevelMap == nil {
			cfg.ThinkingLevelMap = map[string]string{}
		}
		ui.Column(c.Key(m.ModelID)).FillWidth().Gap(10).Padding(12, 14).Radius(12).Background(pal(t).Inset).Children(func() {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				Tile(c, icBrain, TintPurple, 24, false)
				ui.Text(c, m.ModelID).FontSize(13).FontWeight(600).Grow(1).Basis(0).SingleLine()
				if d.Request.Target == domain.Pi && protocol == "anthropic" {
					ui.Text(c, s.text("adaptive")).FontSize(12).TextColor(t.TextMuted)
					ui.Switch(c.Key("adaptive"), &cfg.ForceAdaptiveThinking).Label(s.text("adaptive"))
				}
				ui.Text(c, s.text("reasoning_enabled")).FontSize(12).TextColor(t.TextMuted)
				ui.Switch(c.Key("reasoning"), &cfg.Reasoning).Label(s.text("reasoning_enabled"))
			})
			if !cfg.Reasoning {
				return
			}
			mapped := []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"}
			grid(c, 4, len(mapped), 8, func(i int) {
				level := mapped[i]
				mapping := cfg.ThinkingLevelMap[level]
				field(c, level, func() {
					if ui.TextInput(c.Key(level), &mapping).FillWidth().Placeholder(s.text("mapping")).Changed() {
						cfg.ThinkingLevelMap[level] = mapping
					}
				})
			})
		})
		d.ThinkingModels[m.ModelID] = cfg
	}
}
