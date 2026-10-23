package webdav

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"os"
	"path/filepath"
)

func (m *Manager) Backup(ctx context.Context, destination, reason string) (result domain.BackupOperationResult, err error) {
	op, done, e := m.begin(ctx, destination == "webdav")
	if e != nil {
		return result, e
	}
	defer done()
	settings, e := m.store.GetSettings()
	if e != nil {
		return result, e
	}
	if destination == "local" {
		created, e := m.backups.Create(reason, settings.MaxBackupCopies)
		if e != nil {
			return result, e
		}
		return domain.BackupOperationResult{FileName: created.FileName, LocalPath: &created.Path, Warning: created.Warning}, nil
	}
	if destination != "webdav" {
		return result, domain.NewError("validation_failed", "backup destination must be local or webdav")
	}
	if _, e = m.store.RecordWebDAVSyncStatus("running", nil, false); e != nil {
		return result, e
	}
	defer func() {
		status := "success"
		message := result.Warning
		if err != nil {
			status = "failed"
			s := err.Error()
			message = &s
		} else if message != nil {
			status = "warning"
		}
		_, recordErr := m.store.RecordWebDAVSyncStatus(status, message, err == nil)
		err = errors.Join(err, recordErr)
	}()
	client, e := m.client()
	if e != nil {
		return result, e
	}
	config, configErr := m.store.GetWebDAVConfig()
	if configErr != nil {
		return result, configErr
	}
	temp, e := os.MkdirTemp(m.root, ".webdav-backup-")
	if e != nil {
		return result, e
	}
	defer func() {
		cleanupErr := os.RemoveAll(temp)
		if cleanupErr != nil && err == nil && result.Uploaded {
			message := "Backup uploaded; local staging cleanup failed: " + cleanupErr.Error()
			if result.Warning != nil {
				message = *result.Warning + "; " + message
			}
			result.Warning = &message
		} else {
			err = errors.Join(err, cleanupErr)
		}
	}()
	if e = fileops.SecureDirectory(temp); e != nil {
		return result, e
	}
	created, e := m.backups.CreateIn(temp, reason)
	if e != nil {
		return result, e
	}
	if e = client.Upload(op, created.FileName, created.Path); e != nil {
		return result, e
	}
	result.FileName = created.FileName
	result.Uploaded = true
	if _, e = client.Cleanup(op, config.MaxRemoteBackups); e != nil {
		warning := "Backup uploaded; remote cleanup failed: " + e.Error()
		result.Warning = &warning
	}
	return result, nil
}
func (m *Manager) ListLocal(ctx context.Context) ([]domain.LocalBackupInfo, error) {
	_, done, e := m.begin(ctx, false)
	if e != nil {
		return nil, e
	}
	defer done()
	return m.backups.List()
}
func (m *Manager) List(ctx context.Context) ([]domain.RemoteBackupInfo, error) {
	op, done, e := m.begin(ctx, false)
	if e != nil {
		return nil, e
	}
	defer done()
	client, e := m.client()
	if e != nil {
		return nil, e
	}
	return client.List(op)
}
func (m *Manager) DeleteLocal(ctx context.Context, name string) error {
	_, done, e := m.begin(ctx, false)
	if e != nil {
		return e
	}
	defer done()
	return m.backups.Delete(name)
}
func (m *Manager) Delete(ctx context.Context, name string) error {
	op, done, e := m.begin(ctx, false)
	if e != nil {
		return e
	}
	defer done()
	client, e := m.client()
	if e != nil {
		return e
	}
	return client.Delete(op, name)
}
func (m *Manager) RestoreLocal(ctx context.Context, name string) (domain.RestoreQueueResult, error) {
	_, done, e := m.begin(ctx, false)
	if e != nil {
		return domain.RestoreQueueResult{}, e
	}
	defer done()
	settings, e := m.store.GetSettings()
	if e != nil {
		return domain.RestoreQueueResult{}, e
	}
	return m.backups.QueueLocalRestore(name, settings.MaxBackupCopies)
}
func (m *Manager) Restore(ctx context.Context, name string) (result domain.RestoreQueueResult, err error) {
	op, done, e := m.begin(ctx, false)
	if e != nil {
		return result, e
	}
	defer done()
	client, e := m.client()
	if e != nil {
		return result, e
	}
	temp, e := os.MkdirTemp(m.root, ".webdav-restore-")
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, os.RemoveAll(temp)) }()
	if e = fileops.SecureDirectory(temp); e != nil {
		return result, e
	}
	archive := filepath.Join(temp, "backup.zip")
	if e = client.Download(op, name, archive); e != nil {
		return result, e
	}
	settings, e := m.store.GetSettings()
	if e != nil {
		return result, e
	}
	return m.backups.QueueRestore(archive, settings.MaxBackupCopies)
}
