package app

import (
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/native"
	"os"
	"runtime"
)

func (h *host) ready(background bool) {
	s := h.shell
	mygo.Theme.SetSource(mygo.ThemeSource(s.Settings.ThemeMode))
	hidden := mygo.App.WasOpenedAtLogin() && loginHidden(s.Settings)
	// Explicit launches always open the main window; --background is a development login simulation
	if s.Config.Development && background {
		hidden = true
	}
	h.main = mygo.NewWindow(mygo.WindowOptions{Title: "XiaoBaiSwitch", Width: 1100, Height: 720, MinWidth: 960, MinHeight: 640, TitleBarStyle: mygo.TitleBarHidden, TrafficLightPosition: &mygo.Point{X: 18, Y: 20}, Hidden: hidden, AlwaysOnTop: s.Settings.AlwaysOnTop, Content: ui.View(s.View)})
	s.Invalidate = func() {
		h.main.Invalidate()
		if h.panel != nil {
			h.panel.Invalidate()
		}
	}
	s.SettingsChanged = h.changed
	s.PromptInTray = func() bool {
		return h.panel != nil && h.panel.IsVisible() && (h.panel.IsFocused() || !h.main.IsVisible())
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{{Label: s.text("show"), Click: func(*mygo.MenuItem, *mygo.Window) { h.showMain() }}, mygo.Separator(), {Label: s.text("quit"), Role: mygo.RoleQuit}})
	var err error
	h.tray, err = mygo.NewTray(mygo.TrayOptions{Icon: native.Icon, ToolTip: "XiaoBaiSwitch"})
	if err != nil {
		s.TrayError = err.Error()
		h.showMain()
	} else {
		h.newPanel()
		if runtime.GOOS == "linux" {
			menu = mygo.NewMenu([]*mygo.MenuItem{{Label: s.text("quick_apply"), Click: func(*mygo.MenuItem, *mygo.Window) { h.showPanel() }}, {Label: s.text("show"), Click: func(*mygo.MenuItem, *mygo.Window) { h.showMain() }}, mygo.Separator(), {Label: s.text("quit"), Role: mygo.RoleQuit}})
			h.tray.SetMenu(menu)
		} else {
			h.tray.OnClick(func() {
				if h.panel.IsVisible() && !h.pinned {
					h.panel.Hide()
				} else {
					h.showPanel()
				}
			})
			h.tray.OnRightClick(func() { h.tray.SetMenu(h.menu); defer h.tray.SetMenu(nil); h.tray.PopUpMenu() })
		}
	}
	h.main.OnClose(func(e *mygo.CloseEvent) {
		if h.quitting {
			return
		}
		e.PreventDefault()
		if h.tray != nil && s.Settings.CloseToTray {
			h.main.Hide()
		} else {
			mygo.App.Quit()
		}
	})
	if !s.Config.Development {
		if err = h.changed(s.Settings); err != nil {
			s.result(err)
		}
		if err = mygo.App.RegisterURLScheme("xiaobaiswitch"); err != nil {
			s.result(err)
		}
	}
	if !hidden {
		h.showMain()
	}
	if err := h.updateTrayMenu(s.Settings); err != nil {
		s.result(err)
	}
	h.openArguments(os.Args[1:])
	s.refreshStatus("")
}
func (h *host) newPanel() {
	h.panel = mygo.NewWindow(mygo.WindowOptions{Title: "XiaoBaiSwitch", Width: 420, Height: 680, Hidden: true, Frameless: true, DisableResize: true, DisableMinimize: true, DisableMaximize: true, SkipTaskbar: true, AlwaysOnTop: true, Content: ui.View(h.trayView)})
	h.panel.OnBlur(func() {
		if !h.pinned && h.shell.Confirm == nil && !h.shell.hasPrompt() {
			h.panel.Hide()
		}
	})
	h.panel.OnClose(func(e *mygo.CloseEvent) {
		if !h.quitting {
			e.PreventDefault()
			h.panel.Hide()
		}
	})
	h.panel.OnMove(func() {
		if h.pinned {
			h.clampPanel()
			h.rememberPanelPosition()
		}
	})
	if raw := h.shell.Settings.Extra["trayPinned"]; raw != nil {
		if err := json.Unmarshal(raw, &h.pinned); err != nil {
			h.shell.result(err)
		}
	}
	if h.pinned {
		if raw := h.shell.Settings.Extra["trayPosition"]; raw != nil {
			var point mygo.Point
			if err := json.Unmarshal(raw, &point); err != nil {
				h.shell.result(err)
			} else {
				h.panel.SetPosition(point.X, point.Y)
			}
		}
	}
	mygo.Screen.OnDisplaysChanged(func() {
		if h.panel != nil {
			h.clampPanel()
		}
	})
}
func (h *host) showPanel() {
	if h.panel == nil {
		return
	}
	if !h.pinned {
		anchor := h.tray.Bounds()
		if anchor.Width == 0 {
			h.panel.Center()
		} else {
			b := h.panel.Bounds()
			x := anchor.X + (anchor.Width-b.Width)/2
			y := anchor.Y + anchor.Height + 6
			area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: anchor.X, Y: anchor.Y}).WorkArea
			if y+b.Height > area.Y+area.Height {
				y = anchor.Y - b.Height - 6
			}
			h.panel.SetPosition(x, y)
		}
	}
	h.clampPanel()
	h.panel.Show()
	h.panel.Focus()
}
func (h *host) clampPanel() {
	b := h.panel.Bounds()
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}).WorkArea
	fitted := clampPanelBounds(b, area)
	if fitted != b {
		h.panel.SetBounds(fitted)
	}
}
func (h *host) trayView(c *ui.Context) {
	s := h.shell
	s.theme(c)
	t := c.Theme()
	if s.RestoreReady {
		s.restorePendingView(c)
		return
	}
	ui.Column(c).Fill().Background(pal(t).Sidebar).Children(func() {
		ui.Row(c).FillWidth().Padding(14, 14, 10, 16).Gap(10).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Image(c, logo).Size(28, 28).Radius(7)
			ui.Column(c).Grow(1).Basis(0).Gap(1).Children(func() {
				ui.Text(c, "XiaoBaiSwitch").FontSize(14).FontWeight(700).SingleLine()
				applied := 0
				for _, target := range targetKinds {
					if status, ok := s.Statuses[target]; ok && status.Status == domain.Applied {
						applied++
					}
				}
				ui.Text(c, fmt.Sprintf("%s %d / %d", s.text("stat_applied"), applied, len(targetKinds))).FontSize(11).TextColor(t.TextMuted)
			})
			if s.Busy {
				ui.Spinner(c).Size(14, 14).Label(s.text("working"))
			}
			if BusyPillButton(c, icRefresh, "", s.Busy, false).Label(s.text("status_refresh")).Tooltip(s.text("status_refresh")).Clicked() {
				s.refreshStatus("")
			}
			icon := icPin
			if h.pinned {
				icon = icPinned
			}
			pin := PillButton(c, icon, "", false).Label(s.text("pin")).Tooltip(s.text("pin"))
			if pin.Clicked() {
				h.pinned = !h.pinned
				if s.Settings.Extra == nil {
					s.Settings.Extra = map[string]json.RawMessage{}
				}
				raw, err := json.Marshal(h.pinned)
				if s.result(err) {
					s.Settings.Extra["trayPinned"] = raw
					s.saveSettings(s.Settings)
				}
			}
		})
		ui.Row(c).FillWidth().Padding(0, 12, 10, 12).Gap(6).Children(func() {
			for i, target := range targetKinds {
				h.trayTab(c, i, target)
			}
		})
		target := targetKinds[min(h.trayTarget, len(targetKinds)-1)]
		ui.Scroll(c.Key("tray-body")).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(0, 12, 12, 12).Children(func() {
				Panel(c).Gap(12).Children(func() {
					ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
						targetTile(c, target, 30)
						ui.Text(c, targetLabel(target)).FontSize(15).FontWeight(700).TextColor(routeByID(routeForTarget(target)).Tint.Text(t)).Grow(1).Basis(0)
						label, col := s.targetTone(target)
						DotPill(c, label, col)
					})
					s.targetView(c.Key(string(target)), target, true)
				})
			})
		})
		ui.Row(c).FillWidth().Padding(10, 12, 12, 12).Gap(8).BorderWidth(1, 0, 0, 0).BorderColor(pal(t).Line).AlignItems(ui.Center).Children(func() {
			ui.Text(c, s.text("tray_secret_hint")).FontSize(11).TextColor(t.TextMuted).Grow(1).Basis(0).SingleLine()
			if AccentPillButton(c, icExternal, s.text("open_main"), false).Clicked() {
				h.showMain()
				if !h.pinned {
					h.panel.Hide()
				}
			}
		})
	})
	s.confirmationView(c, true)
	s.promptView(c, true)
}
func (h *host) trayTab(c *ui.Context, i int, target domain.TargetKind) {
	s := h.shell
	t := c.Theme()
	s.draft(target)
	selected := h.trayTarget == i
	b := ui.ButtonBase(c.Key("tray-tab-"+string(target))).Label(targetLabel(target)).Grow(1).Basis(0).Padding(8, 4).Gap(5).Radius(12).AlignItems(ui.Center).FocusRing(false)
	switch {
	case selected:
		b.Background(pal(t).Card).Shadow(0, 1, 3, 0, ui.Hex("#000000").Alpha(0.08))
	case b.Hovered():
		b.Background(pal(t).Hover)
	}
	b.Children(func() {
		ui.Column(c).FillWidth().Gap(5).AlignItems(ui.Center).Children(func() {
			targetTile(c, target, 26)
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				_, col := s.targetTone(target)
				ui.Box(c).Size(6, 6).Radius(3).Background(col)
				ui.Text(c, targetLabel(target)).FontSize(11).FontWeight(600).SingleLine()
			})
		})
	})
	if b.Clicked() {
		h.trayTarget = i
	}
}
func targetLabel(t domain.TargetKind) string {
	switch t {
	case domain.ClaudeCode:
		return "Claude Code"
	case domain.Codex:
		return "Codex"
	case domain.Pi:
		return "Pi"
	case domain.Prime:
		return "Prime"
	}
	panic(fmt.Sprintf("unknown target %s", t))
}
