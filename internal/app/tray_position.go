package app

import (
	"encoding/json"
	"github.com/egoist/mygo"
	"time"
)

func clampPanelBounds(b, area mygo.Rectangle) mygo.Rectangle {
	b.Width = 420
	b.Height = min(680, area.Height*85/100)
	b.X = max(area.X, min(b.X, area.X+max(0, area.Width-b.Width)))
	b.Y = max(area.Y, min(b.Y, area.Y+max(0, area.Height-b.Height)))
	return b
}
func (h *host) rememberPanelPosition() {
	if !h.pinned || h.quitting {
		return
	}
	if h.positionTimer != nil {
		h.positionTimer.Stop()
	}
	h.positionTimer = time.AfterFunc(400*time.Millisecond, func() {
		mygo.RunOnMain(func() {
			if h.quitting || !h.pinned || h.shell.RestoreReady {
				return
			}
			b := h.panel.Bounds()
			raw, err := json.Marshal(mygo.Point{X: b.X, Y: b.Y})
			if err != nil {
				h.shell.result(err)
				return
			}
			settings, err := h.shell.Service.GetSettings(h.shell.Context)
			if err != nil {
				h.shell.result(err)
				return
			}
			if settings.Extra == nil {
				settings.Extra = map[string]json.RawMessage{}
			}
			settings.Extra["trayPosition"] = raw
			if err = h.shell.Service.SaveSettings(h.shell.Context, settings); err != nil {
				h.shell.result(err)
				return
			}
			h.shell.Settings.Extra = settings.Extra
		})
	})
}
