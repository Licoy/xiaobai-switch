package domain

import "encoding/json"

var _ json.RawMessage

type SiteApiKeySummary struct {
	ID                      string  `json:"id"`
	Label                   string  `json:"label"`
	KeyPrefix               string  `json:"keyPrefix"`
	IsActive                bool    `json:"isActive"`
	QuotaRevision           string  `json:"quotaRevision"`
	SelectedModelID         *string `json:"selectedModelId"`
	LastModelFetchAt        *int64  `json:"lastModelFetchAt"`
	LastModelFetchLatencyMS *int64  `json:"lastModelFetchLatencyMs"`
	LastModelFetchError     *string `json:"lastModelFetchError"`
}

type SiteDTO struct {
	ID                      string              `json:"id"`
	Name                    string              `json:"name"`
	BaseURL                 string              `json:"baseUrl"`
	BaseURLs                []string            `json:"baseUrls"`
	KeyPrefix               string              `json:"keyPrefix"`
	QuotaRevision           string              `json:"quotaRevision"`
	HasKey                  bool                `json:"hasKey"`
	Protocol                string              `json:"protocol"`
	ClaudeAuthKeyStyle      string              `json:"claudeAuthKeyStyle"`
	Notes                   *string             `json:"notes"`
	Enabled                 bool                `json:"enabled"`
	SortOrder               int64               `json:"sortOrder"`
	SelectedModelID         *string             `json:"selectedModelId"`
	LastModelFetchAt        *int64              `json:"lastModelFetchAt"`
	LastModelFetchLatencyMS *int64              `json:"lastModelFetchLatencyMs"`
	LastModelFetchError     *string             `json:"lastModelFetchError"`
	CreatedAt               int64               `json:"createdAt"`
	UpdatedAt               int64               `json:"updatedAt"`
	Capabilities            SiteCapabilities    `json:"capabilities"`
	ActiveAPIKeyID          *string             `json:"activeApiKeyId"`
	APIKeys                 []SiteApiKeySummary `json:"apiKeys"`
}

type SiteModelDTO struct {
	ID          string           `json:"id"`
	SiteID      string           `json:"siteId"`
	APIKeyID    string           `json:"apiKeyId"`
	ModelID     string           `json:"modelId"`
	DisplayName string           `json:"displayName"`
	OwnedBy     *string          `json:"ownedBy"`
	Raw         *json.RawMessage `json:"raw"`
	IsManual    bool             `json:"isManual"`
}

type AddSiteApiKeyInput struct {
	Label  *string `json:"label"`
	APIKey string  `json:"apiKey"`
}

type CreateSiteInput struct {
	Name               string               `json:"name"`
	BaseURL            string               `json:"baseUrl"`
	BaseURLs           *[]string            `json:"baseUrls"`
	APIKey             string               `json:"apiKey"`
	APIKeyLabel        *string              `json:"apiKeyLabel"`
	ExtraAPIKeys       []AddSiteApiKeyInput `json:"extraApiKeys"`
	Protocol           *string              `json:"protocol"`
	ClaudeAuthKeyStyle *string              `json:"claudeAuthKeyStyle"`
	Notes              *string              `json:"notes"`
	Capabilities       *SiteCapabilities    `json:"capabilities"`
}

type DeepLinkSiteImportInput struct {
	Name         string            `json:"name"`
	BaseURLs     []string          `json:"baseUrls"`
	APIKey       string            `json:"apiKey"`
	Protocol     *string           `json:"protocol"`
	Notes        *string           `json:"notes"`
	Capabilities *SiteCapabilities `json:"capabilities"`
	KeyName      *string           `json:"keyName"`
}

type DeepLinkSiteImportResult struct {
	Site            SiteDTO `json:"site"`
	Created         bool    `json:"created"`
	AddedAPIKey     bool    `json:"addedApiKey"`
	ReusedAPIKey    bool    `json:"reusedApiKey"`
	ActivatedAPIKey bool    `json:"activatedApiKey"`
}

type UpdateSiteApiKeyInput struct {
	Label  *string `json:"label"`
	APIKey *string `json:"apiKey"`
}

type UpsertSiteApiKeyInput struct {
	ID     *string `json:"id"`
	Label  *string `json:"label"`
	APIKey string  `json:"apiKey"`
}

type UpdateSiteInput struct {
	Name               *string                  `json:"name"`
	BaseURL            *string                  `json:"baseUrl"`
	BaseURLs           *[]string                `json:"baseUrls"`
	APIKey             *string                  `json:"apiKey"`
	APIKeys            *[]UpsertSiteApiKeyInput `json:"apiKeys"`
	Protocol           *string                  `json:"protocol"`
	ClaudeAuthKeyStyle *string                  `json:"claudeAuthKeyStyle"`
	Notes              *string                  `json:"notes"`
	Enabled            *bool                    `json:"enabled"`
	SelectedModelID    *string                  `json:"selectedModelId"`
	SortOrder          *int64                   `json:"sortOrder"`
	Capabilities       *SiteCapabilities        `json:"capabilities"`
}

type FetchModelsResult struct {
	Models    []SiteModelDTO `json:"models"`
	LatencyMS uint64         `json:"latencyMs"`
	Endpoint  string         `json:"endpoint"`
	FetchedAt int64          `json:"fetchedAt"`
	APIKeyID  string         `json:"apiKeyId"`
}

type ProbeSiteApiKeyResult struct {
	ModelCount int    `json:"modelCount"`
	LatencyMS  uint64 `json:"latencyMs"`
	Endpoint   string `json:"endpoint"`
}

type ModelFetchOutcome struct {
	Ok        bool    `json:"ok"`
	APIKeyID  string  `json:"apiKeyId"`
	LatencyMS uint64  `json:"latencyMs"`
	Endpoint  *string `json:"endpoint"`
	FetchedAt *int64  `json:"fetchedAt"`
	Error     *string `json:"error"`
}

type ModelProbeResult struct {
	ModelID   string  `json:"modelId"`
	Ok        bool    `json:"ok"`
	LatencyMS uint64  `json:"latencyMs"`
	Status    *uint16 `json:"status"`
	Error     *string `json:"error"`
	Endpoint  string  `json:"endpoint"`
}
