package domain

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type SiteProtocol string

const (
	OpenAICompatible SiteProtocol = "openai_compatible"
	Anthropic        SiteProtocol = "anthropic"
)

type TargetKind string

const (
	ClaudeCode TargetKind = "claude_code"
	Codex      TargetKind = "codex"
	Pi         TargetKind = "pi"
	Prime      TargetKind = "prime"
)

func (t TargetKind) Valid() bool { return t == ClaudeCode || t == Codex || t == Pi || t == Prime }
func (t *TargetKind) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "prime_agent" || s == "prime-agent" {
		s = "prime"
	}
	v := TargetKind(s)
	if !v.Valid() {
		return NewError("validation_failed", "unknown target")
	}
	*t = v
	return nil
}

type ClaudeAuthKeyStyle string

const (
	AnthropicAuthToken ClaudeAuthKeyStyle = "anthropic_auth_token"
	AnthropicAPIKey    ClaudeAuthKeyStyle = "anthropic_api_key"
)

func (s ClaudeAuthKeyStyle) EnvKey() string {
	if s == AnthropicAPIKey {
		return "ANTHROPIC_API_KEY"
	}
	return "ANTHROPIC_AUTH_TOKEN"
}
func (s ClaudeAuthKeyStyle) OtherEnvKey() string {
	if s == AnthropicAPIKey {
		return "ANTHROPIC_AUTH_TOKEN"
	}
	return "ANTHROPIC_API_KEY"
}

type ApplyStatus string

const (
	Applied    ApplyStatus = "applied"
	Stale      ApplyStatus = "stale"
	Orphan     ApplyStatus = "orphan"
	NotApplied ApplyStatus = "not_applied"
	Failed     ApplyStatus = "failed"
)

type ClaudeEffortLevel string
type CodexReasoningEffort string
type CapabilitySource string

const (
	CapabilitySite   CapabilitySource = "site"
	CapabilityCustom CapabilitySource = "custom"
)

type SiteCapabilities map[string]bool
type QuotaProbeStatus string
type QuotaSource string
type AppBackupKind string
type CatalogModel struct {
	ModelID     string `json:"modelId"`
	DisplayName string `json:"displayName"`
}
type AppError struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Details *string `json:"details,omitempty"`
}

func (e *AppError) Error() string             { return e.Code + ": " + e.Message }
func NewError(code, message string) *AppError { return &AppError{Code: code, Message: message} }
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[:8], s[8:12], s[12:16], s[16:20], s[20:])
}
func compactID(id string) string {
	s := strings.ReplaceAll(id, "-", "")
	if len(s) > 12 {
		s = s[:12]
	}
	return s
}
func ProviderIDForSite(id string) string { return "xiaobai_" + strings.ToLower(compactID(id)) }
func EnvKeyForSite(id string) string {
	return "XIAOBAI_SITE_" + strings.ToUpper(compactID(id)) + "_API_KEY"
}

type ApplyRequest struct {
	SiteID   string             `json:"siteId"`
	APIKeyID string             `json:"apiKeyId"`
	Target   TargetKind         `json:"target"`
	ModelID  string             `json:"modelId"`
	Claude   ClaudeApplyOptions `json:"claude"`
	Codex    CodexApplyOptions  `json:"codex"`
	Pi       PiApplyOptions     `json:"pi"`
	Prime    PrimeApplyOptions  `json:"prime"`
}
type TargetPreset struct {
	SiteID    string       `json:"siteId"`
	APIKeyID  string       `json:"apiKeyId"`
	Target    TargetKind   `json:"target"`
	Options   ApplyRequest `json:"options"`
	UpdatedAt int64        `json:"updatedAt"`
}
