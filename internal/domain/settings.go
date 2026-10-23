package domain

import (
	"encoding/json"
	"reflect"
	"strings"
)

func DefaultSettings() AppSettings {
	return AppSettings{Language: "zh-CN", ThemeMode: "system", PrimaryColor: "#0C76F8", CodexEnvInjectMode: "auto", AutoCheckUpdate: true, UpdateCheckInterval: 60, MaxBackupCopies: 30, ProxyMode: "system", ProxyProtocol: "http", RouteProbeTtlMinutes: 10, CloseToTray: true}
}

type settingsAlias AppSettings

func (s *AppSettings) UnmarshalJSON(b []byte) error {
	v := settingsAlias(DefaultSettings())
	if e := json.Unmarshal(b, &v); e != nil {
		return e
	}
	var extra map[string]json.RawMessage
	if e := json.Unmarshal(b, &extra); e != nil {
		return e
	}
	rt := reflect.TypeOf(settingsAlias{})
	for i := 0; i < rt.NumField(); i++ {
		tag := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]
		if tag != "" && tag != "-" {
			delete(extra, tag)
		}
	}
	*s = AppSettings(v)
	s.Extra = extra
	return nil
}
func (s AppSettings) MarshalJSON() ([]byte, error) {
	b, e := json.Marshal(settingsAlias(s))
	if e != nil {
		return nil, e
	}
	var out map[string]json.RawMessage
	if e = json.Unmarshal(b, &out); e != nil {
		return nil, e
	}
	for k, v := range s.Extra {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
	return json.Marshal(out)
}
func NormalizeSettings(s AppSettings) AppSettings {
	if strings.EqualFold(s.PrimaryColor, "#1677ff") {
		s.PrimaryColor = "#0C76F8"
	}
	s.MaxBackupCopies = min(200, max(1, s.MaxBackupCopies))
	s.RouteProbeTtlMinutes = min(1440, max(1, s.RouteProbeTtlMinutes))
	// Keep the legacy field compatible while native checks always run hourly.
	s.UpdateCheckInterval = 60
	if s.ProxyMode != "custom" && s.ProxyMode != "none" {
		s.ProxyMode = "system"
	}
	s.ProxyProtocol = strings.ToLower(strings.TrimSpace(s.ProxyProtocol))
	if s.ProxyProtocol != "https" && s.ProxyProtocol != "socks5" {
		s.ProxyProtocol = "http"
	}
	if s.ProxyHost != nil {
		host := strings.TrimSpace(*s.ProxyHost)
		s.ProxyHost = nil
		if host != "" {
			s.ProxyHost = &host
		}
	}
	// Explicit launches always show the window; the legacy field maps only to startup
	if s.LaunchAtLoginHidden == nil && s.StartInTray {
		v := true
		s.LaunchAtLoginHidden = &v
	}
	s.StartInTray = false
	return s
}
