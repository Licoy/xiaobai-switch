package app

import (
	"context"

	"errors"
	"github.com/egoist/mygo/ui"
	"strings"
	"sync"
	"time"

	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/l10n"
	"github.com/licoy/xiaobai-switch/internal/service"
	"github.com/licoy/xiaobai-switch/internal/siteicon"
	"github.com/licoy/xiaobai-switch/internal/store"
	"github.com/licoy/xiaobai-switch/internal/update"
	"github.com/licoy/xiaobai-switch/internal/webdav"
)

type Shell struct {
	StatusRefreshing, StatusRefreshPending              bool
	BackupRefreshing, BackupRefreshPending              bool
	Updates                                             *update.Manager
	Backups                                             *webdav.Manager
	BackupUI                                            backupState
	RestoreReady, RestartRequested                      bool
	CloseOnce                                           sync.Once
	CloseError                                          error
	Skills                                              skillsState
	ModelSearch                                         string
	ModelProbes                                         map[string]domain.ModelProbeResult
	BatchCancel                                         context.CancelFunc
	BatchProgress, BatchTotal                           int
	RouteProbes                                         map[string][]domain.UrlProbeResult
	RouteProbeTimes                                     map[string]time.Time
	QuotaSuccess                                        map[string]domain.SiteQuota
	ErrorDetail, WarningDetail, StartupNotice           string
	ErrorOpen                                           bool
	Context                                             context.Context
	Cancel                                              context.CancelFunc
	Work                                                sync.WaitGroup
	Closing                                             bool
	ShutdownError                                       error
	Warning                                             string
	TargetBackups                                       []domain.BackupInfo
	Service                                             *service.Service
	Statuses                                            map[domain.TargetKind]service.TargetStatus
	StatusLoaded                                        bool
	OperationResults                                    []domain.ApplyTargetResult
	Config                                              Config
	Store                                               *store.Store
	Settings                                            domain.AppSettings
	Current, Selected, Search, Notice, Error, TrayError string
	Sites                                               []domain.SiteDTO
	Models                                              []domain.SiteModelDTO
	Editor                                              *siteEditor
	KeyEditor                                           *keyEditor
	ModelInput                                          string
	Confirm                                             *confirmation
	Busy                                                bool
	Invalidate                                          func()
	SettingsChanged                                     func(domain.AppSettings) error
	Drafts                                              map[domain.TargetKind]*targetDraft
	Quotas                                              map[string]domain.SiteQuota
	PromptInTray                                        func() bool
	SiteIcons                                           map[string]*siteIcon
	FetchIcon                                           func(context.Context, domain.AppSettings, string, func([]byte) bool) siteicon.Result
	SiteFilter                                          int
	themeKey                                            string
	themeCache                                          *ui.Theme
}

type confirmation struct {
	ToggleLabel string
	Toggle      bool
	Tray        bool
	Dismiss     func()

	Title, Detail string
	Action        func() error
	Secret        bool
	Danger        bool
}

func NewShell(config Config, db *store.Store) (*Shell, error) {
	settings, err := db.GetSettings()
	if err != nil {
		return nil, err
	}
	if err := validateSettings(settings); err != nil {
		return nil, err
	}
	catalog, err := l10n.Load(settings.Language)
	if err != nil {
		return nil, err
	}
	config.Messages = catalog
	s := &Shell{Config: config, Store: db, Settings: settings, Current: "overview", Drafts: map[domain.TargetKind]*targetDraft{}, Quotas: map[string]domain.SiteQuota{}, QuotaSuccess: map[string]domain.SiteQuota{}}
	s.Context, s.Cancel = context.WithCancel(context.Background())
	s.RouteProbes = map[string][]domain.UrlProbeResult{}
	s.RouteProbeTimes = map[string]time.Time{}
	s.ModelProbes = map[string]domain.ModelProbeResult{}
	s.Skills = newSkillsState()
	s.Statuses = map[domain.TargetKind]service.TargetStatus{}
	s.Service, err = service.New(db, service.Options{Root: config.DataDir, Home: config.Home, OnChange: s.serviceChanged})
	if err != nil {
		return nil, err
	}
	s.BackupUI.Config, err = db.GetWebDAVConfig()
	if err != nil {
		s.Cancel()
		return nil, errors.Join(err, s.Service.Close())
	}
	s.Backups, err = webdav.NewManager(config.DataDir, db, s.backupChanged)
	if err != nil {
		s.Cancel()
		return nil, errors.Join(err, s.Service.Close())
	}
	if err = s.reload(); err != nil {
		s.Cancel()
		return nil, errors.Join(err, s.Backups.Close(), s.Service.Close())
	}
	s.Updates, err = update.New(update.Options{Settings: db.GetSettings, Disabled: config.Development, OnChange: s.updateChanged})
	if err != nil {
		return nil, errors.Join(err, s.closeManagers())
	}
	return s, nil
}
func (s *Shell) text(key string) string { return s.Config.Messages.Text(key) }
func (s *Shell) label(r Route) string {
	if r.Group == "apply" {
		return r.Label
	}
	return s.text(r.Label)
}
func value[T any](p *T) T {
	if p != nil {
		return *p
	}
	var zero T
	return zero
}
func pointer[T any](v T) *T { return &v }
func optional(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}
func (s *Shell) site() *domain.SiteDTO {
	for i := range s.Sites {
		if s.Sites[i].ID == s.Selected {
			return &s.Sites[i]
		}
	}
	return nil
}
func (s *Shell) reload() error {
	sites, err := s.Store.ListSites()
	if err != nil {
		return err
	}
	s.Sites = sites
	if s.site() == nil {
		s.Selected = ""
		if len(sites) > 0 {
			s.Selected = sites[0].ID
		}
	}
	s.Models = nil
	if s.Selected != "" {
		s.Models, err = s.Store.ListModels(s.Selected)
	}
	if err != nil {
		return err
	}
	for _, draft := range s.Drafts {
		var found *domain.SiteDTO
		for i := range s.Sites {
			if s.Sites[i].ID == draft.Request.SiteID {
				found = &s.Sites[i]
				break
			}
		}
		if found == nil {
			draft.Request.SiteID = ""
			draft.Models = nil
			s.hydrateAppliedDraft(draft)
			if draft.Request.SiteID == "" {
				for _, site := range s.Sites {
					if site.Enabled {
						s.hydrateDraft(draft, site)
						break
					}
				}
			}
			continue
		}
		if draft.Request.APIKeyID != value(found.ActiveAPIKeyID) {
			s.hydrateDraft(draft, *found)
		} else {
			draft.Models, err = s.Store.ListModelsForKey(found.ID, draft.Request.APIKeyID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
func (s *Shell) result(err error) bool {
	s.Notice = ""
	s.Error = ""
	s.ErrorDetail = ""
	if err != nil {
		s.Error = s.errorText(err)
		s.ErrorDetail = s.safeErrorDetail(err.Error())
		return false
	}
	return true
}
func (s *Shell) mutate(action func() error) {
	if s.writesBlocked() {
		return
	}
	if s.result(action()) {
		s.result(s.reload())
	}
}
func (s *Shell) selectSite(id string) {
	s.Selected = id
	if err := s.reload(); err != nil {
		s.result(err)
	}
}
func (s *Shell) confirm(title, detail string, action func() error) {
	s.Confirm = &confirmation{Title: title, Detail: detail, Action: action}
}
func (s *Shell) confirmDanger(title, detail string, action func() error) {
	s.Confirm = &confirmation{Title: title, Detail: detail, Action: action, Danger: true}
}
func (s *Shell) reorder(id string, delta int) {
	ids := make([]string, len(s.Sites))
	for i, site := range s.Sites {
		ids[i] = site.ID
	}
	for i, v := range ids {
		if v == id {
			j := i + delta
			if j < 0 || j >= len(ids) {
				return
			}
			ids[i], ids[j] = ids[j], ids[i]
			break
		}
	}
	s.mutate(func() error { return s.Service.ReorderSites(s.Context, ids) })
}
func (s *Shell) enable(siteID string, on bool) {
	s.serviceChange(func(ctx context.Context) error {
		_, err := s.Service.UpdateSite(ctx, siteID, domain.UpdateSiteInput{Enabled: &on})
		return err
	})
}
