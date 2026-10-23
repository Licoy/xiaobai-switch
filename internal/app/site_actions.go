package app

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"strings"
)

func (s *Shell) writeSite(e siteEditor) (string, error) {
	urls := lines(e.URLs)
	if len(urls) == 0 {
		return "", domain.NewError("validation_failed", s.text("url_required"))
	}
	if e.ID == "" {
		extra := []domain.AddSiteApiKeyInput{}
		for _, key := range e.ExtraKeys {
			if key = strings.TrimSpace(key); key != "" {
				extra = append(extra, domain.AddSiteApiKeyInput{APIKey: key})
			}
		}
		site, err := s.Service.CreateSite(s.Context, domain.CreateSiteInput{Name: e.Name, BaseURL: urls[0], BaseURLs: &urls, APIKey: e.Key, ExtraAPIKeys: extra, Protocol: &e.Protocol, ClaudeAuthKeyStyle: &e.Auth, Notes: &e.Notes, Capabilities: &e.Capabilities})
		return site.ID, err
	}
	site, err := s.Service.UpdateSite(s.Context, e.ID, domain.UpdateSiteInput{Name: &e.Name, BaseURLs: &urls, Protocol: &e.Protocol, ClaudeAuthKeyStyle: &e.Auth, Notes: &e.Notes, Capabilities: &e.Capabilities})
	return site.ID, err
}
func (s *Shell) saveSite() error {
	id, err := s.writeSite(*s.Editor)
	if err != nil {
		return err
	}
	s.Selected = id
	if s.Editor.ID != "" {
		s.forgetSiteIcon(id)
	}
	s.clearEditor()
	return s.reload()
}
func (s *Shell) saveSiteAsync() {
	editor := s.Editor
	input := *editor
	input.ExtraKeys = append([]string(nil), editor.ExtraKeys...)
	input.Capabilities = domain.SiteCapabilities{}
	for k, v := range editor.Capabilities {
		input.Capabilities[k] = v
	}
	s.task(func() (func(), error) {
		id, err := s.writeSite(input)
		input.Key = ""
		for i := range input.ExtraKeys {
			input.ExtraKeys[i] = ""
		}
		input.ExtraKeys = nil
		if err != nil {
			return nil, err
		}
		return func() {
			s.Selected = id
			if input.ID != "" {
				s.forgetSiteIcon(id)
			}
			if s.Editor == editor {
				s.clearEditor()
			}
			if err := s.reload(); err != nil {
				s.result(err)
				return
			}
		}, nil
	})
}
func (s *Shell) writeKey(e keyEditor) error {
	if e.ID == "" {
		_, err := s.Service.AddSiteAPIKey(s.Context, e.SiteID, domain.AddSiteApiKeyInput{Label: &e.Label, APIKey: e.Key})
		return err
	}
	_, err := s.Service.UpdateSiteAPIKey(s.Context, e.SiteID, e.ID, domain.UpdateSiteApiKeyInput{Label: &e.Label, APIKey: optional(e.Key)})
	return err
}
func (s *Shell) saveKeyAsync() {
	editor := s.KeyEditor
	input := *editor
	s.task(func() (func(), error) {
		err := s.writeKey(input)
		input.Key = ""
		if err != nil {
			return nil, err
		}
		return func() {
			if s.KeyEditor == editor {
				s.clearKeyEditor()
			}
			if err := s.reload(); err != nil {
				s.result(err)
				return
			}
		}, nil
	})
}
func (s *Shell) activateKey(siteID, keyID string) {
	s.task(func() (func(), error) {
		result, err := s.Service.SwitchKey(s.Context, siteID, keyID)
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
}
