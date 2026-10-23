package app

import (
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

type backupState struct {
	Config     domain.WebDavConfigView
	Password   string
	Local      []domain.LocalBackupInfo
	Remote     []domain.RemoteBackupInfo
	Overview   domain.BackupOverview
	Loaded     bool
	ConfigOpen bool
}

func (s *Shell) loadBackups(remote bool) {
	s.task(func() (func(), error) {
		local, err := s.Backups.ListLocal(s.Context)
		if err != nil {
			return nil, err
		}
		targets, err := s.Service.ListBackups(s.Context)
		if err != nil {
			return nil, err
		}
		overview, err := s.Backups.Overview()
		if err != nil {
			return nil, err
		}
		var rows []domain.RemoteBackupInfo
		if remote {
			rows, err = s.Backups.List(s.Context)
			if err != nil {
				return nil, err
			}
		}
		return func() {
			s.BackupUI.Local = local
			s.TargetBackups = targets
			s.BackupUI.Overview = overview
			s.BackupUI.Loaded = true
			if remote {
				s.BackupUI.Remote = rows
			}
		}, nil
	})
}
func (s *Shell) createAppBackup(remote bool) {
	destination := "local"
	if remote {
		destination = "webdav"
	}
	s.task(func() (func(), error) {
		result, err := s.Backups.Backup(s.Context, destination, "manual")
		if err != nil {
			return nil, err
		}
		local, err := s.Backups.ListLocal(s.Context)
		if err != nil {
			return nil, err
		}
		overview, err := s.Backups.Overview()
		if err != nil {
			return nil, err
		}
		return func() {
			s.BackupUI.Local = local
			s.BackupUI.Overview = overview
			s.Notice = s.text("backup_created") + " · " + result.FileName
			if result.Warning != nil {
				s.Warning = s.text("backup_warning")
				s.WarningDetail = s.safeErrorDetail(*result.Warning)
			}
		}, nil
	})
}
func (s *Shell) deleteAppBackup(name string, remote bool) {
	s.confirmDanger(s.text("delete"), name, func() error {
		s.task(func() (func(), error) {
			var err error
			if remote {
				err = s.Backups.Delete(s.Context, name)
			} else {
				err = s.Backups.DeleteLocal(s.Context, name)
			}
			if err != nil {
				return nil, err
			}
			local, err := s.Backups.ListLocal(s.Context)
			if err != nil {
				return nil, err
			}
			var rows []domain.RemoteBackupInfo
			if remote {
				rows, err = s.Backups.List(s.Context)
				if err != nil {
					return nil, err
				}
			}
			return func() {
				s.BackupUI.Local = local
				if remote {
					s.BackupUI.Remote = rows
				}
			}, nil
		})
		return nil
	})
}
func (s *Shell) restoreAppBackup(name string, remote bool) {
	s.confirm(s.text("restore_app_backup"), name+"\n"+s.text("restore_relaunch_notice"), func() error {
		s.task(func() (func(), error) {
			var result domain.RestoreQueueResult
			var err error
			if remote {
				result, err = s.Backups.Restore(s.Context, name)
			} else {
				result, err = s.Backups.RestoreLocal(s.Context, name)
			}
			if err != nil {
				return nil, err
			}
			closeErr := s.Backups.Close()
			if closeErr != nil {
				result.Warning = pointer(s.safeErrorDetail(closeErr.Error()))
			}
			return func() {
				s.Notice = s.text("restore_queued")
				if result.Warning != nil {
					s.Warning = s.text("backup_warning")
					s.WarningDetail = s.safeErrorDetail(*result.Warning)
				}
				if result.RelaunchRequired {
					s.RestoreReady = true
				}
			}, nil
		})
		return nil
	})
}
func (s *Shell) saveWebDAV(test bool) {
	input := s.BackupUI.Config
	password := s.BackupUI.Password
	s.task(func() (func(), error) {
		if test {
			err := s.Backups.Test(s.Context, domain.TestWebDavConnectionInput{BaseURL: input.BaseURL, Username: input.Username, Password: optional(password), RemotePath: input.RemotePath, AcceptInvalidCerts: input.AcceptInvalidCerts})
			password = ""
			if err != nil {
				return nil, err
			}
			return func() { s.BackupUI.Password = ""; s.Notice = s.text("connection_ok") }, nil
		}
		view, err := s.Backups.SaveConfig(s.Context, domain.SaveWebDavConfigInput{BaseURL: input.BaseURL, Username: input.Username, Password: optional(password), RemotePath: input.RemotePath, AcceptInvalidCerts: input.AcceptInvalidCerts, AutoSyncEnabled: input.AutoSyncEnabled, SyncIntervalMinutes: input.SyncIntervalMinutes, MaxRemoteBackups: input.MaxRemoteBackups})
		password = ""
		if err != nil {
			return nil, err
		}
		overview, err := s.Backups.Overview()
		return func() {
			s.BackupUI.Config = view
			s.BackupUI.Password = ""
			s.BackupUI.Overview = overview
			s.Notice = s.text("webdav_saved")
		}, err
	})
}
func (s *Shell) closeManagers() error {
	s.CloseOnce.Do(func() {
		s.Cancel()
		s.Backups.Cancel()
		err := s.Backups.Close()
		s.Work.Wait()
		if s.Updates != nil {
			err = errors.Join(err, s.Updates.Close())
		}
		serviceErr := s.Service.Close()
		s.CloseError = errors.Join(err, serviceErr, s.ShutdownError)
	})
	return s.CloseError
}
