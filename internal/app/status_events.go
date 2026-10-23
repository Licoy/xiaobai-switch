package app

import (
	"errors"
	"github.com/egoist/mygo"
)

func (s *Shell) serviceChanged() {
	if !mygo.App.IsReady() {
		return
	}
	go mygo.RunOnMain(func() {
		if s.Closing {
			return
		}
		s.queueStatusRefresh()
	})
}
func (s *Shell) queueStatusRefresh() {
	if s.Closing {
		return
	}
	if s.StatusRefreshing {
		s.StatusRefreshPending = true
		return
	}
	s.StatusRefreshing = true
	s.Work.Add(1)
	go func() {
		defer s.Work.Done()
		rows, err := s.Service.Status(s.Context)
		mygo.RunOnMain(func() {
			s.StatusRefreshing = false
			if s.Closing {
				if err != nil {
					s.ShutdownError = errors.Join(s.ShutdownError, err)
				}
				return
			}
			if err != nil {
				s.result(err)
			} else {
				s.acceptStatuses(rows)
			}
			if s.StatusRefreshPending {
				s.StatusRefreshPending = false
				s.queueStatusRefresh()
			}
			if s.Invalidate != nil {
				s.Invalidate()
			}
		})
	}()
}
