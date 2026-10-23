package service

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"path/filepath"
)

var targets = []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime}

// RefreshStatus bypasses the CLI cache for an explicit user refresh
func (s *Service) RefreshStatus(ctx context.Context) ([]TargetStatus, error) {
	if e := checkContext(ctx); e != nil {
		return nil, e
	}
	s.detector.Refresh()
	return s.Status(ctx)
}

func (s *Service) Status(ctx context.Context) ([]TargetStatus, error) {
	if e := checkContext(ctx); e != nil {
		return nil, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, e := s.st.GetSettings()
	if e != nil {
		return nil, e
	}
	tools := s.detector.Snapshot()
	out := []TargetStatus{}
	for _, target := range targets {
		home, e := s.targetHome(target, settings)
		if e != nil {
			return nil, e
		}
		if e = s.validateTargetFiles(target, home); e != nil {
			return nil, e
		}
		env, e := s.path("env", "codex.env")
		if e != nil {
			return nil, e
		}
		b, e := s.st.GetBinding(target)
		if e != nil {
			return nil, e
		}
		var site *domain.SiteRow
		key := ""
		if b != nil && b.SiteID != nil {
			v, err := s.st.GetSite(*b.SiteID)
			if err != nil {
				var app *domain.AppError
				if !errors.As(err, &app) || app.Code != "not_found" {
					return nil, err
				}
			} else {
				site = &domain.SiteRow{ID: v.ID, BaseURL: v.BaseURL, Protocol: domain.SiteProtocol(v.Protocol)}
				keyID := ""
				if b.APIKey.ID != nil {
					keyID = *b.APIKey.ID
				}
				key, err = s.st.GetSiteAPIKey(v.ID, keyID)
				if err != nil {
					var app *domain.AppError
					if !errors.As(err, &app) || app.Code != "not_found" {
						return nil, err
					}
				}
			}
		}
		st, e := adapters.Inspect(adapters.StatusRequest{Target: target, Home: home, EnvPath: env, Binding: b, Site: site, APIKey: key})
		if e != nil {
			return nil, publicError(e, key)
		}
		name := "settings.json"
		if target == domain.Codex {
			name = "config.toml"
		}
		if target == domain.Pi || target == domain.Prime {
			name = "models.json"
		}
		row := TargetStatus{Target: target, Status: st.Status, Fields: st.Fields, Differences: st.Differences, ConfigPath: filepath.Join(home, name), CLIRefreshing: tools.Refreshing}
		if message, ok := tools.Errors[target]; ok {
			row.CLIError = &message
		}
		for _, tool := range tools.Tools {
			if tool.Kind == target {
				row.Installed = tool.Installed
				row.Version = tool.Version
			}
		}
		out = append(out, row)
	}
	return out, nil
}
