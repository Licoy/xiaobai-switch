package domain

import "strings"

const (
	BaselineThinkingLevel   = "baseline_default_thinking_level"
	BaselineModelThinking   = "baseline_model_thinking_level"
	WroteThinkingLevel      = "wrote_default_thinking_level"
	ClearedModelThinkingKey = "cleared_model_thinking_key"
	BaselineMissing         = "__xiaobai_missing__"
)

func ParseThinkingLevel(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "off", "minimal", "low", "medium", "high", "xhigh", "max":
		return s, true
	}
	return "", false
}
func IsThinkingTarget(t TargetKind) bool { return t == Pi || t == Prime }
func (p SiteThinkingPreset) ToWrite() ThinkingWrite {
	return ThinkingWrite{DefaultLevel: p.DefaultLevel, Extended: p.Extended, Models: p.Models}
}
func (w ThinkingWrite) MappingFor(id, level string) string {
	if v := strings.TrimSpace(w.Models[id].ThinkingLevelMap[level]); v != "" {
		return v
	}
	if level == "xhigh" && w.Extended.Xhigh != nil {
		return strings.TrimSpace(*w.Extended.Xhigh)
	}
	if level == "max" && w.Extended.Max != nil {
		return strings.TrimSpace(*w.Extended.Max)
	}
	return ""
}
func AllowsExtended(t TargetKind, p SiteProtocol, c ModelThinkingConfig) bool {
	return p == OpenAICompatible || (t == Pi && p == Anthropic && c.ForceAdaptiveThinking)
}
func ValidateThinking(w ThinkingWrite, t TargetKind, p SiteProtocol, modelID string) error {
	if !IsThinkingTarget(t) {
		return NewError("validation_failed", "thinking presets are only supported for Pi and Prime")
	}
	for _, cfg := range w.Models {
		if cfg.ForceAdaptiveThinking && (t != Pi || p != Anthropic) {
			return NewError("validation_failed", "adaptive thinking is only available for Pi with Anthropic")
		}
		for level := range cfg.ThinkingLevelMap {
			if _, ok := ParseThinkingLevel(level); !ok {
				return NewError("validation_failed", "unknown thinking level in model map")
			}
		}
	}
	if w.DefaultLevel == nil {
		return nil
	}
	level, ok := ParseThinkingLevel(*w.DefaultLevel)
	if !ok {
		return NewError("validation_failed", "unknown default thinking level")
	}
	if modelID != "" && !w.Models[modelID].Reasoning {
		return NewError("validation_failed", "enable thinking on the default model")
	}
	if level != "max" && level != "xhigh" {
		return nil
	}
	if t == Prime && p == Anthropic {
		return NewError("validation_failed", "Prime Anthropic does not support extended thinking")
	}
	if modelID != "" {
		if !AllowsExtended(t, p, w.Models[modelID]) {
			return NewError("validation_failed", "enable adaptive thinking before using extended levels")
		}
		if w.MappingFor(modelID, level) == "" {
			return NewError("validation_failed", "set provider mapping for extended thinking")
		}
		return nil
	}
	mapping := w.MappingFor("", level) != ""
	adaptive := false
	for id, c := range w.Models {
		mapping = mapping || w.MappingFor(id, level) != ""
		adaptive = adaptive || c.ForceAdaptiveThinking
	}
	if !mapping {
		return NewError("validation_failed", "set provider mapping for extended thinking")
	}
	if t == Pi && p == Anthropic && !adaptive {
		return NewError("validation_failed", "enable adaptive thinking before using extended levels")
	}
	return nil
}
