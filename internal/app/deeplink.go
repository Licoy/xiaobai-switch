package app

import (
	"github.com/licoy/xiaobai-switch/internal/deeplink"
	"github.com/licoy/xiaobai-switch/internal/secure"
	"sort"
	"strings"
)

func (h *host) importURL(raw string) {
	s := h.shell
	if s.writesBlocked() || s.Closing {
		return
	}
	input, err := deeplink.Parse(raw)
	if err != nil {
		s.result(err)
		return
	}
	if input.APIKey == "" {
		s.Current = "sites"
		s.editSite(nil)
		editor := s.Editor
		editor.Name = input.Name
		editor.URLs = strings.Join(input.BaseURLs, "\n")
		if input.Protocol != nil {
			editor.Protocol = *input.Protocol
		}
		editor.Notes = value(input.Notes)
		if input.Capabilities != nil {
			for key, on := range *input.Capabilities {
				editor.Capabilities[key] = on
			}
		}
		if s.Invalidate != nil {
			s.Invalidate()
		}
		return
	}
	detail := input.Name + "\n" + strings.Join(input.BaseURLs, "\n") + "\n" + s.text("protocol") + ": " + value(input.Protocol) + "\n" + s.text("notes") + ": " + value(input.Notes) + "\nAPI key: " + secure.KeyPrefix(input.APIKey)
	if input.Capabilities != nil {
		keys := []string{}
		for key := range *input.Capabilities {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			on := s.text("disabled")
			if (*input.Capabilities)[key] {
				on = s.text("enabled")
			}
			detail += "\n" + key + ": " + on
		}
	}
	detail += "\n" + s.text("import_warning")
	s.confirm(s.text("import_site"), detail, func() error {
		result, err := deeplink.Import(s.Store, input)
		input.APIKey = ""
		if err != nil {
			return err
		}
		s.Current = "sites"
		s.Selected = result.Site.ID
		return s.reload()
	})
	s.Confirm.Secret = true
	s.Confirm.Dismiss = func() { input.APIKey = "" }
	if s.Invalidate != nil {
		s.Invalidate()
	}
}
