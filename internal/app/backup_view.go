package app

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"
)

func formatSize(n uint64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
func (s *Shell) backupView(c *ui.Context) {
	t := c.Theme()
	o := s.BackupUI.Overview
	remote := "—"
	if len(s.BackupUI.Remote) > 0 {
		remote = fmt.Sprintf("%d", len(s.BackupUI.Remote))
	}
	next := "—"
	if o.NextScheduledAt != nil {
		next = time.UnixMilli(*o.NextScheduledAt).Format("01-02 15:04")
	}
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		Stat(c, icArchive, TintGreen, s.text("local_backups"), fmt.Sprintf("%d", len(s.BackupUI.Local)))
		Stat(c, icCloud, TintBlue, s.text("remote_backups"), remote)
		Stat(c, icStack, TintOrange, s.text("target_backups"), fmt.Sprintf("%d", len(s.TargetBackups)))
		Stat(c, icTime, TintPurple, s.text("next_backup"), next)
	})
	if s.RestoreReady {
		Banner(c, icRestart, t.Warning, s.text("restore_queued"))
	}
	Panel(c).Children(func() {
		PanelTitle(c, icArchive, TintGreen, s.text("application_backups"), "", false, func() {
			if PillButton(c, icFolder, s.text("open_backup_dir"), false).Clicked() {
				s.openDirectory(filepath.Join(s.Config.DataDir, "backups", "app"))
			}
			if BusyPillButton(c, icRefresh, s.text("refresh"), false, s.Busy).Clicked() {
				s.loadBackups(false)
			}
			if AccentPillButton(c, icAdd, s.text("create_local_backup"), s.Busy).Clicked() {
				s.createAppBackup(false)
			}
		})
		optionRows(c, func(row func(label, detail string, control func())) {
			row(s.text("max_backups"), s.text("max_backups_detail"), func() { s.settingNumber(c, "max_backups", &s.Settings.MaxBackupCopies, 1, 200) })
		})
		if len(s.BackupUI.Local) == 0 {
			EmptyState(c, icArchive, TintGreen, s.text("no_backups"), "")
			return
		}
		s.backupList(c, len(s.BackupUI.Local), func(i int) {
			row := s.BackupUI.Local[i]
			s.backupRow(c, icArchive, TintGreen, row.FileName, fmt.Sprintf("%s · %s · %s", time.UnixMilli(row.CreatedAt).Format("2006-01-02 15:04"), row.DeviceName, formatSize(row.Size)), row.Error, func() {
				if PillButton(c, icRestart, s.text("restore_app_backup"), s.Busy || row.Error != nil).Clicked() {
					s.restoreAppBackup(row.FileName, false)
				}
				if DangerPillButton(c, icDelete, "", s.Busy).Label(s.text("delete")).Tooltip(s.text("delete")).Clicked() {
					s.deleteAppBackup(row.FileName, false)
				}
			})
		})
	})
	Panel(c).Children(func() {
		status, col := s.text("webdav_not_configured"), TintGray.C(t)
		if o.WebdavConfigured {
			status, col = s.text("webdav_configured"), t.Success
			if label, ok := s.Config.Messages["sync_"+o.WebdavSync.Status]; ok && o.WebdavSync.Status != "" {
				status = label
				if o.WebdavSync.Status == "failed" {
					col = t.Danger
				} else if o.WebdavSync.Status == "warning" {
					col = t.Warning
				}
			}
		}
		PanelTitle(c, icCloud, TintBlue, "WebDAV", "", false, func() {
			DotPill(c, status, col)
		})
		if o.WebdavSync.Error != nil {
			Banner(c, icWarning, t.Warning, s.safeErrorDetail(*o.WebdavSync.Error))
		}
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			if AccentPillButton(c, icUpload, s.text("backup_to_webdav"), s.Busy).Clicked() {
				s.createAppBackup(true)
			}
			if PillButton(c, icRefresh, s.text("list_remote"), s.Busy).Clicked() {
				s.loadBackups(true)
			}
			if s.Busy && PillButton(c, icClose, s.text("cancel"), false).Clicked() {
				s.Backups.Cancel()
			}
		})
		s.webDAVConfigView(c)
		if len(s.BackupUI.Remote) > 0 {
			s.backupList(c, len(s.BackupUI.Remote), func(i int) {
				row := s.BackupUI.Remote[i]
				s.backupRow(c, icCloud, TintBlue, row.FileName, row.LastModified+" · "+row.DeviceName+" · "+formatSize(row.Size), nil, func() {
					if PillButton(c, icDownload, s.text("restore_app_backup"), s.Busy).Clicked() {
						s.restoreAppBackup(row.FileName, true)
					}
					if DangerPillButton(c, icDelete, "", s.Busy).Label(s.text("delete")).Tooltip(s.text("delete")).Clicked() {
						s.deleteAppBackup(row.FileName, true)
					}
				})
			})
		}
	})
	s.targetBackupView(c)
}
func (s *Shell) backupList(c *ui.Context, n int, row func(i int)) {
	ui.Column(c).FillWidth().Gap(0).Padding(2, 12).Radius(12).Background(pal(c.Theme()).Inset).Children(func() {
		for i := 0; i < n; i++ {
			if i > 0 {
				Divider(c)
			}
			ui.Column(c.Key(i)).FillWidth().Children(func() { row(i) })
		}
	})
}
func (s *Shell) backupRow(c *ui.Context, icon *ui.SVG, k Tint, name, detail string, failure *string, actions func()) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(10).Padding(8, 0).AlignItems(ui.Center).Children(func() {
		Tile(c, icon, k, 28, false)
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			ui.Text(c, name).FontSize(13).FontWeight(600).SingleLine().Selectable()
			ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted).SingleLine()
			if failure != nil {
				ui.Text(c, s.safeErrorDetail(*failure)).FontSize(11).TextColor(t.Warning).MaxLines(2)
			}
		})
		actions()
	})
}
func (s *Shell) webDAVConfigView(c *ui.Context) {
	t := c.Theme()
	v := &s.BackupUI.Config
	disclosure(c, "webdav-config", icSettings, s.text("webdav_configuration"), &s.BackupUI.ConfigOpen, func() {
		grid(c, 2, 4, 12, func(i int) {
			switch i {
			case 0:
				field(c, "Base URL", func() { ui.TextInput(c, &v.BaseURL).FillWidth().Placeholder("https://dav.example.com") })
			case 1:
				field(c, s.text("remote_path"), func() { ui.TextInput(c, &v.RemotePath).FillWidth() })
			case 2:
				field(c, s.text("username"), func() { ui.TextInput(c, &v.Username).FillWidth() })
			case 3:
				field(c, s.text("password"), func() { ui.TextInput(c, &s.BackupUI.Password).Password().FillWidth() })
			}
		})
		optionRows(c, func(row func(label, detail string, control func())) {
			row(s.text("accept_invalid_certificates"), "", func() {
				ui.Switch(c.Key("invalid-certs"), &v.AcceptInvalidCerts).Label(s.text("accept_invalid_certificates"))
			})
			row(s.text("auto_sync"), "", func() { ui.Switch(c.Key("auto-sync"), &v.AutoSyncEnabled).Label(s.text("auto_sync")) })
			row(s.text("sync_interval"), "", func() {
				n := float64(v.SyncIntervalMinutes)
				if ui.NumberInput(c.Key("interval"), &n, 1, 1440, 1).Label(s.text("sync_interval")).Changed() {
					v.SyncIntervalMinutes = uint32(n)
				}
			})
			row(s.text("remote_retention"), "", func() {
				n := float64(v.MaxRemoteBackups)
				if ui.NumberInput(c.Key("retention"), &n, 1, 200, 1).Label(s.text("remote_retention")).Changed() {
					v.MaxRemoteBackups = uint32(n)
				}
			})
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icLock).Size(14, 14).TextColor(t.TextMuted)
			ui.Text(c, s.text("webdav_password_notice")).FontSize(11).TextColor(t.TextMuted).Grow(1).Basis(0).Wrap()
			if PillButton(c, icPulse, s.text("test_connection"), s.Busy).Clicked() {
				s.saveWebDAV(true)
			}
			if AccentPillButton(c, icSave, s.text("save"), s.Busy).Clicked() {
				s.saveWebDAV(false)
			}
		})
	})
}
