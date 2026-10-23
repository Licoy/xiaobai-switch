package app

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The native look follows CPA-Bar: colored tiles in the sidebar, soft borderless cards, tinted section titles, large numbers and capsule controls, in the system colors

type Tint int

const (
	TintBlue Tint = iota
	TintGreen
	TintRed
	TintOrange
	TintTeal
	TintYellow
	TintPurple
	TintIndigo
	TintPink
	TintGray
)

var tintHex = [...][2]string{
	TintBlue:   {"#007AFF", "#0A84FF"},
	TintGreen:  {"#28B14C", "#30D158"},
	TintRed:    {"#FF3B30", "#FF453A"},
	TintOrange: {"#F59100", "#FF9F0A"},
	TintTeal:   {"#14A3BE", "#40C8E0"},
	TintYellow: {"#E6B400", "#FFD60A"},
	TintPurple: {"#AF52DE", "#BF5AF2"},
	TintIndigo: {"#5856D6", "#5E5CE6"},
	TintPink:   {"#FF2D55", "#FF375F"},
	TintGray:   {"#8E8E93", "#98989D"},
}

func (k Tint) C(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex(tintHex[k][1])
	}
	return ui.Hex(tintHex[k][0])
}

// Text darkens the tint on light backgrounds to keep it legible
func (k Tint) Text(t *ui.Theme) ui.Color {
	if t.Dark {
		return k.C(t)
	}
	return k.C(t).Mix(ui.RGB(0, 0, 0), 0.12)
}

type palette struct{ Sidebar, Card, Inset, Line, Hover ui.Color }

var palettes = [2]palette{
	{Sidebar: ui.Hex("#F5F5F7"), Card: ui.Hex("#F4F4F6"), Inset: ui.Hex("#FFFFFF"), Line: ui.RGBA(0, 0, 0, 0.07), Hover: ui.RGBA(0, 0, 0, 0.05)},
	{Sidebar: ui.Hex("#202022"), Card: ui.Hex("#262628"), Inset: ui.Hex("#303033"), Line: ui.RGBA(255, 255, 255, 0.08), Hover: ui.RGBA(255, 255, 255, 0.06)},
}

func pal(t *ui.Theme) palette {
	if t.Dark {
		return palettes[1]
	}
	return palettes[0]
}

func buildTheme(dark bool, accent ui.Color) *ui.Theme {
	var t *ui.Theme
	if dark {
		t = ui.DarkTheme()
		t.Background = ui.Hex("#18181A")
		t.Surface = ui.Hex("#333336")
		t.SurfaceHover = ui.Hex("#3C3C3F")
		t.SurfacePressed = ui.Hex("#47474A")
		t.Border = ui.Hex("#3A3A3D")
		t.Text = ui.Hex("#F5F5F7")
		t.TextMuted = ui.Hex("#98989D")
		t.Danger = ui.Hex("#FF453A")
		t.Warning = ui.Hex("#FF9F0A")
		t.Success = ui.Hex("#30D158")
		t.Inverse = ui.Hex("#F5F5F7")
		t.InverseText = ui.Hex("#1D1D1F")
	} else {
		t = ui.LightTheme()
		t.Background = ui.Hex("#FFFFFF")
		t.Surface = ui.Hex("#FFFFFF")
		t.SurfaceHover = ui.Hex("#F2F2F4")
		t.SurfacePressed = ui.Hex("#E8E8EB")
		t.Border = ui.Hex("#DCDCE0")
		t.Text = ui.Hex("#1D1D1F")
		t.TextMuted = ui.Hex("#86868B")
		t.Danger = ui.Hex("#E5342A")
		t.Warning = ui.Hex("#D97E00")
		t.Success = ui.Hex("#24A148")
		t.Inverse = ui.Hex("#1D1D1F")
		t.InverseText = ui.Hex("#FFFFFF")
	}
	t.Accent = accent
	t.AccentHover = accent.Mix(ui.RGB(255, 255, 255), 0.12)
	t.AccentPressed = accent.Mix(ui.RGB(0, 0, 0), 0.12)
	t.AccentText = ui.RGB(255, 255, 255)
	t.Selection = accent.Alpha(0.3)
	t.Focus = accent.Alpha(0.55)
	t.Radius = 8
	return t
}

// Tile is a rounded square holding an icon: solid with a white icon, or a light wash with the icon in the tint
func Tile(c *ui.Context, icon *ui.SVG, k Tint, size float32, solid bool) ui.Element {
	t := c.Theme()
	return colorTile(c, icon, k.C(t), k.Text(t), size, solid)
}
func colorTile(c *ui.Context, icon *ui.SVG, col, text ui.Color, size float32, solid bool) ui.Element {
	bg, fg := col.Alpha(0.15), text
	if solid {
		bg, fg = col, ui.RGB(255, 255, 255)
	}
	return ui.Row(c).Size(size, size).Shrink(0).Radius(size * 0.28).Background(bg).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).Size(size*0.58, size*0.58).TextColor(fg)
	})
}

// letterTile marks a site by the first letter of its name, in a tint picked from its id so it stays stable
func letterTile(c *ui.Context, name, id string, size float32) ui.Element {
	t := c.Theme()
	k := tintFor(id)
	letter := "?"
	for _, r := range strings.TrimSpace(name) {
		letter = strings.ToUpper(string(r))
		break
	}
	return ui.Row(c).Size(size, size).Shrink(0).Radius(size * 0.28).Background(k.C(t)).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		ui.Text(c, letter).FontSize(size * 0.46).FontWeight(700).TextColor(ui.RGB(255, 255, 255)).SingleLine()
	})
}
func tintFor(id string) Tint {
	h := uint32(2166136261)
	for i := 0; i < len(id); i++ {
		h = (h ^ uint32(id[i])) * 16777619
	}
	choices := []Tint{TintBlue, TintGreen, TintOrange, TintTeal, TintPurple, TintIndigo, TintPink, TintRed}
	return choices[h%uint32(len(choices))]
}

// Panel is a large card of a page
func Panel(c *ui.Context) ui.Element {
	return ui.Column(c).FillWidth().Padding(18).Gap(14).Radius(16).Background(pal(c.Theme()).Card)
}

// PanelTitle heads a panel with a tinted tile and title, muted text and actions at the right; it reports a click when it links somewhere
func PanelTitle(c *ui.Context, icon *ui.SVG, k Tint, title, trailing string, link bool, actions func()) bool {
	t := c.Theme()
	row := ui.Row(c)
	if link {
		row = ui.ButtonBase(c).FocusRing(false).Label(title).Padding(0).Background(ui.Color{})
	}
	row.FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		Tile(c, icon, k, 30, false)
		ui.Text(c, title).FontSize(16).FontWeight(600).TextColor(k.Text(t)).SingleLine().Grow(1)
		if trailing != "" {
			ui.Text(c, trailing).FontSize(12).TextColor(t.TextMuted).SingleLine()
		}
		if actions != nil {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(actions)
		}
		if link {
			col := t.TextMuted
			if row.Hovered() {
				col = t.Text
			}
			ui.Icon(c, icChevron).Size(16, 16).TextColor(col)
		}
	})
	return link && row.Clicked()
}

// BigValue is a caption over a large number and its unit
func BigValue(c *ui.Context, caption, value, unit string, col ui.Color) ui.Element {
	t := c.Theme()
	return ui.Column(c).Gap(2).Children(func() {
		if caption != "" {
			ui.Text(c, caption).FontSize(12).TextColor(t.TextMuted).SingleLine()
		}
		ui.Row(c).Gap(4).AlignItems(ui.End).Children(func() {
			ui.Text(c, value).FontSize(32).FontWeight(700).TextColor(col).FontFeatures("tnum").SingleLine()
			if unit != "" {
				ui.Text(c, unit).FontSize(15).FontWeight(600).TextColor(t.TextMuted).Padding(0, 0, 5, 0).SingleLine()
			}
		})
	})
}

type Metric struct {
	Label, Value string
	Color        ui.Color
}

// Metrics lays out labeled values in equal columns
func Metrics(c *ui.Context, ms ...Metric) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		for i, m := range ms {
			ui.Column(c.Key(i)).Grow(1).Basis(0).Gap(3).Children(func() {
				ui.Text(c, m.Label).FontSize(12).TextColor(t.TextMuted).SingleLine()
				col := m.Color
				if col == (ui.Color{}) {
					col = t.Text
				}
				ui.Text(c, m.Value).FontSize(14).FontWeight(600).TextColor(col).FontFeatures("tnum").SingleLine()
			})
		}
	})
}

// Stat is a tinted number card for summary rows
func Stat(c *ui.Context, icon *ui.SVG, k Tint, caption, value string) ui.Element {
	t := c.Theme()
	return ui.Column(c).Grow(1).Basis(0).Gap(10).Padding(14, 16).Radius(16).Background(pal(t).Card).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			Tile(c, icon, k, 24, false)
			ui.Text(c, caption).FontSize(12).TextColor(t.TextMuted).SingleLine()
		})
		ui.Text(c, value).FontSize(26).FontWeight(700).TextColor(k.Text(t)).FontFeatures("tnum").SingleLine()
	})
}

// Pill is a status in a tinted capsule
func Pill(c *ui.Context, text string, col ui.Color) ui.Element {
	return ui.Row(c).Shrink(0).Padding(2, 9).Radius(999).Background(col.Alpha(0.15)).AlignItems(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(11).FontWeight(600).TextColor(col).SingleLine()
	})
}

// DotPill is a Pill with a status dot, so state never depends on color alone
func DotPill(c *ui.Context, text string, col ui.Color) ui.Element {
	return ui.Row(c).Shrink(0).Padding(2, 9).Gap(6).Radius(999).Background(col.Alpha(0.15)).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(col)
		ui.Text(c, text).FontSize(11).FontWeight(600).TextColor(col).SingleLine()
	})
}

// PillButton is a capsule button with an icon; disabled ones report no clicks
func PillButton(c *ui.Context, icon *ui.SVG, label string, disabled bool) ui.Element {
	return pillButton(c, icon, label, disabled, false, ui.Color{})
}

// DangerPillButton is a PillButton for destructive actions
func DangerPillButton(c *ui.Context, icon *ui.SVG, label string, disabled bool) ui.Element {
	return pillButton(c, icon, label, disabled, false, c.Theme().Danger)
}

// BusyPillButton spins its icon while busy instead of showing a spinner beside it
func BusyPillButton(c *ui.Context, icon *ui.SVG, label string, busy, disabled bool) ui.Element {
	return pillButton(c, icon, label, disabled || busy, busy, ui.Color{})
}

func pillButton(c *ui.Context, icon *ui.SVG, label string, disabled, busy bool, tone ui.Color) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Disabled(disabled).Padding(5, 12).Gap(6).Radius(999).AlignItems(ui.Center).Shrink(0)
	if label == "" {
		b.Padding(6, 7)
	}
	base := t.Text
	if tone != (ui.Color{}) {
		base = tone
	}
	bg := base.Alpha(0.07)
	switch {
	case disabled:
	case b.Pressed():
		bg = base.Alpha(0.16)
	case b.Hovered():
		bg = base.Alpha(0.11)
	}
	b.Background(bg)
	return b.Children(func() {
		col := base
		if disabled {
			col = t.TextMuted
		}
		switch {
		case busy:
			spin := ui.Icon(c, icRefresh).Size(14, 14).TextColor(col)
			spin.Rotate(spin.Loop("spin", time.Second, ui.Linear) * 360)
		case icon != nil:
			ui.Icon(c, icon).Size(14, 14).TextColor(col)
		}
		if label != "" {
			ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(col).SingleLine()
		}
	})
}

// AccentPillButton is the primary action of a toolbar
func AccentPillButton(c *ui.Context, icon *ui.SVG, label string, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Disabled(disabled).Padding(6, 14).Gap(6).Radius(999).AlignItems(ui.Center).Shrink(0)
	bg := t.Accent
	switch {
	case disabled:
		bg = t.Accent.Alpha(0.4)
	case b.Pressed():
		bg = t.AccentPressed
	case b.Hovered():
		bg = t.AccentHover
	}
	b.Background(bg)
	return b.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(14, 14).TextColor(t.AccentText)
		}
		ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(t.AccentText).SingleLine()
	})
}

// PillTabs is a capsule segmented control and reports whether the choice changed
func PillTabs(c *ui.Context, selected *int, labels ...string) bool {
	t := c.Theme()
	seg := ui.SegmentedBase(c, selected, len(labels))
	seg.Track.Shrink(0).Padding(3).Gap(2).Radius(999).Background(t.Text.Alpha(0.06)).Children(func() {
		for i, l := range labels {
			s := seg.Segment(i).Padding(4, 12).Radius(999)
			col := t.TextMuted
			if i == *selected {
				col = t.Text
				s.Background(pal(t).Inset).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
			}
			s.Children(func() { ui.Text(c, l).FontSize(12).FontWeight(500).TextColor(col).SingleLine() })
		}
	})
	return seg.Track.Changed()
}

// stringTabs binds PillTabs to a string value chosen from values
func stringTabs(c *ui.Context, v *string, values []string, labels ...string) bool {
	i := 0
	for j, value := range values {
		if value == *v {
			i = j
		}
	}
	if PillTabs(c, &i, labels...) {
		*v = values[i]
		return true
	}
	return false
}

// Bar is a thin progress bar, full at frac 1
func Bar(c *ui.Context, frac float64, col ui.Color) ui.Element {
	frac = math.Max(0, math.Min(1, frac))
	return ui.Box(c).FillWidth().Height(6).Radius(3).Background(col.Alpha(0.18)).Draw(func(p *ui.Painter, r ui.Rect) {
		if frac > 0 {
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(r.H, r.W*float32(frac)), H: r.H}, col, r.H/2)
		}
	})
}

// Banner is an inline message on a page, for lasting states rather than results of actions, which use the prompt dialog
func Banner(c *ui.Context, icon *ui.SVG, col ui.Color, text string) {
	ui.Row(c).FillWidth().Gap(10).Padding(10, 12).Radius(12).Background(col.Alpha(0.10)).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, icon).Size(16, 16).TextColor(col)
		ui.Text(c, text).Grow(1).Basis(0).FontSize(12).Wrap()
	})
}

// EmptyState fills a list with a message; children appended stay centered with it
func EmptyState(c *ui.Context, icon *ui.SVG, k Tint, title, detail string) ui.Element {
	t := c.Theme()
	return ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(8).Padding(28).Children(func() {
		Tile(c, icon, k, 44, false)
		ui.Text(c, title).FontSize(15).FontWeight(600)
		if detail != "" {
			ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted).TextAlign(ui.Center).MaxWidth(420).Wrap()
		}
	})
}

// Divider is a hairline between rows of a panel
func Divider(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Background(pal(c.Theme()).Line)
}

// settingsGroup is a panel of rows with the label and detail at the left and the control at the right
func settingsGroup(c *ui.Context, icon *ui.SVG, k Tint, title string, rows func(row func(label, detail string, control func()))) {
	Panel(c).Gap(0).Children(func() {
		ui.Row(c).FillWidth().Padding(0, 0, 10, 0).Children(func() { PanelTitle(c, icon, k, title, "", false, nil) })
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
func settingRow(c *ui.Context, label, detail string, control func()) {
	t := c.Theme()
	ui.Row(c.Key(label)).FillWidth().Gap(16).Padding(10, 0).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			if label != "" {
				ui.Text(c, label).FontSize(13).FontWeight(500)
			}
			if detail != "" {
				ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted).Wrap()
			}
		})
		if control != nil {
			ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(control)
		}
	})
}

// columns fits n equal columns of cards across the width
func columns(width, minW float32, most int) int {
	return max(1, min(most, int((width+14)/(minW+14))))
}

// grid lays out items in rows of n equal columns
func grid(c *ui.Context, n, count int, gap float32, item func(i int)) {
	ui.Column(c).FillWidth().Gap(gap).Children(func() {
		for start := 0; start < count; start += n {
			ui.Row(c.Key(start)).FillWidth().Gap(gap).AlignItems(ui.Stretch).Children(func() {
				for i := start; i < start+n; i++ {
					ui.Column(c.Key(i)).Grow(1).Basis(0).Children(func() {
						if i < count {
							item(i)
						}
					})
				}
			})
		}
	})
}

func mono(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).Font("monospace").FontSize(11)
}

// disclosure is an inset row that folds a section of a form or panel
func disclosure(c *ui.Context, key string, icon *ui.SVG, title string, open *bool, body func()) {
	t := c.Theme()
	parts := ui.CollapsibleBase(c.Key(key), open)
	parts.Trigger.FillWidth().Gap(8).Padding(10, 12).Radius(12).Background(pal(t).Inset).AlignItems(ui.Center).Label(title).Children(func() {
		ui.Icon(c, icon).Size(15, 15).TextColor(t.TextMuted)
		ui.Text(c, title).FontSize(13).FontWeight(600).Grow(1)
		ui.Icon(c, icDown).Size(15, 15).TextColor(t.TextMuted).Rotate(-90 + 90*parts.Progress())
	})
	parts.Panel(func() { ui.Column(c).FillWidth().Gap(12).Children(body) }).Padding(10, 0, 0, 0)
}
