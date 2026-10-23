package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/buildinfo"
)

const repoURL = "https://github.com/Licoy/xiaobai-switch"

func (s *Shell) updateView(c *ui.Context) {
	t := c.Theme()
	Panel(c).Children(func() {
		ui.Row(c).FillWidth().Gap(18).AlignItems(ui.Center).Children(func() {
			ui.Image(c, logo).Size(72, 72).Radius(18)
			ui.Column(c).Grow(1).Basis(0).Gap(6).Children(func() {
				ui.Text(c, "XiaoBaiSwitch "+buildinfo.Version).FontSize(22).FontWeight(700).Selectable()
				ui.Text(c, s.text("about_detail")).FontSize(13).TextColor(t.TextMuted).Wrap()
				ui.Row(c).Gap(6).Children(func() {
					Pill(c, "macOS · Windows · Linux", TintBlue.C(t))
					Pill(c, "MIT", TintPurple.C(t))
				})
			})
			if PillButton(c, icGithub, "GitHub", false).Clicked() {
				s.openURL(repoURL)
			}
		})
	})
	s.updatePanel(c)
	settingsGroup(c, icSettings, TintGray, s.text("update_preferences"), func(row func(label, detail string, control func())) {
		row(s.text("auto_update"), s.text("auto_update_detail"), func() { s.settingSwitch(c, "auto_update", &s.Settings.AutoCheckUpdate) })
	})
}
func (s *Shell) updatePanel(c *ui.Context) {
	t := c.Theme()
	Panel(c).Children(func() {
		if s.Updates == nil {
			PanelTitle(c, icDownload, TintBlue, s.text("updates"), "", false, nil)
			ui.Text(c, s.text("update_not_connected")).FontSize(12).TextColor(t.TextMuted).Wrap()
			return
		}
		state := s.Updates.Snapshot()
		label, col := s.text("update_latest"), t.Success
		switch {
		case !state.Enabled:
			label, col = s.text("updates_disabled"), TintGray.C(t)
		case state.Error != nil:
			label, col = s.text("update_failed"), t.Danger
		case state.Checking:
			label, col = s.text("update_checking"), t.Accent
		case state.Installing:
			label, col = s.text("update_installing"), t.Accent
		case state.Installed:
			label, col = s.text("update_installed"), t.Success
		case state.Available:
			label, col = s.text("update_available"), TintOrange.C(t)
		}
		PanelTitle(c, icDownload, TintBlue, s.text("updates"), "", false, func() {
			DotPill(c, label, col)
		})
		if !state.Enabled {
			return
		}
		if state.Error != nil {
			Banner(c, icWarning, t.Danger, s.safeErrorDetail(*state.Error))
		}
		if state.Installing {
			frac := 0.0
			if state.Total > 0 {
				frac = float64(state.Downloaded) / float64(state.Total)
			}
			Bar(c, frac, t.Accent)
			ui.Text(c, fmt.Sprintf("%s / %s", formatSize(uint64(state.Downloaded)), formatSize(uint64(state.Total)))).FontSize(11).TextColor(t.TextMuted)
		}
		if state.Available {
			ui.Column(c).FillWidth().Gap(6).Padding(12, 14).Radius(12).Background(pal(t).Inset).Children(func() {
				ui.Text(c, state.Version).FontSize(17).FontWeight(700).TextColor(TintOrange.Text(t))
				if state.Notes != "" {
					ui.Text(c, state.Notes).FontSize(12).TextColor(t.TextMuted).Wrap().Selectable()
				}
			})
		}
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			if state.Installed {
				if AccentPillButton(c, icRestart, s.text("restart_now"), false).Clicked() {
					s.restart()
				}
				return
			}
			if BusyPillButton(c, icRefresh, s.text("check_updates"), state.Checking, s.Busy || state.Installing).Clicked() {
				s.task(func() (func(), error) {
					err := s.Updates.Check(s.Context)
					return func() { s.Notice = s.text("update_check_completed") }, err
				})
			}
			if state.Available && AccentPillButton(c, icInstall, s.text("install_update"), s.Busy || state.Installing).Clicked() {
				s.confirm(s.text("install_update"), state.Version, func() error { s.task(func() (func(), error) { return nil, s.Updates.Install(s.Context) }); return nil })
			}
			if (state.Checking || state.Installing) && PillButton(c, icClose, s.text("cancel"), false).Clicked() {
				s.Updates.Cancel()
			}
		})
	})
}
func (s *Shell) writesBlocked() bool {
	if !s.RestoreReady {
		return false
	}
	s.Notice = s.text("restore_queued")
	return true
}
func (s *Shell) restorePendingView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(28).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		Panel(c).MaxWidth(460).AlignItems(ui.Center).Children(func() {
			Tile(c, icRestart, TintOrange, 52, true)
			ui.Text(c, s.text("restore_queued")).FontSize(18).FontWeight(700).TextAlign(ui.Center).Wrap()
			ui.Text(c, s.text("restore_relaunch_notice")).FontSize(13).TextColor(t.TextMuted).TextAlign(ui.Center).Wrap()
			if s.WarningDetail != "" {
				Banner(c, icWarning, t.Warning, s.WarningDetail)
			}
			if AccentPillButton(c, icRestart, s.text("restart_now"), false).Clicked() {
				s.restart()
			}
		})
	})
}
