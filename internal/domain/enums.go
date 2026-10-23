package domain

import (
	"encoding/json"
	"strings"
)

func enumString(raw []byte, allowed ...string) (string, error) {
	var s string
	if e := json.Unmarshal(raw, &s); e != nil {
		return "", e
	}
	for _, a := range allowed {
		if s == a {
			return s, nil
		}
	}
	return "", NewError("validation_failed", "unknown enum value")
}
func (p *SiteProtocol) UnmarshalJSON(raw []byte) error {
	s, e := enumString(raw, "openai_compatible", "anthropic")
	if e == nil {
		*p = SiteProtocol(s)
	}
	return e
}
func (p *ClaudeAuthKeyStyle) UnmarshalJSON(raw []byte) error {
	s, e := enumString(raw, "anthropic_auth_token", "anthropic_api_key")
	if e == nil {
		*p = ClaudeAuthKeyStyle(s)
	}
	return e
}
func (p *ClaudeEffortLevel) UnmarshalJSON(raw []byte) error {
	var s string
	if e := json.Unmarshal(raw, &s); e != nil {
		return e
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "max" {
		s = "xhigh"
	}
	switch s {
	case "low", "medium", "high", "xhigh":
		*p = ClaudeEffortLevel(s)
		return nil
	}
	return NewError("validation_failed", "unknown Claude effort level")
}
func (p *CodexReasoningEffort) UnmarshalJSON(raw []byte) error {
	var s string
	if e := json.Unmarshal(raw, &s); e != nil {
		return e
	}
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "minimal", "low", "medium", "high", "xhigh":
		*p = CodexReasoningEffort(s)
		return nil
	}
	return NewError("validation_failed", "unknown Codex reasoning effort")
}
