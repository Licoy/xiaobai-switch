package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

var skillTargets = []domain.SkillTarget{domain.SkillAgents, domain.SkillClaudeCode, domain.SkillCodex, domain.SkillPi, domain.SkillPrime}

func skillTargetLabel(target domain.SkillTarget) string {
	switch target {
	case domain.SkillAgents:
		return "Agents"
	case domain.SkillClaudeCode:
		return "Claude Code"
	case domain.SkillCodex:
		return "Codex"
	case domain.SkillPi:
		return "Pi"
	case domain.SkillPrime:
		return "Prime"
	}
	return string(target)
}
func skillTargetTile(c *ui.Context, target domain.SkillTarget, size float32) ui.Element {
	if kind := domain.TargetKind(target); target != domain.SkillAgents {
		return targetTile(c, kind, size)
	}
	return Tile(c, icTerminal, TintGray, size, true)
}
func joinLines(lines []string) string { return strings.Join(lines, "\n") }
func (s *Shell) skillsView(c *ui.Context) {
	t := c.Theme()
	v := &s.Skills
	enabled := 0
	for _, info := range v.Local {
		if info.Enabled {
			enabled++
		}
	}
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		Stat(c, icPuzzle, TintYellow, s.text("installed_skills"), fmt.Sprintf("%d", len(v.Local)))
		Stat(c, icToggle, TintGreen, s.text("enabled"), fmt.Sprintf("%d", enabled))
		Stat(c, icStore, TintPurple, s.text("skill_market"), fmt.Sprintf("%d", len(v.Market)))
	})
	Panel(c).Children(func() {
		PanelTitle(c, icPuzzle, TintYellow, s.text("installed_skills"), fmt.Sprintf("%d", len(v.Local)), false, func() {
			ui.TextInput(c.Key("skill-filter"), &v.Filter).Width(220).Placeholder(s.text("search_skills")).Label(s.text("search_skills"))
		})
		shown := 0
		filter := strings.ToLower(strings.TrimSpace(v.Filter))
		n := columns(s.contentWidth(c)-36, 300, 2)
		rows := []domain.SkillInfo{}
		for _, info := range v.Local {
			if strings.Contains(strings.ToLower(info.Name+" "+info.Description), filter) {
				rows = append(rows, info)
			}
		}
		shown = len(rows)
		if shown == 0 {
			EmptyState(c, icPuzzle, TintYellow, s.text("no_skills"), s.text("no_skills_detail"))
			return
		}
		grid(c, n, len(rows), 10, func(i int) { s.localSkillCard(c, rows[i]) })
	})
	Panel(c).Children(func() {
		PanelTitle(c, icStore, TintPurple, s.text("skill_market"), "", false, func() {
			stringTabs(c.Key("skill-source"), &v.Source, []string{"skills.sh", "github"}, "skills.sh", "GitHub")
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			input := ui.TextInput(c.Key("skill-query"), &v.Query).Grow(1).Basis(0).Placeholder(s.text("search_skills")).Label(s.text("search_skills"))
			if input.Submitted() && !s.Busy {
				s.searchSkills()
			}
			if AccentPillButton(c, icSearch, s.text("search_action"), s.Busy).Clicked() {
				s.searchSkills()
			}
		})
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, s.text("install_target")).FontSize(12).TextColor(t.TextMuted)
			values, labels := []string{}, []string{}
			for _, target := range skillTargets {
				values, labels = append(values, string(target)), append(labels, skillTargetLabel(target))
			}
			selected := string(v.Target)
			if stringTabs(c.Key("skill-target"), &selected, values, labels...) {
				v.Target = domain.SkillTarget(selected)
			}
		})
		if len(v.Market) == 0 {
			EmptyState(c, icStore, TintPurple, s.text("market_empty"), s.text("market_empty_detail"))
			return
		}
		s.backupList(c, len(v.Market), func(i int) { s.marketRow(c, v.Market[i]) })
	})
	Panel(c).Children(func() {
		PanelTitle(c, icLinks, TintTeal, s.text("install_from_source"), "", false, nil)
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.End).Children(func() {
			ui.Column(c).Grow(2).Basis(0).Children(func() {
				field(c, s.text("source"), func() {
					ui.TextInput(c.Key("install-source"), &v.InstallSource).FillWidth().Placeholder("owner/repo").Label(s.text("source"))
				})
			})
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				field(c, s.text("skill_name_optional"), func() {
					ui.TextInput(c.Key("install-name"), &v.InstallName).FillWidth().Label(s.text("skill_name_optional"))
				})
			})
			if AccentPillButton(c, icInstall, s.text("install_skill"), s.Busy || strings.TrimSpace(v.InstallSource) == "").Clicked() {
				s.installSkill(v.InstallSource, v.InstallName, v.Target)
			}
		})
		ui.Text(c, s.text("install_target")+": "+skillTargetLabel(v.Target)).FontSize(11).TextColor(t.TextMuted)
	})
	s.skillDetailView(c)
}
func (s *Shell) localSkillCard(c *ui.Context, info domain.SkillInfo) {
	t := c.Theme()
	ui.Column(c.Key(string(info.Target)+"/"+info.SourcePath)).FillWidth().Gap(10).Padding(12, 14).Radius(12).Background(pal(t).Inset).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			Tile(c, icPuzzle, tintFor(info.Name), 30, false)
			ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
				ui.Text(c, info.Name).FontSize(13).FontWeight(600).SingleLine()
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					skillTargetTile(c, info.Target, 14)
					ui.Text(c, skillTargetLabel(info.Target)).FontSize(11).TextColor(t.TextMuted).SingleLine()
					if info.Version != nil {
						ui.Text(c, "v"+*info.Version).FontSize(11).TextColor(t.TextMuted).SingleLine()
					}
				})
			})
			enabled := info.Enabled
			if ui.Switch(c.Key("enabled"), &enabled).Label(s.text("enabled")).Disabled(s.Busy).Changed() {
				s.localSkillAction(info, "toggle")
			}
		})
		desc := info.Description
		if desc == "" {
			desc = "—"
		}
		ui.Text(c, desc).FontSize(12).TextColor(t.TextMuted).MaxLines(2).Wrap()
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			if PillButton(c, icEye, s.text("detail"), s.Busy).Clicked() {
				s.localSkillAction(info, "detail")
			}
			if PillButton(c, icRefresh, s.text("update_skill"), s.Busy).Clicked() {
				s.confirm(s.text("update_skill"), info.Name, func() error { s.localSkillAction(info, "update"); return nil })
			}
			ui.Spacer(c)
			if PillButton(c, icFolder, "", false).Label(s.text("open_directory")).Tooltip(s.text("open_directory")).Clicked() {
				s.openDirectory(info.DirectoryPath)
			}
			if DangerPillButton(c, icDelete, "", s.Busy).Label(s.text("delete")).Tooltip(s.text("delete")).Clicked() {
				s.confirmDanger(s.text("delete"), info.Name+"\n"+info.DirectoryPath, func() error { s.localSkillAction(info, "delete"); return nil })
			}
		})
	})
}
func (s *Shell) marketRow(c *ui.Context, item domain.MarketplaceSkill) {
	t := c.Theme()
	ui.Row(c.Key(item.Repo+"/"+item.Name)).FillWidth().Gap(10).Padding(8, 0).AlignItems(ui.Center).Children(func() {
		letterTile(c, item.Name, item.Repo+"/"+item.Name, 30)
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, item.Name).FontSize(13).FontWeight(600).SingleLine()
				for _, target := range item.InstalledTargets {
					Pill(c, skillTargetLabel(target), t.Success)
				}
			})
			if item.Description != "" {
				ui.Text(c, item.Description).FontSize(12).TextColor(t.TextMuted).MaxLines(2).Wrap()
			}
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icGithub).Size(12, 12).TextColor(t.TextMuted)
				mono(c, item.Repo).TextColor(t.TextMuted).SingleLine()
				if item.Stars > 0 {
					ui.Icon(c, icStar).Size(12, 12).TextColor(TintYellow.C(t))
					ui.Text(c, fmt.Sprintf("%d", item.Stars)).FontSize(11).TextColor(t.TextMuted)
				}
				if item.Installs > 0 {
					ui.Icon(c, icDownload).Size(12, 12).TextColor(TintBlue.C(t))
					ui.Text(c, fmt.Sprintf("%d", item.Installs)).FontSize(11).TextColor(t.TextMuted)
				}
			})
		})
		if PillButton(c, icInstall, s.text("install_skill"), s.Busy).Clicked() {
			s.installSkill(item.Repo, item.Name, s.Skills.Target)
		}
	})
}
func (s *Shell) skillDetailView(c *ui.Context) {
	v := &s.Skills
	if v.Detail == nil {
		return
	}
	t := c.Theme()
	dialog := ui.Modal(c.Key("skill_detail"), &v.DetailOpen, func() {
		ui.Column(c).Width(600).Gap(14).Children(func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				Tile(c, icPuzzle, tintFor(v.Detail.Info.Name), 36, true)
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, v.Detail.Info.Name).FontSize(17).FontWeight(700).SingleLine()
					ui.Text(c, skillTargetLabel(v.Detail.Info.Target)).FontSize(12).TextColor(t.TextMuted)
				})
			})
			ui.Scroll(c).Height(340).FillWidth().Radius(12).Background(pal(t).Inset).Children(func() {
				mono(c, v.Detail.Content).Wrap().Selectable().Padding(12)
			})
			if len(v.Detail.Files) > 0 {
				ui.Text(c, strings.Join(v.Detail.Files, ", ")).Wrap().FontSize(11).TextColor(t.TextMuted)
			}
			dialogButtons(c, func() {
				if AccentPillButton(c, icCheck, s.text("close"), false).Clicked() {
					v.DetailOpen = false
				}
			})
		})
	})
	if dialog.Dismissed() || !v.DetailOpen {
		v.Detail = nil
	}
}
