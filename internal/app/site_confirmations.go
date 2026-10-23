package app

import (
	"context"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"strings"
)

func (s *Shell) affectedTargets(siteID string) (string, error) {
	bindings, err := s.Store.ListBindings()
	if err != nil {
		return "", err
	}
	names := []string{}
	for _, b := range bindings {
		if value(b.SiteID) == siteID {
			names = append(names, targetLabel(b.Target))
		}
	}
	if len(names) == 0 {
		return s.text("no_affected_targets"), nil
	}
	return s.text("affected_targets") + ": " + strings.Join(names, ", "), nil
}
func (s *Shell) routeChange(siteID, route string) {
	affected, err := s.affectedTargets(siteID)
	if err != nil {
		s.result(err)
		return
	}
	prompt := &confirmation{Title: s.text("switch_route"), Detail: route + "\n" + affected, ToggleLabel: s.text("apply_route_targets"), Toggle: true}
	prompt.Action = func() error {
		s.task(func() (func(), error) {
			result, err := s.Service.SwitchRouteWithOptions(s.Context, siteID, route, prompt.Toggle)
			if err != nil {
				return nil, err
			}
			return func() {
				s.OperationResults = result.Results
				if err := s.reload(); err != nil {
					s.result(err)
				}
			}, nil
		})
		return nil
	}
	s.Confirm = prompt
}
func (s *Shell) deleteSite(site domain.SiteDTO) {
	affected, err := s.affectedTargets(site.ID)
	if err != nil {
		s.result(err)
		return
	}
	prompt := &confirmation{Title: s.text("delete"), Detail: fmt.Sprintf("%s\n%s\n%s", site.Name, affected, s.text("orphan_delete_notice")), ToggleLabel: s.text("cleanup_before_delete"), Danger: true}
	cache := s.iconCache()
	prompt.Action = func() error {
		if !prompt.Toggle {
			s.serviceChange(func(ctx context.Context) error {
				if err := s.Service.DeleteSite(ctx, site.ID); err != nil {
					return err
				}
				return cache.Invalidate(site.ID)
			})
			return nil
		}
		s.task(func() (func(), error) {
			result, err := s.Service.DeleteSiteWithCleanup(s.Context, site.ID)
			if err != nil {
				return nil, err
			}
			if err := cache.Invalidate(site.ID); err != nil {
				return nil, err
			}
			return func() {
				s.OperationResults = result.Results
				if err := s.reload(); err != nil {
					s.result(err)
				}
			}, nil
		})
		return nil
	}
	s.Confirm = prompt
}
