package skills

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const enabledFile = "SKILL.md"
const disabledFile = "SKILL.md.disabled"
const installManifest = ".xiaobai-skill.json"
const maxSkillBytes int64 = 1024 * 1024

var fsMu sync.Mutex
var targets = []domain.SkillTarget{domain.SkillAgents, domain.SkillClaudeCode, domain.SkillCodex, domain.SkillPi, domain.SkillPrime}

type Manager struct {
	roots  map[domain.SkillTarget]string
	client *http.Client
	rename func(string, string) error
}

func New(settings domain.AppSettings, client *http.Client) (*Manager, error) {
	if client == nil {
		return nil, domain.NewError("validation_failed", "skills HTTP client is required")
	}
	home, e := paths.HomeDir()
	if e != nil {
		return nil, e
	}
	optional := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	roots := map[domain.SkillTarget]string{domain.SkillAgents: filepath.Join(home, ".agents")}
	for _, pair := range []struct {
		target   domain.SkillTarget
		resolve  func(string) (string, error)
		override *string
	}{{domain.SkillClaudeCode, paths.ResolveClaudeHome, settings.ClaudeHomeOverride}, {domain.SkillCodex, paths.ResolveCodexHome, settings.CodexHomeOverride}, {domain.SkillPi, paths.ResolvePiAgentDir, settings.PiAgentDirOverride}, {domain.SkillPrime, paths.ResolvePrimeAgentDir, settings.PrimeAgentDirOverride}} {
		roots[pair.target], e = pair.resolve(optional(pair.override))
		if e != nil {
			return nil, e
		}
	}
	for t, p := range roots {
		roots[t], e = paths.ValidatePath(filepath.Join(p, "skills"))
		if e != nil {
			return nil, e
		}
	}
	return &Manager{roots: roots, client: client, rename: os.Rename}, nil
}
func validation(message string) error { return domain.NewError("validation_failed", message) }
func checkPath(root, path string, allowMissing bool) error {
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return validation("skill paths must be absolute")
	}
	rel, e := filepath.Rel(root, path)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return validation("skill escaped target root")
	}
	if rel != "." {
		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			if strings.HasPrefix(part, ".") {
				return validation("hidden skill paths cannot be managed")
			}
		}
	}
	if _, e = paths.ValidatePath(path); e != nil {
		return e
	}
	current := filepath.Clean(path)
	for {
		info, e := os.Lstat(current)
		if e == nil && info.Mode()&os.ModeSymlink != 0 {
			return validation("skill paths cannot contain symbolic links")
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if os.IsNotExist(e) && !allowMissing {
			return e
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return nil
}
func (m *Manager) root(target domain.SkillTarget) (string, error) {
	root, ok := m.roots[target]
	if !ok {
		return "", validation("unknown skill target")
	}
	return root, checkPath(root, root, true)
}
func readText(path string) (string, error) {
	info, e := os.Lstat(path)
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() || info.Size() > maxSkillBytes {
		return "", validation("skill source must be a regular file within size limit")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	if int64(len(b)) > maxSkillBytes || !utf8.Valid(b) {
		return "", validation("skill must be UTF-8 within size limit")
	}
	return string(b), nil
}
func infoAt(target domain.SkillTarget, root, source string) (domain.SkillInfo, error) {
	if e := checkPath(root, source, false); e != nil {
		return domain.SkillInfo{}, e
	}
	content, e := readText(source)
	if e != nil {
		return domain.SkillInfo{}, e
	}
	p, e := parseSkill(content)
	if e != nil {
		return domain.SkillInfo{}, e
	}
	return domain.SkillInfo{Name: p.name, Description: p.description, Author: p.author, Version: p.version, Target: target, SourcePath: source, SkillsPath: root, DirectoryPath: filepath.Dir(source), Enabled: filepath.Base(source) == enabledFile}, nil
}
func scan(target domain.SkillTarget, root, dir string, depth int) ([]domain.SkillInfo, error) {
	if depth > 12 {
		return nil, validation("skill nesting exceeds limit")
	}
	var source string
	for _, name := range []string{enabledFile, disabledFile} {
		p := filepath.Join(dir, name)
		_, e := os.Lstat(p)
		if e == nil {
			if source != "" {
				return nil, validation("both enabled and disabled skill files exist")
			}
			source = p
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}
	if source != "" {
		info, e := infoAt(target, root, source)
		if e != nil {
			return nil, e
		}
		return []domain.SkillInfo{info}, nil
	}
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return []domain.SkillInfo{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []domain.SkillInfo{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		nested, e := scan(target, root, filepath.Join(dir, entry.Name()), depth+1)
		if e != nil {
			return nil, e
		}
		out = append(out, nested...)
		if len(out) > 4096 {
			return nil, validation("too many skills")
		}
	}
	return out, nil
}
func (m *Manager) List() ([]domain.SkillInfo, error) {
	fsMu.Lock()
	defer fsMu.Unlock()
	out := []domain.SkillInfo{}
	for _, target := range targets {
		root, e := m.root(target)
		if e != nil {
			return nil, e
		}
		items, e := scan(target, root, root, 0)
		if e != nil {
			return nil, e
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Name == items[j].Name {
				return items[i].SourcePath < items[j].SourcePath
			}
			return items[i].Name < items[j].Name
		})
		out = append(out, items...)
	}
	return out, nil
}
func (m *Manager) validateSource(target domain.SkillTarget, source string) (string, error) {
	root, e := m.root(target)
	if e != nil {
		return "", e
	}
	if filepath.Base(source) != enabledFile && filepath.Base(source) != disabledFile {
		return "", validation("sourcePath must point to a skill file")
	}
	return root, checkPath(root, source, false)
}
