package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/envinject"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"path/filepath"
	"strings"
	"time"
)

type Request struct {
	Target         domain.TargetKind
	Site           domain.SiteRow
	APIKey         string `json:"-"`
	ModelID        string
	Home           string
	EnvPath        string
	EnvMode        string
	ShellHome      string
	Platform       string
	Claude         domain.ClaudeApplyOptions
	Codex          domain.CodexApplyOptions
	Pi             domain.PiApplyOptions
	Prime          domain.PrimeApplyOptions
	BeforeBinding  *domain.TargetBinding
	ForceExclusive bool
}
type Plan struct {
	Target    domain.TargetKind    `json:"target"`
	Changes   []fileops.Change     `json:"files"`
	Binding   domain.TargetBinding `json:"binding"`
	SiteID    string               `json:"-"`
	Injection *envinject.Plan      `json:"-"`
}
type ExecuteOptions struct {
	BackupDir string
	LocksDir  string
}
type document interface {
	get([]string) (any, bool)
	set([]string, any, bool) error
	bytes() []byte
}
type editor struct {
	doc    document
	change fileops.Change
	plan   *Plan
	name   string
}

func Build(r Request) (Plan, error) {
	p := Plan{Target: r.Target, SiteID: r.Site.ID}
	if !r.Target.Valid() || r.Site.ID == "" || strings.TrimSpace(r.ModelID) == "" || r.APIKey == "" {
		return p, errors.New("validation_failed: target, site, model and API key are required")
	}
	if e := ValidateOptions(r); e != nil {
		return p, e
	}
	if !filepath.IsAbs(r.Home) {
		return p, errors.New("validation_failed: absolute target home required")
	}
	if _, e := normalizedURL(r.Site.BaseURL, false); e != nil {
		return p, e
	}
	id := r.Site.ID
	p.Binding = domain.TargetBinding{Target: r.Target, SiteID: &id, SiteNameSnapshot: r.Site.Name, ModelID: r.ModelID, KeyFingerprint: fingerprint(r.APIKey), ExpectedFields: map[string]string{}, AppliedAt: time.Now().UnixMilli()}
	if r.BeforeBinding != nil {
		normalized := normalizeBinding(*r.BeforeBinding)
		r.BeforeBinding = &normalized
	}
	var e error
	switch r.Target {
	case domain.ClaudeCode:
		e = buildClaude(r, &p)
	case domain.Codex:
		e = buildCodex(r, &p)
	case domain.Pi, domain.Prime:
		e = buildAgent(r, &p)
	}
	if e == nil {
		carryBaselines(r.BeforeBinding, &p)
		e = carryOwnership(r.BeforeBinding, &p)
		if e == nil {
			e = validatePlan(p)
		}
	}
	return p, e
}
func Execute(p Plan, o ExecuteOptions) (result fileops.Result, err error) {
	keys := []string{string(p.Target)}
	if p.SiteID != "" {
		keys = append(keys, "site:"+p.SiteID)
	}
	var cli []string
	if p.Target == domain.Pi || p.Target == domain.Prime {
		for _, c := range p.Changes {
			if filepath.Base(c.Path) == "auth.json" || filepath.Base(c.Path) == "settings.json" {
				cli = append(cli, c.Path)
			}
		}
	}
	held, e := fileops.Acquire(o.LocksDir, keys, cli)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	if p.Injection != nil {
		injection := *p.Injection
		injection.Files = p.Changes
		return envinject.Execute(injection, o.BackupDir)
	}
	return fileops.Execute(p.Changes, o.BackupDir)
}
func openEditor(p *Plan, path, mode string) (*editor, error) {
	c, e := fileops.Snapshot(path, nil)
	if e != nil {
		return nil, e
	}
	var d document
	if mode == "toml" {
		d, e = parseTOML(c.Before)
	} else {
		d, e = parseJSON(c.Before, mode)
	}
	if e != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), e)
	}
	return &editor{d, c, p, filepath.Base(path)}, nil
}
func (e *editor) write(path []string, value any, remove bool) error {
	key := e.name + ":" + joinPath(path)
	old, exists := e.doc.get(path)
	baseline := missing
	if exists {
		b, err := json.Marshal(old)
		if err != nil {
			return err
		}
		baseline = string(b)
	}
	secret := e.name == "auth.json" || isSecret(path)
	if secret || containsSecret(old) {
		baseline = missing
	}
	if err := e.doc.set(path, value, remove); err != nil {
		return err
	}
	e.plan.Binding.ExpectedFields["baseline:"+key] = baseline
	if remove {
		e.plan.Binding.ExpectedFields[key] = missing
	} else {
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		s := string(b)
		if secret {
			s = "sha256:" + fingerprint(s)
		}
		e.plan.Binding.ExpectedFields[key] = s
	}
	return nil
}
func (e *editor) finish() {
	e.change.After = e.doc.bytes()
	e.plan.Changes = append(e.plan.Changes, e.change)
	e.plan.Binding.ManagedPaths = append(e.plan.Binding.ManagedPaths, e.change.Path)
}

const missing = "__xiaobai_missing__"

func fingerprint(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func isSecret(path []string) bool {
	for _, s := range path {
		if s == "key" || strings.Contains(s, "API_KEY") || strings.Contains(s, "AUTH_TOKEN") {
			return true
		}
	}
	return false
}
func normalizedURL(raw string, codex bool) (string, error) {
	p, e := netx.NormalizeURL(raw)
	if e != nil {
		return "", e
	}
	if codex {
		return p.CodexBaseURL, nil
	}
	return p.ClaudeBaseURL, nil
}

func containsSecret(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for k, item := range v {
			lower := strings.ToLower(k)
			if strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
				return true
			}
			if containsSecret(item) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if containsSecret(item) {
				return true
			}
		}
	}
	return false
}
func validatePlan(p Plan) error {
	for _, c := range p.Changes {
		if c.Delete {
			continue
		}
		switch filepath.Base(c.Path) {
		case "config.toml":
			if _, e := parseTOML(c.After); e != nil {
				return e
			}
		case "models.json":
			if _, e := parseJSON(c.After, "models"); e != nil {
				return e
			}
		case "settings.json", "auth.json", "xiaobai-model-catalog.json":
			if _, e := parseJSON(c.After, "strict"); e != nil {
				return e
			}
		}
	}
	return nil
}

// ExecuteHeld requires the caller to hold site, target and CLI file locks through
// its durable database commit; it deliberately does not reacquire those locks.
func ExecuteHeld(p Plan, backupDir string) (fileops.Result, error) {
	if p.Injection != nil {
		injection := *p.Injection
		injection.Files = p.Changes
		return envinject.Execute(injection, backupDir)
	}
	return fileops.Execute(p.Changes, backupDir)
}
