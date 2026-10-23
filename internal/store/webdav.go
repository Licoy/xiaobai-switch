package store

import (
	"database/sql"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"net/url"
	"strings"
)

type StoredWebDavConfig struct {
	View              domain.WebDavConfigView
	PasswordEncrypted string
}

func defaultWebDAV() domain.WebDavConfigView {
	return domain.WebDavConfigView{RemotePath: "xiaobai-switch", SyncIntervalMinutes: 60, MaxRemoteBackups: 10}
}
func (s *Store) GetWebDAVConfig() (domain.WebDavConfigView, error) {
	c, e := s.StoredWebDAVConfig()
	if e != nil {
		return domain.WebDavConfigView{}, e
	}
	if c == nil {
		return defaultWebDAV(), nil
	}
	return c.View, nil
}
func (s *Store) StoredWebDAVConfig() (*StoredWebDavConfig, error) {
	c := StoredWebDavConfig{}
	v := &c.View
	e := s.db.QueryRow(`SELECT base_url,username,password_encrypted,remote_path,accept_invalid_certs,auto_sync_enabled,sync_interval_minutes,max_remote_backups FROM webdav_config WHERE id=1`).Scan(&v.BaseURL, &v.Username, &c.PasswordEncrypted, &v.RemotePath, &v.AcceptInvalidCerts, &v.AutoSyncEnabled, &v.SyncIntervalMinutes, &v.MaxRemoteBackups)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v.HasPassword = c.PasswordEncrypted != ""
	return &c, nil
}
func (s *Store) GetWebDAVPassword() (string, error) {
	c, e := s.StoredWebDAVConfig()
	if e != nil {
		return "", e
	}
	if c == nil {
		return "", domain.NewError("webdav_not_configured", "WebDAV is not configured")
	}
	if c.PasswordEncrypted == "" {
		return "", nil
	}
	return s.crypto.Decrypt(c.PasswordEncrypted)
}
func (s *Store) SaveWebDAVConfig(in domain.SaveWebDavConfigInput) (domain.WebDavConfigView, error) {
	u, e := url.Parse(in.BaseURL)
	if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return domain.WebDavConfigView{}, domain.NewError("validation_failed", "WebDAV base URL must be HTTP(S) without credentials")
	}
	if in.SyncIntervalMinutes < 1 || in.MaxRemoteBackups < 1 {
		return domain.WebDavConfigView{}, domain.NewError("validation_failed", "WebDAV intervals and retention must be positive")
	}
	if strings.Contains(in.RemotePath, "..") || strings.Contains(in.RemotePath, "\\") {
		return domain.WebDavConfigView{}, domain.NewError("validation_failed", "invalid WebDAV remote path")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.StoredWebDAVConfig()
	if e != nil {
		return domain.WebDavConfigView{}, e
	}
	encrypted := ""
	if c != nil {
		encrypted = c.PasswordEncrypted
	}
	if in.Password != nil && *in.Password != "" {
		if *in.Password == "" {
			encrypted = ""
		} else {
			encrypted, e = s.crypto.Encrypt(*in.Password)
			if e != nil {
				return domain.WebDavConfigView{}, e
			}
		}
	}
	_, e = s.db.Exec(`INSERT INTO webdav_config(id,base_url,username,password_encrypted,remote_path,accept_invalid_certs,auto_sync_enabled,sync_interval_minutes,max_remote_backups,updated_at) VALUES(1,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET base_url=excluded.base_url,username=excluded.username,password_encrypted=excluded.password_encrypted,remote_path=excluded.remote_path,accept_invalid_certs=excluded.accept_invalid_certs,auto_sync_enabled=excluded.auto_sync_enabled,sync_interval_minutes=excluded.sync_interval_minutes,max_remote_backups=excluded.max_remote_backups,updated_at=excluded.updated_at`, in.BaseURL, in.Username, encrypted, in.RemotePath, in.AcceptInvalidCerts, in.AutoSyncEnabled, in.SyncIntervalMinutes, in.MaxRemoteBackups, now())
	if e != nil {
		return domain.WebDavConfigView{}, e
	}
	return s.GetWebDAVConfig()
}
func (s *Store) GetWebDAVSyncStatus() (domain.WebDavSyncStatus, error) {
	v := domain.WebDavSyncStatus{Status: "never"}
	e := s.db.QueryRow("SELECT last_attempt_at,last_success_at,status,error FROM webdav_sync_state WHERE id=1").Scan(&v.LastAttemptAt, &v.LastSuccessAt, &v.Status, &v.Error)
	if e == sql.ErrNoRows {
		return v, nil
	}
	return v, e
}
func (s *Store) RecordWebDAVSyncStatus(status string, message *string, success bool) (domain.WebDavSyncStatus, error) {
	ts := now()
	var succeeded *int64
	if success {
		succeeded = &ts
	}
	_, e := s.db.Exec(`INSERT INTO webdav_sync_state(id,last_attempt_at,last_success_at,status,error) VALUES(1,?,?,?,?) ON CONFLICT(id) DO UPDATE SET last_attempt_at=excluded.last_attempt_at,last_success_at=COALESCE(excluded.last_success_at,webdav_sync_state.last_success_at),status=excluded.status,error=excluded.error`, ts, succeeded, status, message)
	if e != nil {
		return domain.WebDavSyncStatus{}, e
	}
	return s.GetWebDAVSyncStatus()
}
