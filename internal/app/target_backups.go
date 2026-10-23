package app

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"path/filepath"
	"strings"
	"time"
)

func (s *Shell) targetBackupView(c *ui.Context) {
	Panel(c).Children(func() {
		PanelTitle(c, icStack, TintOrange, s.text("target_backups"), fmt.Sprintf("%d", len(s.TargetBackups)), false, func() {
			if PillButton(c, icFolder, s.text("open_backup_dir"), false).Clicked() {
				s.openDirectory(filepath.Join(s.Config.DataDir, "backups"))
			}
			if BusyPillButton(c, icRefresh, s.text("refresh"), false, s.Busy).Clicked() {
				s.loadTargetBackups()
			}
		})
		if len(s.TargetBackups) == 0 {
			EmptyState(c, icStack, TintOrange, s.text("no_backups"), s.text("target_backups_detail"))
			return
		}
		t := c.Theme()
		s.backupList(c, len(s.TargetBackups), func(i int) {
			backup := s.TargetBackups[i]
			target := domain.TargetKind(backup.Target)
			ui.Row(c).FillWidth().Gap(10).Padding(8, 0).AlignItems(ui.Center).Children(func() {
				targetTile(c, target, 28)
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, targetLabel(target)+" · "+time.UnixMilli(backup.CreatedAt).Format("2006-01-02 15:04")).FontSize(13).FontWeight(600).SingleLine()
					mono(c, strings.Join(backup.Files, ", ")).TextColor(t.TextMuted).MaxLines(2)
				})
				if PillButton(c, icRestart, s.text("restore_backup"), s.Busy).Clicked() {
					s.previewTargetBackup(backup)
				}
				if DangerPillButton(c, icDelete, "", s.Busy).Label(s.text("delete")).Tooltip(s.text("delete")).Clicked() {
					s.confirmDanger(s.text("delete"), targetLabel(target)+" · "+backup.ID, func() error {
						s.task(func() (func(), error) {
							if err := s.Service.DeleteBackup(s.Context, backup.ID); err != nil {
								return nil, err
							}
							rows, err := s.Service.ListBackups(s.Context)
							return func() { s.TargetBackups = rows }, err
						})
						return nil
					})
				}
			})
		})
	})
}
func (s *Shell) loadTargetBackups() {
	s.task(func() (func(), error) {
		rows, err := s.Service.ListBackups(s.Context)
		return func() { s.TargetBackups = rows }, err
	})
}
func (s *Shell) previewTargetBackup(backup domain.BackupInfo) {
	s.task(func() (func(), error) {
		preview, err := s.Service.PreviewBackup(s.Context, backup.ID)
		if err != nil {
			return nil, err
		}
		return func() {
			names := []string{}
			for _, f := range preview.Files {
				names = append(names, f.Name)
			}
			s.confirm(s.text("restore_backup"), backup.ID+"\n"+strings.Join(names, "\n"), func() error {
				s.serviceChange(func(ctx context.Context) error { return s.Service.RestoreBackup(ctx, backup.ID) })
				return nil
			})
		}, nil
	})
}
