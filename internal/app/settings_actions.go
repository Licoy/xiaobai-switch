package app

import (
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/l10n"
	"github.com/licoy/xiaobai-switch/internal/paths"
)

func validateSettings(v domain.AppSettings) error {
	if !hexColor.MatchString(v.PrimaryColor) {
		return domain.NewError("validation_failed", "invalid primary color")
	}
	for _, dir := range []*string{v.ClaudeHomeOverride, v.CodexHomeOverride, v.PiAgentDirOverride, v.PrimeAgentDirOverride} {
		if dir != nil && *dir != "" {
			if _, err := paths.ValidatePath(*dir); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Shell) saveSettings(next domain.AppSettings) {
	if s.writesBlocked() {
		saved, err := s.Store.GetSettings()
		if err != nil {
			s.result(err)
		} else {
			s.Settings = saved
		}
		return
	}
	old, err := s.Service.GetSettings(s.Context)
	if err != nil {
		s.result(err)
		return
	}
	if err = validateSettings(next); err != nil {
		s.Settings = old
		s.result(err)
		return
	}
	if s.SettingsChanged != nil {
		if err = s.SettingsChanged(next); err != nil {
			s.Settings = old
			s.result(errors.Join(err, s.SettingsChanged(old)))
			return
		}
	}
	if err = s.Service.SaveSettings(s.Context, next); err != nil {
		s.Settings = old
		if s.SettingsChanged != nil {
			err = errors.Join(err, s.SettingsChanged(old))
		}
		s.result(err)
		return
	}
	saved, err := s.Service.GetSettings(s.Context)
	if err != nil {
		s.result(err)
		return
	}
	s.Settings = saved
	catalog, err := l10n.Load(saved.Language)
	if err != nil {
		s.result(err)
		return
	}
	s.Config.Messages = catalog
	if s.Updates != nil {
		s.Updates.SettingsChanged()
	}
	s.result(nil)
}

func loginHidden(settings domain.AppSettings) bool {
	if settings.LaunchAtLoginHidden != nil {
		return *settings.LaunchAtLoginHidden
	}
	return settings.StartInTray
}
