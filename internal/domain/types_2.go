package domain

import "encoding/json"

var _ json.RawMessage

type WebDavSyncStatus struct {
	LastAttemptAt *int64  `json:"lastAttemptAt"`
	LastSuccessAt *int64  `json:"lastSuccessAt"`
	Status        string  `json:"status"`
	Error         *string `json:"error"`
}

type BackupOverview struct {
	LatestLocalBackupAt   *int64           `json:"latestLocalBackupAt"`
	WebdavConfigured      bool             `json:"webdavConfigured"`
	WebdavAutoSyncEnabled bool             `json:"webdavAutoSyncEnabled"`
	WebdavSync            WebDavSyncStatus `json:"webdavSync"`
	NextScheduledAt       *int64           `json:"nextScheduledAt"`
}

type BackupOperationResult struct {
	FileName  string  `json:"fileName"`
	LocalPath *string `json:"localPath"`
	Uploaded  bool    `json:"uploaded"`
	Warning   *string `json:"warning"`
}

type RestoreStartupResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SwitchRouteResult struct {
	Site    SiteDTO             `json:"site"`
	Results []ApplyTargetResult `json:"results"`
}

type SwitchSiteApiKeyResult struct {
	Site    SiteDTO             `json:"site"`
	Models  []SiteModelDTO      `json:"models"`
	Fetch   ModelFetchOutcome   `json:"fetch"`
	Results []ApplyTargetResult `json:"results"`
}

type UrlProbeResult struct {
	URL       string  `json:"url"`
	Ok        bool    `json:"ok"`
	LatencyMS uint64  `json:"latencyMs"`
	Status    *uint16 `json:"status"`
	Error     *string `json:"error"`
}

type HttpBytesResult struct {
	Status      uint16 `json:"status"`
	ContentType string `json:"contentType"`
	FinalURL    string `json:"finalUrl"`
	Base64      string `json:"base64"`
}

type ApplyRecordDTO struct {
	ID                       string  `json:"id"`
	SiteID                   *string `json:"siteId"`
	SiteNameSnapshot         string  `json:"siteNameSnapshot"`
	Target                   string  `json:"target"`
	ModelID                  string  `json:"modelId"`
	ProviderID               *string `json:"providerId"`
	Status                   string  `json:"status"`
	BackupDir                *string `json:"backupDir"`
	Error                    *string `json:"error"`
	AppliedAt                int64   `json:"appliedAt"`
	SiteAPIKeyID             *string `json:"siteApiKeyId"`
	SiteAPIKeyLabelSnapshot  *string `json:"siteApiKeyLabelSnapshot"`
	SiteAPIKeyPrefixSnapshot *string `json:"siteApiKeyPrefixSnapshot"`
}

type BackupInfo struct {
	ID               string   `json:"id"`
	Target           string   `json:"target"`
	Dir              string   `json:"dir"`
	CreatedAt        int64    `json:"createdAt"`
	Files            []string `json:"files"`
	ApplyRecordID    *string  `json:"applyRecordId"`
	SiteNameSnapshot *string  `json:"siteNameSnapshot"`
	ModelID          *string  `json:"modelId"`
}

type BackupFileInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type BackupPreview struct {
	ID      string             `json:"id"`
	Summary map[string]*string `json:"summary"`
	Files   []BackupFileInfo   `json:"files"`
}

type CliToolInfo struct {
	Kind      TargetKind `json:"kind"`
	Installed bool       `json:"installed"`
	Version   *string    `json:"version"`
	Path      *string    `json:"path"`
}

type TouchedKeys struct {
	Paths         []string  `json:"paths"`
	CreatedPaths  []string  `json:"createdPaths"`
	EnvKeys       []string  `json:"envKeys"`
	ClaudeEnvKeys []string  `json:"claudeEnvKeys"`
	ShellRcPaths  *[]string `json:"shellRcPaths"`
}
