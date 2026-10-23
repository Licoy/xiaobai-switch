package domain

import "encoding/json"

var _ json.RawMessage

type BindingApiKeySnapshot struct {
	ID     *string `json:"id"`
	Label  *string `json:"label"`
	Prefix *string `json:"prefix"`
}

type TargetBinding struct {
	Target           TargetKind            `json:"target"`
	SiteID           *string               `json:"siteId"`
	SiteNameSnapshot string                `json:"siteNameSnapshot"`
	ModelID          string                `json:"modelId"`
	ProviderID       *string               `json:"providerId"`
	KeyFingerprint   string                `json:"keyFingerprint"`
	ManagedPaths     []string              `json:"managedPaths"`
	ManagedEnvKeys   []string              `json:"managedEnvKeys"`
	ExpectedFields   map[string]string     `json:"expectedFields"`
	Orphan           bool                  `json:"orphan"`
	AppliedAt        int64                 `json:"appliedAt"`
	ApplyRecordID    *string               `json:"applyRecordId"`
	APIKey           BindingApiKeySnapshot `json:"apiKey"`
}

type SiteKeyState struct {
	ActiveAPIKeyID *string             `json:"activeApiKeyId"`
	APIKeys        []SiteApiKeySummary `json:"apiKeys"`
}

type SiteRow struct {
	ID                      string             `json:"id"`
	Name                    string             `json:"name"`
	BaseURL                 string             `json:"baseUrl"`
	BaseURLs                []string           `json:"baseUrls"`
	APIKeyEncrypted         string             `json:"-"`
	KeyPrefix               string             `json:"keyPrefix"`
	Protocol                SiteProtocol       `json:"protocol"`
	ClaudeAuthKeyStyle      ClaudeAuthKeyStyle `json:"claudeAuthKeyStyle"`
	Notes                   *string            `json:"notes"`
	Enabled                 bool               `json:"enabled"`
	SortOrder               int64              `json:"sortOrder"`
	SelectedModelID         *string            `json:"selectedModelId"`
	LastModelFetchAt        *int64             `json:"lastModelFetchAt"`
	LastModelFetchLatencyMS *int64             `json:"lastModelFetchLatencyMs"`
	LastModelFetchError     *string            `json:"lastModelFetchError"`
	CreatedAt               int64              `json:"createdAt"`
	UpdatedAt               int64              `json:"updatedAt"`
	Capabilities            SiteCapabilities   `json:"capabilities"`
	Keys                    SiteKeyState       `json:"keys"`
}

type SiteApiKeyRow struct {
	ID                      string  `json:"id"`
	SiteID                  string  `json:"siteId"`
	Label                   string  `json:"label"`
	APIKeyEncrypted         string  `json:"-"`
	KeyPrefix               string  `json:"keyPrefix"`
	IsActive                bool    `json:"isActive"`
	SelectedModelID         *string `json:"selectedModelId"`
	LastModelFetchAt        *int64  `json:"lastModelFetchAt"`
	LastModelFetchLatencyMS *int64  `json:"lastModelFetchLatencyMs"`
	LastModelFetchError     *string `json:"lastModelFetchError"`
	CreatedAt               int64   `json:"createdAt"`
	UpdatedAt               int64   `json:"updatedAt"`
}

type ThinkingExtendedMap struct {
	Xhigh *string `json:"xhigh"`
	Max   *string `json:"max"`
}

type ModelThinkingConfig struct {
	Reasoning             bool              `json:"reasoning"`
	ForceAdaptiveThinking bool              `json:"forceAdaptiveThinking"`
	ThinkingLevelMap      map[string]string `json:"thinkingLevelMap"`
}

type ThinkingWrite struct {
	DefaultLevel *string                        `json:"defaultLevel"`
	Extended     ThinkingExtendedMap            `json:"extended"`
	Models       map[string]ModelThinkingConfig `json:"models"`
}

type SiteThinkingPreset struct {
	SiteID       string                         `json:"siteId"`
	Target       TargetKind                     `json:"target"`
	DefaultLevel *string                        `json:"defaultLevel"`
	Extended     ThinkingExtendedMap            `json:"extended"`
	Models       map[string]ModelThinkingConfig `json:"models"`
}
