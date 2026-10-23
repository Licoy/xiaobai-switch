package app

import (
	"context"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/service"
	"sort"
	"strings"
)

func (s *Shell) prepareApply(d *targetDraft, tray bool) {
	request := s.buildRequest(d)
	s.task(func() (func(), error) {
		ctx := s.Context
		preview, err := s.Service.PrepareApply(ctx, request)
		if err != nil {
			return nil, err
		}
		return func() {
			detail := targetLabel(preview.Target) + " · " + preview.ModelID + "\n\n" + strings.Join(preview.Files, "\n") + "\n\n" + s.text("secret_notice")
			keys := []string{}
			for key := range preview.Summary {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				detail += "\n" + key + ": " + preview.Summary[key]
			}
			s.Confirm = &confirmation{Title: s.text("apply_preview"), Detail: detail, Tray: tray,
				Dismiss: func() {
					if s.Closing {
						return
					}
					if err := s.Service.CancelPreview(s.Context, preview.ID); err != nil {
						s.result(err)
					}
				},
				Action: func() error { s.applyPreview(preview.ID); return nil },
			}
		}, nil
	})
}
func (s *Shell) applyPreview(id string) {
	s.task(func() (func(), error) {
		result, err := s.Service.Apply(s.Context, id)
		if err != nil {
			return nil, err
		}
		statuses, err := s.Service.Status(s.Context)
		if err != nil {
			return nil, err
		}
		return func() {
			s.acceptStatuses(statuses)
			s.OperationResults = result.Results
			if err := s.reload(); err != nil {
				s.result(err)
				return
			}
			s.Notice = s.text("apply_success")
		}, nil
	})
}
func (s *Shell) refreshStatus(target domain.TargetKind) {
	s.task(func() (func(), error) {
		statuses, err := s.Service.RefreshStatus(s.Context)
		if err != nil {
			return nil, err
		}
		return func() { s.acceptStatuses(statuses) }, nil
	})
}
func (s *Shell) acceptStatuses(statuses []service.TargetStatus) {
	initial := !s.StatusLoaded
	s.StatusLoaded = true
	for _, status := range statuses {
		s.Statuses[status.Target] = status
		d := s.Drafts[status.Target]
		if d == nil {
			continue
		}
		copy := status
		d.Status = &copy
		binding, err := s.Store.GetBinding(status.Target)
		if err != nil {
			s.result(err)
		} else {
			d.Binding = binding
		}
		if initial {
			s.hydrateAppliedDraft(d)
		}
	}
}
func (s *Shell) hydrateAppliedDraft(d *targetDraft) {
	binding, err := s.Store.GetBinding(d.Request.Target)
	if err != nil {
		s.result(err)
		return
	}
	if binding == nil || binding.SiteID == nil {
		return
	}
	for _, site := range s.Sites {
		if site.ID == *binding.SiteID && site.Enabled {
			s.hydrateDraft(d, site)
			return
		}
	}
}
func (s *Shell) restoreTarget(target domain.TargetKind, official bool) {
	key := "revert"
	if official {
		key = "restore_official"
	}
	s.confirm(s.text(key), targetLabel(target), func() error {
		s.task(func() (func(), error) {
			var err error
			if official {
				err = s.Service.RestoreOfficial(s.Context, target)
			} else {
				err = s.Service.Revert(s.Context, target)
			}
			if err != nil {
				return nil, err
			}
			status, err := s.Service.Status(s.Context)
			if err != nil {
				return nil, err
			}
			return func() {
				s.acceptStatuses(status)
				if official {
					s.Notice = s.text("official_restored")
				} else {
					s.Notice = s.text("reverted")
				}
			}, nil
		})
		return nil
	})
}
func (s *Shell) serviceChange(action func(context.Context) error) {
	s.task(func() (func(), error) {
		if err := action(s.Context); err != nil {
			return nil, err
		}
		statuses, err := s.Service.Status(s.Context)
		if err != nil {
			return nil, err
		}
		return func() {
			s.acceptStatuses(statuses)
			if err := s.reload(); err != nil {
				s.result(err)
			}
		}, nil
	})
}
func (s *Shell) operationSummary() string {
	parts := []string{}
	for _, r := range s.OperationResults {
		status := s.text("operation_failed")
		if r.Ok {
			status = s.text("apply_success")
		}
		parts = append(parts, fmt.Sprintf("%s · %s", targetLabel(r.Target), status+" · "+s.safeErrorDetail(r.Message)))
	}
	return strings.Join(parts, "\n")
}
