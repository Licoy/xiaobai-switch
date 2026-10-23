package domain

import "encoding/json"

var _ json.RawMessage

type SiteQuota struct {
	Status       QuotaProbeStatus `json:"status"`
	RemainingUSD *float64         `json:"remainingUsd"`
	UsedUSD      *float64         `json:"usedUsd"`
	TotalUSD     *float64         `json:"totalUsd"`
	Unlimited    bool             `json:"unlimited"`
	Unit         *string          `json:"unit"`
	ExpiresAt    *int64           `json:"expiresAt"`
	Source       *QuotaSource     `json:"source"`
	Endpoint     *string          `json:"endpoint"`
	FetchedAt    int64            `json:"fetchedAt"`
	LatencyMS    uint64           `json:"latencyMs"`
	Error        *string          `json:"error"`
}

type TargetLiveStatus struct {
	Kind            TargetKind         `json:"kind"`
	Installed       bool               `json:"installed"`
	Version         *string            `json:"version"`
	ConfigPath      string             `json:"configPath"`
	Status          ApplyStatus        `json:"status"`
	AppliedSiteID   *string            `json:"appliedSiteId"`
	AppliedSiteName *string            `json:"appliedSiteName"`
	AppliedModelID  *string            `json:"appliedModelId"`
	ProviderID      *string            `json:"providerId"`
	Orphan          bool               `json:"orphan"`
	LiveSummary     map[string]*string `json:"liveSummary"`
	LastAppliedAt   *int64             `json:"lastAppliedAt"`
	StaleReason     *string            `json:"staleReason"`
}

type ClaudeApplyOptions struct {
	AuthKeyStyle  *ClaudeAuthKeyStyle `json:"authKeyStyle,omitempty"`
	FableModelID  *string             `json:"fableModelId"`
	OpusModelID   *string             `json:"opusModelId"`
	SonnetModelID *string             `json:"sonnetModelId"`
	HaikuModelID  *string             `json:"haikuModelId"`
	EffortLevel   *ClaudeEffortLevel  `json:"effortLevel"`
	Use1mContext  bool                `json:"use1mContext"`
}

type CodexApplyOptions struct {
	WriteAllModels     bool                  `json:"writeAllModels"`
	ReasoningEffort    *CodexReasoningEffort `json:"reasoningEffort"`
	CatalogModels      []CatalogModel        `json:"catalogModels"`
	RemoteCompaction   bool                  `json:"remoteCompaction"`
	ImageUnderstanding bool                  `json:"imageUnderstanding"`
	ImageGeneration    bool                  `json:"imageGeneration"`
	WebSearch          bool                  `json:"webSearch"`
	CapabilitySource   CapabilitySource      `json:"capabilitySource"`
}

type PiApplyOptions struct {
	WriteAllModels bool           `json:"writeAllModels"`
	CatalogModels  []CatalogModel `json:"catalogModels"`
	Thinking       ThinkingWrite  `json:"thinking"`
}

type PrimeApplyOptions struct {
	WriteAllModels bool           `json:"writeAllModels"`
	CatalogModels  []CatalogModel `json:"catalogModels"`
	Thinking       ThinkingWrite  `json:"thinking"`
}

type ApplyTargetResult struct {
	Target      TargetKind          `json:"target"`
	Ok          bool                `json:"ok"`
	Status      ApplyStatus         `json:"status"`
	BackupPaths []string            `json:"backupPaths"`
	Message     string              `json:"message"`
	LiveSummary *map[string]*string `json:"liveSummary"`
	TouchedKeys *[]string           `json:"touchedKeys"`
}

type ApplyResult struct {
	SiteID    string              `json:"siteId"`
	ModelID   string              `json:"modelId"`
	Results   []ApplyTargetResult `json:"results"`
	AppliedAt int64               `json:"appliedAt"`
}

type AppSettings struct {
	Extra                       map[string]json.RawMessage `json:"-"`
	LaunchAtLoginHidden         *bool                      `json:"launchAtLoginHidden,omitempty"`
	Language                    string                     `json:"language"`
	ThemeMode                   string                     `json:"themeMode"`
	PrimaryColor                string                     `json:"primaryColor"`
	AutoStart                   bool                       `json:"autoStart"`
	AlwaysOnTop                 bool                       `json:"alwaysOnTop"`
	ClaudeHomeOverride          *string                    `json:"claudeHomeOverride"`
	CodexHomeOverride           *string                    `json:"codexHomeOverride"`
	PiAgentDirOverride          *string                    `json:"piAgentDirOverride"`
	PrimeAgentDirOverride       *string                    `json:"primeAgentDirOverride"`
	CodexEnvInjectMode          string                     `json:"codexEnvInjectMode"`
	ForceExclusiveClaudeAuthKey bool                       `json:"forceExclusiveClaudeAuthKey"`
	AutoCheckUpdate             bool                       `json:"autoCheckUpdate"`
	UpdateCheckInterval         uint32                     `json:"updateCheckInterval"`
	MaxBackupCopies             uint32                     `json:"maxBackupCopies"`
	ProxyMode                   string                     `json:"proxyMode"`
	ProxyProtocol               string                     `json:"proxyProtocol"`
	ProxyHost                   *string                    `json:"proxyHost"`
	ProxyPort                   *uint16                    `json:"proxyPort"`
	RouteProbeTtlMinutes        uint32                     `json:"routeProbeTtlMinutes"`
	CloseToTray                 bool                       `json:"closeToTray"`
	StartInTray                 bool                       `json:"startInTray"`
}

type WebDavConfigView struct {
	BaseURL             string `json:"baseUrl"`
	Username            string `json:"username"`
	RemotePath          string `json:"remotePath"`
	AcceptInvalidCerts  bool   `json:"acceptInvalidCerts"`
	HasPassword         bool   `json:"hasPassword"`
	AutoSyncEnabled     bool   `json:"autoSyncEnabled"`
	SyncIntervalMinutes uint32 `json:"syncIntervalMinutes"`
	MaxRemoteBackups    uint32 `json:"maxRemoteBackups"`
}

type SaveWebDavConfigInput struct {
	BaseURL             string  `json:"baseUrl"`
	Username            string  `json:"username"`
	Password            *string `json:"password"`
	RemotePath          string  `json:"remotePath"`
	AcceptInvalidCerts  bool    `json:"acceptInvalidCerts"`
	AutoSyncEnabled     bool    `json:"autoSyncEnabled"`
	SyncIntervalMinutes uint32  `json:"syncIntervalMinutes"`
	MaxRemoteBackups    uint32  `json:"maxRemoteBackups"`
}

type TestWebDavConnectionInput struct {
	BaseURL            string  `json:"baseUrl"`
	Username           string  `json:"username"`
	Password           *string `json:"password"`
	RemotePath         string  `json:"remotePath"`
	AcceptInvalidCerts bool    `json:"acceptInvalidCerts"`
}

type RemoteBackupInfo struct {
	FileName     string `json:"fileName"`
	Size         uint64 `json:"size"`
	LastModified string `json:"lastModified"`
	DeviceName   string `json:"deviceName"`
}

type LocalBackupInfo struct {
	FileName   string  `json:"fileName"`
	Size       uint64  `json:"size"`
	CreatedAt  int64   `json:"createdAt"`
	DeviceName string  `json:"deviceName"`
	Reason     *string `json:"reason"`
	AppVersion *string `json:"appVersion"`
	Error      *string `json:"error"`
}
