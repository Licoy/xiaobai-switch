package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"strings"
)

type siteEditor struct {
	URLPreviewOpen                             bool
	ID, Name, URLs, Key, Notes, Protocol, Auth string
	ExtraKeys                                  []string
	Advanced, CapabilitiesOpen, Open           bool
	Capabilities                               domain.SiteCapabilities
}
type keyEditor struct {
	SiteID, ID, Label, Key string
	Open, Reveal           bool
}

func (s *Shell) editSite(site *domain.SiteDTO) {
	e := &siteEditor{Open: true, Protocol: string(domain.OpenAICompatible), Auth: string(domain.AnthropicAuthToken), Capabilities: domain.SiteCapabilities{}}
	if site != nil {
		e.ID = site.ID
		e.Name = site.Name
		e.URLs = strings.Join(site.BaseURLs, "\n")
		e.Notes = value(site.Notes)
		e.Protocol = site.Protocol
		e.Auth = site.ClaudeAuthKeyStyle
		for k, v := range site.Capabilities {
			e.Capabilities[k] = v
		}
	}
	s.Editor = e
	s.Error = ""
}
func lines(text string) []string {
	out := []string{}
	for _, line := range strings.Split(text, "\n") {
		if v := strings.TrimSpace(line); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func (s *Shell) clearEditor() {
	if s.Editor != nil {
		s.Editor.Key = ""
		for i := range s.Editor.ExtraKeys {
			s.Editor.ExtraKeys[i] = ""
		}
		s.Editor.ExtraKeys = nil
		s.Error, s.ErrorDetail = "", ""
	}
	s.Editor = nil
}
func (s *Shell) siteEditorView(c *ui.Context) {
	e := s.Editor
	t := c.Theme()
	dialog := ui.Modal(c.Key("site_editor"), &e.Open, func() {
		ui.Column(c).Width(modalWidth(false) + 20).Gap(16).Children(func() {
			title, icon := "edit_site", icEdit
			if e.ID == "" {
				title, icon = "add_site", icAdd
			}
			dialogHeader(c, icon, TintIndigo, s.text(title))
			ui.Scroll(c).MaxHeight(440).Children(func() {
				ui.Column(c).FillWidth().Gap(14).Padding(0, 4, 0, 0).Children(func() {
					field(c, s.text("name"), func() {
						ui.TextInput(c.Key("name"), &e.Name).FillWidth().Placeholder("Example Relay").Label(s.text("name"))
					})
					field(c, s.text("base_urls"), func() {
						ui.TextArea(c.Key("urls"), &e.URLs).Height(72).FillWidth().Placeholder("https://api.example.com").Label(s.text("base_urls"))
					})
					s.urlPreviewView(c, e.URLs, &e.URLPreviewOpen)
					if e.ID == "" {
						field(c, "API key", func() {
							ui.TextInput(c.Key("key"), &e.Key).Password().FillWidth().Placeholder("sk-…").Label("API key")
						})
						for i := range e.ExtraKeys {
							ui.Row(c.Key(i)).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
								ui.TextInput(c, &e.ExtraKeys[i]).Password().Grow(1).Basis(0).Placeholder(s.text("extra_key")).Label(s.text("extra_key"))
								if DangerPillButton(c, icDelete, "", false).Label(s.text("delete")).Clicked() {
									e.ExtraKeys[i] = ""
									e.ExtraKeys = append(e.ExtraKeys[:i], e.ExtraKeys[i+1:]...)
								}
							})
						}
						ui.Row(c).Children(func() {
							if PillButton(c, icKey, s.text("add_key"), false).Clicked() {
								e.ExtraKeys = append(e.ExtraKeys, "")
							}
						})
					}
					disclosure(c, "advanced", icSettings, s.text("advanced"), &e.Advanced, func() {
						field(c, s.text("protocol"), func() {
							stringTabs(c.Key("protocol"), &e.Protocol, []string{"openai_compatible", "anthropic"}, s.protocolLabel("openai_compatible"), s.protocolLabel("anthropic"))
						})
						field(c, s.text("auth_style"), func() {
							stringTabs(c.Key("auth"), &e.Auth, []string{"anthropic_auth_token", "anthropic_api_key"}, "AUTH_TOKEN", "API_KEY")
						})
						field(c, s.text("notes"), func() { ui.TextArea(c.Key("notes"), &e.Notes).Height(60).FillWidth().Label(s.text("notes")) })
					})
					disclosure(c, "capabilities", icMagic, s.text("codex_capabilities"), &e.CapabilitiesOpen, func() {
						optionRows(c, func(row func(label, detail string, control func())) {
							for _, key := range capabilityKeys {
								row(s.text(key), key, func() {
									on := e.Capabilities[key]
									if ui.Switch(c.Key(key), &on).Label(s.text(key)).Changed() {
										e.Capabilities[key] = on
									}
								})
							}
						})
					})
				})
			})
			if s.Error != "" {
				Banner(c, icFailure, t.Danger, s.Error)
			}
			dialogButtons(c, func() {
				if PillButton(c, nil, s.text("cancel"), false).Clicked() {
					s.clearEditor()
				}
				if AccentPillButton(c, icSave, s.text("save"), s.Busy).Clicked() {
					s.saveSiteAsync()
				}
			})
		})
	})
	if dialog.Dismissed() || !e.Open {
		s.clearEditor()
	}
}
func (s *Shell) editKey(site domain.SiteDTO, key domain.SiteApiKeySummary) {
	s.KeyEditor = &keyEditor{SiteID: site.ID, ID: key.ID, Label: key.Label, Open: true}
	s.Error = ""
}
func (s *Shell) clearKeyEditor() {
	if s.KeyEditor != nil {
		s.KeyEditor.Key = ""
		s.Error, s.ErrorDetail = "", ""
	}
	s.KeyEditor = nil
}
func (s *Shell) keyEditorView(c *ui.Context) {
	e := s.KeyEditor
	t := c.Theme()
	dialog := ui.Modal(c.Key("key_editor"), &e.Open, func() {
		ui.Column(c).Width(modalWidth(false) - 40).Gap(16).Children(func() {
			dialogHeader(c, icKey, TintYellow, s.text("edit_key"))
			field(c, s.text("key_label"), func() { ui.TextInput(c.Key("label"), &e.Label).FillWidth().Label(s.text("key_label")) })
			field(c, "API key", func() {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					input := ui.TextInput(c.Key("key"), &e.Key).Grow(1).Basis(0).Label("API key")
					if !e.Reveal {
						input.Password()
					}
					icon, label := icEye, s.text("show_key")
					if e.Reveal {
						icon = icEyeOff
					}
					if PillButton(c, icon, "", false).Label(label).Tooltip(label).Clicked() {
						e.Reveal = !e.Reveal
					}
					if e.ID != "" && PillButton(c, icLock, s.text("load_key"), false).Clicked() {
						key, err := s.Service.GetSiteAPIKey(s.Context, e.SiteID, e.ID)
						if err != nil {
							s.result(err)
						} else {
							e.Key = key
							s.Error = ""
							s.Notice = ""
						}
					}
				})
			})
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Icon(c, icLock).Size(13, 13).TextColor(t.TextMuted)
				ui.Text(c, s.text("key_editor_notice")).Grow(1).Basis(0).Wrap().FontSize(12).TextColor(t.TextMuted)
			})
			if s.Error != "" {
				Banner(c, icFailure, t.Danger, s.Error)
			}
			dialogButtons(c, func() {
				if PillButton(c, nil, s.text("cancel"), false).Clicked() {
					s.clearKeyEditor()
				}
				if AccentPillButton(c, icSave, s.text("save"), s.Busy).Clicked() {
					s.saveKeyAsync()
				}
			})
		})
	})
	if dialog.Dismissed() || !e.Open {
		s.clearKeyEditor()
	}
}

var capabilityKeys = []string{"codex-compact", "codex-vision", "codex-imagegen", "codex-search"}
