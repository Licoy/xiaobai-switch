package app

import (
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func (s *Shell) fetchModels(site domain.SiteDTO) {
	s.task(func() (func(), error) {
		result, err := s.Service.FetchModels(s.Context, site.ID, value(site.ActiveAPIKeyID))
		if err != nil {
			return nil, err
		}
		return func() {
			if err := s.reload(); err != nil {
				s.result(err)
				return
			}
			s.Notice = countMessage(s.text("models"), len(result.Models))
		}, nil
	})
}
func (s *Shell) probeKey(site domain.SiteDTO) {
	s.task(func() (func(), error) {
		result, err := s.Service.ProbeKey(s.Context, site.ID, value(site.ActiveAPIKeyID))
		if err != nil {
			return nil, err
		}
		return func() {
			s.Notice = fmt.Sprintf("%s · %d · %d ms", s.text("models"), result.ModelCount, result.LatencyMS)
		}, nil
	})
}
func (s *Shell) probeModel(site domain.SiteDTO, modelID string) {
	s.task(func() (func(), error) {
		result, err := s.Service.ProbeModel(s.Context, site.ID, value(site.ActiveAPIKeyID), modelID)
		if err != nil {
			return nil, err
		}
		if !result.Ok {
			return nil, domain.NewError("probe_failed", value(result.Error))
		}
		return func() {
			s.ModelProbes[probeResultKey(site, modelID)] = result
			s.Notice = fmt.Sprintf("%s · %s · %d ms", s.text("probe_ok"), modelID, result.LatencyMS)
		}, nil
	})
}
func (s *Shell) probeQuota(site domain.SiteDTO) {
	s.task(func() (func(), error) {
		result, err := s.Service.ProbeQuota(s.Context, site.ID, value(site.ActiveAPIKeyID))
		if err != nil {
			return func() {
				message := s.safeErrorDetail(err.Error())
				s.recordQuota(site, domain.SiteQuota{Status: "error", Error: &message})
				s.result(err)
			}, nil
		}
		return func() { s.recordQuota(site, result) }, nil
	})
}
