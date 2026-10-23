package app

import (
	"github.com/egoist/mygo"
	"github.com/licoy/xiaobai-switch/internal/update"
)

func (s *Shell) backupChanged() {
	if !mygo.App.IsReady() {
		return
	}
	go mygo.RunOnMain(func() {
		if !s.Closing {
			s.queueBackupRefresh()
		}
	})
}

// Refresh only local database summaries, never start a network operation from a notification
func (s *Shell) queueBackupRefresh() {
	if s.Closing {
		return
	}
	if s.BackupRefreshing {
		s.BackupRefreshPending = true
		return
	}
	s.BackupRefreshing = true
	s.Work.Add(1)
	go func() {
		defer s.Work.Done()
		overview, err := s.Backups.Overview()
		local, listErr := s.Backups.ListLocal(s.Context)
		mygo.RunOnMain(func() {
			s.BackupRefreshing = false
			if s.Closing {
				return
			}
			if err != nil {
				s.result(err)
			} else {
				s.BackupUI.Overview = overview
			}
			if listErr != nil {
				s.result(listErr)
			} else {
				s.BackupUI.Local = local
			}
			if s.BackupRefreshPending {
				s.BackupRefreshPending = false
				s.queueBackupRefresh()
			}
			if s.Invalidate != nil {
				s.Invalidate()
			}
		})
	}()
}
func (s *Shell) updateChanged(_ update.State) {
	if !mygo.App.IsReady() {
		return
	}
	go mygo.RunOnMain(func() {
		if s.Closing {
			return
		}
		if s.Invalidate != nil {
			s.Invalidate()
		}
	})
}
