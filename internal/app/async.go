package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/egoist/mygo"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/service"
)

func (s *Shell) task(work func() (func(), error)) {
	if s.Busy || s.Closing || s.writesBlocked() {
		return
	}
	s.Busy = true
	s.Notice = ""
	s.Error = ""
	s.ErrorDetail = ""
	s.Warning = ""
	s.WarningDetail = ""
	s.OperationResults = nil
	s.Work.Add(1)
	go func() {
		defer s.Work.Done()
		done, err := work()
		var app *domain.AppError
		changed := errors.As(err, &app) && (app.Code == "partial_failure" || app.Code == "committed_with_warning")
		var statuses []service.TargetStatus
		if changed {
			fresh, statusErr := s.Service.Status(s.Context)
			statuses = fresh
			err = errors.Join(err, statusErr)
		}
		mygo.RunOnMain(func() {
			if s.Closing {
				if err != nil && !errors.Is(err, context.Canceled) {
					s.ShutdownError = errors.Join(s.ShutdownError, err)
				}
				return
			}
			s.Busy = false
			s.Error = ""
			s.Notice = ""
			if changed {
				s.acceptStatuses(statuses)
				reloadErr := s.reload()
				if app.Details != nil {
					if app.Code == "partial_failure" {
						if parseErr := json.Unmarshal([]byte(*app.Details), &s.OperationResults); parseErr != nil {
							reloadErr = errors.Join(reloadErr, parseErr)
						}
					}
					if app.Code == "committed_with_warning" {
						var envelope struct {
							Committed bool                `json:"committed"`
							Result    *domain.ApplyResult `json:"result"`
						}
						if parseErr := json.Unmarshal([]byte(*app.Details), &envelope); parseErr != nil {
							reloadErr = errors.Join(reloadErr, parseErr)
						} else if !envelope.Committed {
							reloadErr = errors.Join(reloadErr, errors.New("invalid committed result"))
						} else if envelope.Result != nil {
							s.OperationResults = envelope.Result.Results
						}
					}
				}
				if app.Code == "committed_with_warning" {
					s.Warning = s.text("committed_warning")
					s.WarningDetail = s.safeErrorDetail(app.Message)
					if reloadErr != nil {
						s.result(reloadErr)
					}
				} else {
					s.result(errors.Join(err, reloadErr))
				}
			} else if err != nil {
				reloadErr := s.reload()
				s.result(errors.Join(err, reloadErr))
			} else if done != nil {
				done()
			}
			if s.Invalidate != nil {
				s.Invalidate()
			}
		})
	}()
}
