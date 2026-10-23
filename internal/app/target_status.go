package app

import (
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"sort"
	"strings"
)

func liveField(d *targetDraft, suffix string) string {
	if d.Status == nil {
		return ""
	}
	for key, raw := range d.Status.Fields {
		if strings.HasSuffix(key, suffix) {
			var text string
			if json.Unmarshal([]byte(raw), &text) == nil {
				return text
			}
			return raw
		}
	}
	return ""
}
func (s *Shell) hydrateLiveOptions(d *targetDraft) {
	if d.Binding == nil || value(d.Binding.SiteID) != d.Request.SiteID || d.Status == nil {
		return
	}
	if d.Request.Target == domain.ClaudeCode {
		read := func(name string) string { return liveField(d, "settings.json:env/"+name) }
		model := liveField(d, "settings.json:model")
		if env := read("ANTHROPIC_MODEL"); env != "" {
			model = env
		}
		if read("ANTHROPIC_AUTH_TOKEN") != "" {
			d.Auth = string(domain.AnthropicAuthToken)
		} else if read("ANTHROPIC_API_KEY") != "" {
			d.Auth = string(domain.AnthropicAPIKey)
		}
		if model != "" {
			d.Request.ModelID = strings.TrimSuffix(model, "[1m]")
		}
		aliases := []struct {
			name string
			v    *string
		}{{"ANTHROPIC_DEFAULT_FABLE_MODEL", &d.Fable}, {"ANTHROPIC_DEFAULT_OPUS_MODEL", &d.Opus}, {"ANTHROPIC_DEFAULT_SONNET_MODEL", &d.Sonnet}, {"ANTHROPIC_DEFAULT_HAIKU_MODEL", &d.Haiku}}
		d.Request.Claude.Use1mContext = strings.HasSuffix(model, "[1m]")
		for _, item := range aliases {
			raw := read(item.name)
			*item.v = strings.TrimSuffix(raw, "[1m]")
			d.Request.Claude.Use1mContext = d.Request.Claude.Use1mContext || strings.HasSuffix(raw, "[1m]")
		}
		d.Effort = liveField(d, "settings.json:effortLevel")
	} else if d.Request.Target == domain.Codex {
		if model := liveField(d, "config.toml:model"); model != "" {
			d.Request.ModelID = model
		}
		d.Reasoning = liveField(d, "config.toml:model_reasoning_effort")
	}
}
func (s *Shell) compactStatus(c *ui.Context, d *targetDraft) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(4).Padding(10, 12).Radius(12).Background(pal(t).Inset).Children(func() {
		if d.Binding == nil {
			ui.Text(c, s.text("status_not_applied")).FontSize(12).TextColor(t.TextMuted)
			return
		}
		ui.Text(c, d.Binding.ModelID).FontSize(14).FontWeight(700).SingleLine()
		line := d.Binding.SiteNameSnapshot
		if d.Binding.APIKey.Prefix != nil {
			line += " · " + *d.Binding.APIKey.Prefix
		}
		ui.Text(c, line).FontSize(11).TextColor(t.TextMuted).SingleLine()
	})
}
func (s *Shell) liveFields(c *ui.Context, d *targetDraft) {
	if d.Status == nil || len(d.Status.Fields) == 0 {
		return
	}
	t := c.Theme()
	keys := []string{}
	for key := range d.Status.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	Panel(c).Gap(10).Children(func() {
		parts := ui.CollapsibleBase(c.Key("live-fields"), &d.FieldsOpen)
		parts.Trigger.FillWidth().Gap(10).AlignItems(ui.Center).Label(s.text("config_fields")).Children(func() {
			Tile(c, icCode, TintGray, 30, false)
			ui.Text(c, s.text("config_fields")).FontSize(16).FontWeight(600).TextColor(TintGray.Text(t)).Grow(1)
			ui.Text(c, fmt.Sprintf("%d", len(keys))).FontSize(12).TextColor(t.TextMuted)
			ui.Icon(c, icDown).Size(16, 16).TextColor(t.TextMuted).Rotate(-90 + 90*parts.Progress())
		})
		parts.Panel(func() {
			ui.Column(c).FillWidth().Gap(0).Padding(2, 12).Radius(12).Background(pal(t).Inset).Children(func() {
				for i, key := range keys {
					if i > 0 {
						Divider(c)
					}
					ui.Row(c.Key(key)).FillWidth().Gap(12).Padding(6, 0).AlignItems(ui.Start).Children(func() {
						mono(c, key).TextColor(t.TextMuted).Width(220).Shrink(0).Wrap()
						mono(c, d.Status.Fields[key]).Grow(1).Basis(0).Wrap().Selectable()
					})
				}
			})
		}).Padding(10, 0, 0, 0)
	})
}
