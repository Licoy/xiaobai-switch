package app

import (
	"context"
	"github.com/egoist/mygo"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func probeResultKey(site domain.SiteDTO, modelID string) string {
	return quotaKey(site) + "\x00" + modelID
}
func (s *Shell) batchProbe(site domain.SiteDTO) {
	if s.Busy || s.Closing || s.writesBlocked() {
		return
	}
	ids := []string{}
	for _, model := range s.Models {
		ids = append(ids, model.ModelID)
	}
	ctx, cancel := context.WithCancel(s.Context)
	s.BatchCancel = cancel
	s.BatchProgress = 0
	s.BatchTotal = len(ids)
	s.task(func() (func(), error) {
		defer cancel()
		for _, id := range ids {
			if ctx.Err() != nil {
				break
			}
			result, err := s.Service.ProbeModel(ctx, site.ID, value(site.ActiveAPIKeyID), id)
			if err != nil {
				message := err.Error()
				result = domain.ModelProbeResult{ModelID: id, Error: &message}
			}
			mygo.RunOnMain(func() {
				if s.Closing {
					return
				}
				if result.Error != nil {
					message := s.safeErrorDetail(*result.Error)
					result.Error = &message
				}
				s.ModelProbes[probeResultKey(site, id)] = result
				s.BatchProgress++
				if s.Invalidate != nil {
					s.Invalidate()
				}
			})
		}
		cancelled := ctx.Err() != nil
		return func() {
			s.BatchCancel = nil
			if cancelled {
				s.Notice = s.text("cancelled")
			} else {
				s.Notice = s.text("batch_complete")
			}
		}, nil
	})
}
