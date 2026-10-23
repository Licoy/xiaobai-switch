package app

import (
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/buildinfo"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/native"
)

var logo = ui.MustParseSVG(native.Logo)
var routeIcons = loadRouteIcons()

func loadRouteIcons() map[string]*ui.SVG {
	icons := map[string]*ui.SVG{}
	for _, r := range routes {
		icons[r.ID] = mustIcon(r.Icon)
	}
	return icons
}

const sidebarWidth = 224

func (s *Shell) View(c *ui.Context) {
	t := s.theme(c)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(t.Background).Children(func() {
		ui.Column(c).Width(sidebarWidth).Shrink(0).Background(pal(t).Sidebar).BorderWidth(0, 1, 0, 0).BorderColor(pal(t).Line).Children(func() { s.sidebar(c) })
		ui.Column(c).Grow(1).Basis(0).Children(func() { s.content(c) })
	})
	s.dialogs(c)
}
func (s *Shell) theme(c *ui.Context) *ui.Theme {
	dark := c.Theme().Dark
	if s.Settings.ThemeMode == "dark" {
		dark = true
	} else if s.Settings.ThemeMode == "light" {
		dark = false
	}
	color := s.Settings.PrimaryColor
	if !hexColor.MatchString(color) {
		color = "#0C76F8"
	}
	key := color + strconv.FormatBool(dark)
	if s.themeKey != key || s.themeCache == nil {
		s.themeKey, s.themeCache = key, buildTheme(dark, ui.Hex(color))
	}
	c.SetTheme(s.themeCache)
	return s.themeCache
}
func (s *Shell) sidebar(c *ui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	top := float32(14)
	if bar.Left > 0 && bar.Height > 0 {
		top = max(bar.Height, 52)
	}
	ui.Row(c).FillWidth().Height(top).DragWindow()
	ui.Scroll(c.Key("sidebar")).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(0, 10, 10, 10).Gap(2).Children(func() {
			group := ""
			for _, r := range routes {
				if r.Group != group {
					group = r.Group
					ui.Text(c, s.text(group)).FontSize(12).FontWeight(600).TextColor(t.TextMuted).Padding(14, 8, 4, 8)
				}
				s.nav(c, r)
			}
		})
	})
	ui.Row(c).FillWidth().Padding(10, 14, 14, 14).Gap(10).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(pal(t).Line).Children(func() {
		ui.Image(c, logo).Size(30, 30).Radius(8)
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			ui.Text(c, "XiaoBaiSwitch").FontSize(13).FontWeight(600).SingleLine()
			sub := "v" + buildinfo.Version
			if s.Config.Development {
				label := s.text("development")
				if s.Config.UseRealHome {
					label = s.text("real_home_development")
				}
				ui.Text(c, label).FontSize(11).TextColor(t.Warning).SingleLine()
			} else {
				ui.Text(c, sub).FontSize(11).TextColor(t.TextMuted).SingleLine()
			}
		})
	})
}
func (s *Shell) nav(c *ui.Context, r Route) {
	t := c.Theme()
	sel := s.Current == r.ID
	b := ui.ButtonBase(c.Key(r.ID)).FillWidth().Label(s.label(r)).Padding(5, 8).Gap(10).Radius(8).AlignItems(ui.Center)
	text, muted := t.Text, t.TextMuted
	switch {
	case sel:
		b.Background(t.Accent)
		text, muted = t.AccentText, t.AccentText.Alpha(0.8)
	case b.Hovered():
		b.Background(pal(t).Hover)
	}
	b.Children(func() {
		if sel {
			ui.Row(c).Size(24, 24).Shrink(0).Radius(7).Background(ui.RGBA(255, 255, 255, 0.22)).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, routeIcons[r.ID]).Size(14, 14).TextColor(t.AccentText)
			})
		} else {
			Tile(c, routeIcons[r.ID], r.Tint, 24, true)
		}
		ui.Text(c, s.label(r)).FontSize(13).TextColor(text).SingleLine().Grow(1)
		if r.Group == "apply" {
			_, col := s.targetTone(targetForRoute(r.ID))
			if sel {
				col = t.AccentText
			}
			ui.Box(c).Size(7, 7).Radius(4).Background(col)
		} else if v := s.navValue(r.ID); v != "" {
			ui.Text(c, v).FontSize(12).TextColor(muted).FontFeatures("tnum").SingleLine()
		}
	})
	if b.Clicked() {
		s.navigate(r.ID)
	}
}
func (s *Shell) navigate(id string) {
	if s.Current == "backups" && id != "backups" {
		s.BackupUI.Password = ""
	}
	s.Current = id
	if mygo.App.IsReady() {
		if id == "skills" {
			s.loadSkills()
		}
		if id == "backups" {
			s.loadBackups(false)
		}
	}
}
func (s *Shell) navValue(id string) string {
	switch id {
	case "sites":
		return strconv.Itoa(len(s.Sites))
	case "skills":
		if len(s.Skills.Local) > 0 {
			return strconv.Itoa(len(s.Skills.Local))
		}
	}
	return ""
}

// targetTone names the state of a target's live configuration with its color
func (s *Shell) targetTone(target domain.TargetKind) (string, ui.Color) {
	status, ok := s.Statuses[target]
	if !ok {
		return s.text("status_pending"), TintGray.C(s.themeOrDefault())
	}
	t := s.themeOrDefault()
	switch status.Status {
	case domain.Applied:
		return s.text("status_applied"), t.Success
	case domain.Stale:
		return s.text("status_stale"), t.Warning
	case domain.Orphan, domain.Failed:
		return s.text("status_" + string(status.Status)), t.Danger
	}
	return s.text("status_" + string(status.Status)), TintGray.C(t)
}
func (s *Shell) themeOrDefault() *ui.Theme {
	if s.themeCache == nil {
		s.themeCache = buildTheme(false, ui.Hex("#0C76F8"))
	}
	return s.themeCache
}

func (s *Shell) content(c *ui.Context) {
	t := c.Theme()
	if s.RestoreReady {
		s.restorePendingView(c)
		return
	}
	r := routeByID(s.Current)
	bar := c.TitleBar()
	height := max(bar.Height, 52)
	ui.Row(c).FillWidth().Height(height).Padding(0, bar.Right+20, 0, 24).Gap(10).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Text(c, s.label(r)).FontSize(16).FontWeight(700).SingleLine()
		if r.Group == "apply" {
			label, col := s.targetTone(targetForRoute(r.ID))
			DotPill(c, label, col)
		}
		ui.Box(c).Grow(1)
		if s.Busy {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Size(14, 14).Label(s.text("working"))
				ui.Text(c, s.text("working")).FontSize(12).TextColor(t.TextMuted)
			})
		}
		s.headerActions(c, r)
	})
	if s.Current == "sites" {
		s.sitesView(c)
		return
	}
	_, h := c.Size()
	ui.Scroll(c.Key("page-" + s.Current)).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MinHeight(max(h-height, 0)).Padding(6, 24, 24, 24).Gap(16).Children(func() {
			switch s.Current {
			case "overview":
				s.overviewView(c)
			case "appearance", "network", "paths":
				s.settingsView(c)
			case "claude", "codex", "pi", "prime":
				s.targetView(c, targetForRoute(s.Current), false)
			case "about":
				s.updateView(c)
			case "skills":
				s.skillsView(c)
			case "backups":
				s.backupView(c)
			}
		})
	})
}
func (s *Shell) headerActions(c *ui.Context, r Route) {
	switch {
	case r.ID == "overview" || r.Group == "apply":
		if BusyPillButton(c, icRefresh, s.text("status_refresh"), s.Busy, false).Clicked() {
			s.refreshStatus(targetForRoute(r.ID))
		}
	case r.ID == "skills":
		if BusyPillButton(c, icRefresh, s.text("refresh"), s.Busy, false).Clicked() {
			s.loadSkills()
		}
	case r.ID == "sites":
		if AccentPillButton(c, icAdd, s.text("add_site"), false).Clicked() {
			s.editSite(nil)
		}
	}
}
func (s *Shell) dialogs(c *ui.Context) {
	if s.Editor != nil {
		s.siteEditorView(c)
	}
	if s.KeyEditor != nil {
		s.keyEditorView(c)
	}
	s.confirmationView(c, false)
	s.promptView(c, false)
}

// dialogHeader opens every dialog with a solid tile and its title
func dialogHeader(c *ui.Context, icon *ui.SVG, k Tint, title string) {
	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
		Tile(c, icon, k, 34, true)
		ui.Text(c, title).FontSize(17).FontWeight(700).Grow(1).Basis(0).Wrap()
	})
}
func dialogButtons(c *ui.Context, buttons func()) {
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Grow(1)
		buttons()
	})
}
func (s *Shell) confirmationView(c *ui.Context, tray bool) {
	if s.Confirm == nil || s.Confirm.Tray != tray {
		return
	}
	open := true
	confirm := s.Confirm
	t := c.Theme()
	dialog := ui.Modal(c.Key("confirm"), &open, func() {
		ui.Column(c).Width(modalWidth(tray)).Gap(16).Children(func() {
			icon, k := icInfo, TintBlue
			if confirm.Danger {
				icon, k = icWarning, TintRed
			}
			dialogHeader(c, icon, k, confirm.Title)
			ui.Scroll(c).MaxHeight(modalBodyHeight(tray)).Children(func() {
				ui.Column(c).FillWidth().Padding(12, 14).Radius(12).Background(pal(t).Card).Children(func() {
					ui.Text(c, confirm.Detail).FontSize(13).Wrap().Selectable()
				})
			})
			if confirm.ToggleLabel != "" {
				ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, confirm.ToggleLabel).FontSize(13).Grow(1).Basis(0).Wrap()
					ui.Switch(c.Key("confirm_toggle"), &confirm.Toggle).Label(confirm.ToggleLabel)
				})
			}
			dialogButtons(c, func() {
				if ui.Button(c, s.text("cancel")).Radius(999).Clicked() {
					if confirm.Dismiss != nil {
						confirm.Dismiss()
					}
					s.Confirm = nil
				}
				ok := ui.PrimaryButton(c, s.text("confirm")).Radius(999)
				if confirm.Danger {
					ok.Background(t.Danger)
				}
				if ok.Clicked() {
					s.Confirm = nil
					err := confirm.Action()
					if err != nil {
						s.result(err)
					}
				}
			})
		})
	})
	if dialog.Dismissed() || !open {
		if confirm.Dismiss != nil {
			confirm.Dismiss()
		}
		s.Confirm = nil
	}
}

func (s *Shell) hasPrompt() bool {
	return s.Error != "" || s.Warning != "" || s.StartupNotice != "" || s.TrayError != "" || len(s.OperationResults) > 0 || s.Notice != ""
}
func (s *Shell) clearPrompt() {
	s.Error, s.ErrorDetail, s.Warning, s.StartupNotice, s.TrayError, s.Notice = "", "", "", "", "", ""
	if !s.RestoreReady {
		s.WarningDetail = ""
	}
	s.OperationResults = nil
	s.ErrorOpen = false
}

// promptView shows every result, warning and error as one dialog, in the tray panel when the user works there
func (s *Shell) promptView(c *ui.Context, tray bool) {
	if !s.hasPrompt() || s.Confirm != nil || s.Editor != nil || s.KeyEditor != nil || s.Skills.Detail != nil {
		return
	}
	inTray := s.PromptInTray != nil && s.PromptInTray()
	if inTray != tray {
		return
	}
	failed := false
	for _, r := range s.OperationResults {
		failed = failed || !r.Ok
	}
	icon, k, title := icSuccess, TintGreen, s.text("prompt_success")
	switch {
	case s.Error != "":
		icon, k, title = icFailure, TintRed, s.text("prompt_error")
	case s.Warning != "" || s.TrayError != "" || failed:
		icon, k, title = icWarning, TintOrange, s.text("prompt_warning")
	case s.StartupNotice != "":
		icon, k, title = icInfo, TintBlue, s.text("prompt_notice")
	}
	open := true
	done := false
	dialog := ui.Modal(c.Key("prompt"), &open, func() {
		ui.Column(c).Width(modalWidth(tray)).Gap(16).Children(func() {
			dialogHeader(c, icon, k, title)
			ui.Scroll(c).MaxHeight(modalBodyHeight(tray)).Children(func() {
				ui.Column(c).FillWidth().Gap(10).Children(func() {
					for _, line := range []string{s.StartupNotice, s.Error, s.Warning, s.Notice} {
						if line != "" {
							ui.Text(c, line).FontSize(13).Wrap().Selectable()
						}
					}
					if s.TrayError != "" {
						ui.Text(c, s.text("tray_error")).FontSize(13).Wrap()
						mono(c, s.safeErrorDetail(s.TrayError)).Wrap()
					}
					s.operationResultsView(c)
					s.technicalError(c)
				})
			})
			dialogButtons(c, func() {
				if ui.PrimaryButton(c, s.text("ok")).Radius(999).Clicked() {
					done = true
				}
			})
		})
	})
	if done || dialog.Dismissed() || !open {
		s.clearPrompt()
	}
}
func (s *Shell) operationResultsView(c *ui.Context) {
	if len(s.OperationResults) == 0 {
		return
	}
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(0).Padding(4, 12).Radius(12).Background(pal(t).Card).Children(func() {
		for i, r := range s.OperationResults {
			if i > 0 {
				Divider(c)
			}
			ui.Row(c.Key(i)).FillWidth().Gap(10).Padding(8, 0).AlignItems(ui.Center).Children(func() {
				targetTile(c, r.Target, 24)
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, targetLabel(r.Target)).FontSize(13).FontWeight(600)
					if r.Message != "" {
						ui.Text(c, s.safeErrorDetail(r.Message)).FontSize(11).TextColor(t.TextMuted).Wrap()
					}
				})
				if r.Ok {
					DotPill(c, s.text("apply_success"), t.Success)
				} else {
					DotPill(c, s.text("operation_failed"), t.Danger)
				}
			})
		}
	})
}

// targetTile is the brand mark of a target on its tint
func targetTile(c *ui.Context, target domain.TargetKind, size float32) ui.Element {
	r := routeByID(routeForTarget(target))
	return Tile(c, routeIcons[r.ID], r.Tint, size, true)
}

// field labels a form control
func field(c *ui.Context, label string, draw func()) {
	ui.Column(c.Key(label)).FillWidth().Gap(6).Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(c.Theme().TextMuted)
		draw()
	})
}

func modalWidth(tray bool) float32 {
	if tray {
		return 350
	}
	return 520
}
func modalBodyHeight(tray bool) float32 {
	if tray {
		return 260
	}
	return 380
}
