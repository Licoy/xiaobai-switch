package app

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"github.com/licoy/xiaobai-switch/internal/skills"
	"time"
)

type skillsState struct {
	Local                                             []domain.SkillInfo
	Market                                            []domain.MarketplaceSkill
	Detail                                            *domain.SkillDetail
	Query, Source, InstallSource, InstallName, Filter string
	Target                                            domain.SkillTarget
	DetailOpen                                        bool
}

func newSkillsState() skillsState {
	return skillsState{Source: "skills.sh", Target: domain.SkillAgents}
}
func (s *Shell) loadSkills() {
	settings := s.Settings
	s.task(func() (func(), error) {
		client, err := netx.NewClient(settings, 30*time.Second)
		if err != nil {
			return nil, err
		}
		defer client.CloseIdleConnections()
		manager, err := skills.New(settings, client)
		if err != nil {
			return nil, err
		}
		rows, err := manager.List()
		if err != nil {
			return nil, err
		}
		return func() { s.Skills.Local = rows }, nil
	})
}
func (s *Shell) searchSkills() {
	settings := s.Settings
	query, source := s.Skills.Query, s.Skills.Source
	s.task(func() (func(), error) {
		client, err := netx.NewClient(settings, 30*time.Second)
		if err != nil {
			return nil, err
		}
		defer client.CloseIdleConnections()
		manager, err := skills.New(settings, client)
		if err != nil {
			return nil, err
		}
		rows, err := manager.Search(s.Context, query, source)
		if err != nil {
			return nil, err
		}
		return func() { s.Skills.Market = rows }, nil
	})
}
func (s *Shell) installSkill(source, name string, target domain.SkillTarget) {
	settings := s.Settings
	s.confirm(s.text("install_skill"), source+"\n"+name+"\n"+skillTargetLabel(target), func() error {
		s.task(func() (func(), error) {
			client, err := netx.NewClient(settings, 30*time.Second)
			if err != nil {
				return nil, err
			}
			defer client.CloseIdleConnections()
			manager, err := skills.New(settings, client)
			if err != nil {
				return nil, err
			}
			result, err := manager.Install(s.Context, source, target, name)
			if err != nil {
				return nil, err
			}
			rows, err := manager.List()
			if err != nil {
				return nil, err
			}
			return func() {
				s.Skills.Local = rows
				s.Notice = countMessage(s.text("installed"), len(result.Names))
				if len(result.Warnings) > 0 {
					s.Warning = s.text("install_warning")
					s.WarningDetail = s.safeErrorDetail(joinLines(result.Warnings))
				}
			}, nil
		})
		return nil
	})
}
func (s *Shell) localSkillAction(info domain.SkillInfo, action string) {
	settings := s.Settings
	s.task(func() (func(), error) {
		client, err := netx.NewClient(settings, 30*time.Second)
		if err != nil {
			return nil, err
		}
		defer client.CloseIdleConnections()
		manager, err := skills.New(settings, client)
		if err != nil {
			return nil, err
		}
		if action == "detail" {
			detail, err := manager.Detail(info.Target, info.SourcePath)
			if err != nil {
				return nil, err
			}
			return func() { s.Skills.Detail = &detail; s.Skills.DetailOpen = true }, nil
		}
		if action == "update" {
			result, err := manager.Update(s.Context, info.Target, info.SourcePath)
			if err != nil {
				return nil, err
			}
			rows, err := manager.List()
			if err != nil {
				return nil, err
			}
			return func() {
				s.Skills.Local = rows
				s.Notice = s.text("skill_updated")
				if len(result.Warnings) > 0 {
					s.Warning = s.text("install_warning")
					s.WarningDetail = s.safeErrorDetail(joinLines(result.Warnings))
				}
			}, nil
		}
		if action == "delete" {
			err = manager.Uninstall(info.Target, info.SourcePath)
		} else {
			err = manager.SetEnabled(info.Target, info.SourcePath, !info.Enabled)
		}
		if err != nil {
			return nil, err
		}
		rows, err := manager.List()
		if err != nil {
			return nil, err
		}
		return func() { s.Skills.Local = rows }, nil
	})
}
