package adapters

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func ValidateOptions(r Request) error {
	switch r.Target {
	case domain.ClaudeCode:
		if r.Claude.AuthKeyStyle != nil && *r.Claude.AuthKeyStyle != domain.AnthropicAPIKey && *r.Claude.AuthKeyStyle != domain.AnthropicAuthToken {
			return domain.NewError("validation_failed", "unknown Claude auth style")
		}
		if r.Claude.EffortLevel != nil {
			switch *r.Claude.EffortLevel {
			case "low", "medium", "high", "xhigh", "max":
			default:
				return domain.NewError("validation_failed", "unknown Claude effort")
			}
		}
	case domain.Codex:
		if r.Codex.ReasoningEffort != nil {
			switch *r.Codex.ReasoningEffort {
			case "minimal", "low", "medium", "high", "xhigh":
			default:
				return domain.NewError("validation_failed", "unknown Codex reasoning effort")
			}
		}
	case domain.Pi:
		return domain.ValidateThinking(r.Pi.Thinking, r.Target, r.Site.Protocol, r.ModelID)
	case domain.Prime:
		return domain.ValidateThinking(r.Prime.Thinking, r.Target, r.Site.Protocol, r.ModelID)
	default:
		return domain.NewError("validation_failed", "unknown target")
	}
	return nil
}
