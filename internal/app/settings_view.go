package app

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/paths"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

var accentPresets = []string{"#0C76F8", "#5856D6", "#AF52DE", "#FF2D55", "#FF3B30", "#F59100", "#28B14C", "#14A3BE", "#1D1D1F"}

func (s *Shell) settingsView(c *ui.Context) {
	switch s.Current {
	case "appearance":
		s.appearanceSettings(c)
	case "network":
		s.networkSettings(c)
	case "paths":
		s.pathSettings(c)
	}
}
func (s *Shell) settingTabs(c *ui.Context, key string, v *string, values []string, labels ...string) {
	if stringTabs(c.Key(key), v, values, labels...) {
		s.saveSettings(s.Settings)
	}
}
func (s *Shell) settingSwitch(c *ui.Context, key string, v *bool) {
	if ui.Switch(c.Key(key), v).Label(s.text(key)).Changed() {
		s.saveSettings(s.Settings)
	}
}
func (s *Shell) settingNumber(c *ui.Context, key string, v *uint32, min, max float64) {
	n := float64(*v)
	if ui.NumberInput(c.Key(key), &n, min, max, 1).Label(s.text(key)).Changed() {
		*v = uint32(n)
		s.saveSettings(s.Settings)
	}
}
func (s *Shell) appearanceSettings(c *ui.Context) {
	settingsGroup(c, icPalette, TintPink, s.text("appearance"), func(row func(label, detail string, control func())) {
		row(s.text("language"), "", func() {
			s.settingTabs(c, "language", &s.Settings.Language, []string{"zh-CN", "en-US"}, "简体中文", "English")
		})
		row(s.text("theme_mode"), "", func() {
			s.settingTabs(c, "theme", &s.Settings.ThemeMode, []string{"system", "light", "dark"}, s.text("theme_system"), s.text("theme_light"), s.text("theme_dark"))
		})
		row(s.text("primary_color"), s.text("primary_color_detail"), func() { s.accentPicker(c) })
		row(s.text("always_on_top"), "", func() { s.settingSwitch(c, "always_on_top", &s.Settings.AlwaysOnTop) })
	})
	settingsGroup(c, icRestart, TintGreen, s.text("lifecycle"), func(row func(label, detail string, control func())) {
		detail := ""
		if s.Config.Development {
			detail = s.developmentNotice()
		}
		row(s.text("auto_start"), detail, func() {
			ui.Row(c).Disabled(s.Config.Development).Children(func() { s.settingSwitch(c, "auto_start", &s.Settings.AutoStart) })
		})
		row(s.text("login_hidden"), "", func() {
			hidden := loginHidden(s.Settings)
			if ui.Switch(c.Key("login_hidden"), &hidden).Label(s.text("login_hidden")).Changed() {
				s.Settings.LaunchAtLoginHidden = &hidden
				s.saveSettings(s.Settings)
			}
		})
		row(s.text("close_to_tray"), s.text("close_to_tray_detail"), func() { s.settingSwitch(c, "close_to_tray", &s.Settings.CloseToTray) })
	})
}
func (s *Shell) accentPicker(c *ui.Context) {
	t := c.Theme()
	for _, hex := range accentPresets {
		col := ui.Hex(hex)
		sel := strings.EqualFold(s.Settings.PrimaryColor, hex)
		b := ui.ButtonBase(c.Key(hex)).Label(hex).Size(22, 22).Radius(11).Background(col).FocusRing(false)
		if sel {
			b.Border(2, t.Background).Shadow(0, 0, 0, 2, col)
		} else if b.Hovered() {
			b.Shadow(0, 0, 0, 2, col.Alpha(0.4))
		}
		if b.Clicked() && !sel {
			s.Settings.PrimaryColor = hex
			s.saveSettings(s.Settings)
		}
	}
	color := ui.Hex(s.Settings.PrimaryColor)
	if ui.ColorWell(c.Key("custom-color"), &color).Changed() {
		s.Settings.PrimaryColor = colorHex(color)
		s.saveSettings(s.Settings)
	}
}
func (s *Shell) networkSettings(c *ui.Context) {
	settingsGroup(c, icGlobal, TintTeal, s.text("network"), func(row func(label, detail string, control func())) {
		row(s.text("proxy_mode"), s.text("proxy_mode_detail"), func() {
			s.settingTabs(c, "proxy_mode", &s.Settings.ProxyMode, []string{"system", "none", "custom"}, s.text("proxy_system"), s.text("proxy_none"), s.text("proxy_custom"))
		})
		if s.Settings.ProxyMode == "custom" {
			row(s.text("proxy_protocol"), "", func() {
				s.settingTabs(c, "proxy_protocol", &s.Settings.ProxyProtocol, []string{"http", "https", "socks5"}, "HTTP", "HTTPS", "SOCKS5")
			})
			row(s.text("proxy_host"), s.text("press_enter_save"), func() {
				host := value(s.Settings.ProxyHost)
				input := ui.TextInput(c.Key("proxy_host"), &host).Width(240).Placeholder("127.0.0.1").Label(s.text("proxy_host"))
				if input.Changed() {
					s.Settings.ProxyHost = optional(host)
				}
				if input.Submitted() {
					s.saveSettings(s.Settings)
				}
			})
			row(s.text("proxy_port"), "", func() {
				port := float64(value(s.Settings.ProxyPort))
				if ui.NumberInput(c.Key("proxy_port"), &port, 1, 65535, 1).Label(s.text("proxy_port")).Changed() {
					s.Settings.ProxyPort = pointer(uint16(port))
					s.saveSettings(s.Settings)
				}
			})
		}
	})
	settingsGroup(c, icRoute, TintIndigo, s.text("route_probe"), func(row func(label, detail string, control func())) {
		row(s.text("route_ttl"), s.text("route_ttl_detail"), func() { s.settingNumber(c, "route_ttl", &s.Settings.RouteProbeTtlMinutes, 1, 1440) })
	})
}
func (s *Shell) pathSettings(c *ui.Context) {
	t := c.Theme()
	settingsGroup(c, icDatabase, TintBlue, s.text("storage"), func(row func(label, detail string, control func())) {
		ui.Row(c).FillWidth().Gap(10).Padding(0, 0, 4, 0).AlignItems(ui.Center).Children(func() {
			mono(c, s.Config.DataDir).Grow(1).Basis(0).Wrap().Selectable()
			if PillButton(c, icFolder, s.text("open_directory"), false).Clicked() {
				s.openDirectory(s.Config.DataDir)
			}
		})
		if s.Config.Development {
			Banner(c, icShield, TintOrange.C(t), s.developmentNotice())
		}
	})
	settingsGroup(c, icPathFolder, TintOrange, s.text("target_paths"), func(row func(label, detail string, control func())) {
		for _, item := range []struct {
			key string
			v   **string
		}{{"claude_home", &s.Settings.ClaudeHomeOverride}, {"codex_home", &s.Settings.CodexHomeOverride}, {"pi_home", &s.Settings.PiAgentDirOverride}, {"prime_home", &s.Settings.PrimeAgentDirOverride}} {
			row(s.text(item.key), "", func() { s.pathInput(c, item.key, item.v) })
		}
		row("", s.text("press_enter_save"), nil)
	})
	settingsGroup(c, icTerminal, TintGray, s.text("environment"), func(row func(label, detail string, control func())) {
		row(s.text("env_mode"), s.text("env_matrix"), func() {
			modes := []choice{{"auto", "auto"}, {"file_only", "file_only"}, {"shell_rc", "shell_rc"}, {"user_env", "user_env"}}
			ui.Column(c).Width(160).Children(func() {
				if choose(c.Key("env_mode"), &s.Settings.CodexEnvInjectMode, modes).Changed() {
					s.saveSettings(s.Settings)
				}
			})
		})
		row(s.text("exclusive_auth"), "", func() { s.settingSwitch(c, "exclusive_auth", &s.Settings.ForceExclusiveClaudeAuthKey) })
	})
	Banner(c, icLock, TintOrange.C(t), s.text("secret_notice"))
}
func (s *Shell) pathInput(c *ui.Context, key string, v **string) {
	text := value(*v)
	input := ui.TextInput(c.Key(key), &text).Width(300).Placeholder(s.text("default_directory")).Label(s.text(key))
	if input.Changed() {
		*v = optional(text)
	}
	if input.Submitted() {
		if text != "" {
			if _, err := paths.ValidatePath(text); err != nil {
				s.result(err)
				return
			}
		}
		*v = optional(text)
		s.saveSettings(s.Settings)
	}
}
func colorHex(c ui.Color) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }
