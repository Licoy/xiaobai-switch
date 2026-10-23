package webdav

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"strings"
)

func (m *Manager) client() (*Client, error) {
	stored, e := m.store.StoredWebDAVConfig()
	if e != nil {
		return nil, e
	}
	if stored == nil {
		return nil, domain.NewError("webdav_not_configured", "WebDAV is not configured")
	}
	password, e := m.store.GetWebDAVPassword()
	if e != nil {
		return nil, e
	}
	settings, e := m.store.GetSettings()
	if e != nil {
		return nil, e
	}
	v := stored.View
	return NewClient(RuntimeConfig{BaseURL: v.BaseURL, Username: v.Username, Password: password, RemotePath: v.RemotePath, AcceptInvalidCerts: v.AcceptInvalidCerts}, settings)
}
func (m *Manager) SaveConfig(ctx context.Context, in domain.SaveWebDavConfigInput) (domain.WebDavConfigView, error) {
	_, done, e := m.begin(ctx, true)
	if e != nil {
		return domain.WebDavConfigView{}, e
	}
	defer done()
	if e = ValidateInterval(in.SyncIntervalMinutes); e != nil {
		return domain.WebDavConfigView{}, e
	}
	if e = ValidateRetention(in.MaxRemoteBackups); e != nil {
		return domain.WebDavConfigView{}, e
	}
	password := ""
	if in.Password != nil && *in.Password != "" {
		password = *in.Password
	} else {
		password, e = m.store.GetWebDAVPassword()
		if e != nil || password == "" {
			return domain.WebDavConfigView{}, domain.NewError("validation_failed", "WebDAV password is required for first configuration")
		}
	}
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.Username = strings.TrimSpace(in.Username)
	in.RemotePath = strings.Trim(strings.TrimSpace(in.RemotePath), "/")
	settings, e := m.store.GetSettings()
	if e != nil {
		return domain.WebDavConfigView{}, e
	}
	if _, e = NewClient(RuntimeConfig{BaseURL: in.BaseURL, Username: in.Username, Password: password, RemotePath: in.RemotePath, AcceptInvalidCerts: in.AcceptInvalidCerts}, settings); e != nil {
		return domain.WebDavConfigView{}, e
	}
	in.Password = &password
	return m.store.SaveWebDAVConfig(in)
}
func (m *Manager) Test(ctx context.Context, in domain.TestWebDavConnectionInput) error {
	op, done, e := m.begin(ctx, false)
	if e != nil {
		return e
	}
	defer done()
	password := ""
	if in.Password != nil && *in.Password != "" {
		password = *in.Password
	} else {
		password, e = m.store.GetWebDAVPassword()
		if e != nil {
			return e
		}
	}
	settings, e := m.store.GetSettings()
	if e != nil {
		return e
	}
	client, e := NewClient(RuntimeConfig{BaseURL: in.BaseURL, Username: in.Username, Password: password, RemotePath: in.RemotePath, AcceptInvalidCerts: in.AcceptInvalidCerts}, settings)
	if e != nil {
		return e
	}
	return client.Check(op)
}
