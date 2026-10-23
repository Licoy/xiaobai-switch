package service

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/clidetect"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/store"
	"sync"
	"time"
)

type Options struct {
	Root, Home, Platform string
	OnChange             func()
	CLIEnv               *clidetect.ProbeEnv
}
type Preview struct {
	ID       string            `json:"id"`
	Target   domain.TargetKind `json:"target"`
	SiteID   string            `json:"siteId"`
	ModelID  string            `json:"modelId"`
	Files    []string          `json:"files"`
	Summary  map[string]string `json:"summary"`
	Warnings []string          `json:"warnings"`
}
type TargetStatus struct {
	Target        domain.TargetKind  `json:"target"`
	Status        domain.ApplyStatus `json:"status"`
	Fields        map[string]string  `json:"fields"`
	Differences   []string           `json:"differences"`
	Installed     bool               `json:"installed"`
	Version       *string            `json:"version"`
	ConfigPath    string             `json:"configPath"`
	CLIRefreshing bool               `json:"cliRefreshing"`
	CLIError      *string            `json:"cliError"`
}
type Hydrated struct {
	Site     domain.SiteDTO            `json:"site"`
	APIKeyID string                    `json:"apiKeyId"`
	Models   []domain.SiteModelDTO     `json:"models"`
	Preset   *domain.TargetPreset      `json:"preset"`
	Thinking domain.SiteThinkingPreset `json:"thinking"`
	Binding  *domain.TargetBinding     `json:"binding"`
	Settings domain.AppSettings        `json:"settings"`
}
type pending struct {
	Request       domain.ApplyRequest
	Revision      string
	FilesRevision string
	Expires       time.Time
}
type Service struct {
	st           *store.Store
	options      Options
	mu           sync.Mutex
	pending      map[string]pending
	writeJournal func(string, serviceJournal) error
	detector     *clidetect.Detector
}

// Interface captures the UI-facing application lifecycle without exposing plans or keys.
type Application interface {
	PrepareApply(context.Context, domain.ApplyRequest) (Preview, error)
	Apply(context.Context, string) (domain.ApplyResult, error)
	Status(context.Context) ([]TargetStatus, error)
	RefreshStatus(context.Context) ([]TargetStatus, error)
	Revert(context.Context, domain.TargetKind) error
	RestoreOfficial(context.Context, domain.TargetKind) error
}

var _ Application = (*Service)(nil)
