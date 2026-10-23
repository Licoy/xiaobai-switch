package app

import (
	"encoding/json"
	"github.com/egoist/mygo"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/l10n"
	"runtime"
)

func (h *host) updateTrayMenu(settings domain.AppSettings) error {
	messages, err := l10n.Load(settings.Language)
	if err != nil {
		return err
	}
	h.menu = mygo.NewMenu([]*mygo.MenuItem{
		{Label: messages.Text("quick_apply"), Click: func(*mygo.MenuItem, *mygo.Window) { h.showPanel() }},
		{Label: messages.Text("show"), Click: func(*mygo.MenuItem, *mygo.Window) { h.showMain() }},
		mygo.Separator(), {Label: messages.Text("quit"), Role: mygo.RoleQuit},
	})
	if h.tray != nil && runtime.GOOS == "linux" {
		h.tray.SetMenu(h.menu)
	}
	pinned := false
	if raw := settings.Extra["trayPinned"]; raw != nil {
		if err := json.Unmarshal(raw, &pinned); err != nil {
			return err
		}
	}
	h.pinned = pinned
	return nil
}
