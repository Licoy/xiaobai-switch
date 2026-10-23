package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type candidate struct {
	dir    string
	parsed parsedSkill
}

func parseDirectory(dir string) (*candidate, error) {
	p := filepath.Join(dir, enabledFile)
	if _, e := os.Lstat(p); os.IsNotExist(e) {
		return nil, nil
	} else if e != nil {
		return nil, e
	}
	text, e := readText(p)
	if e != nil {
		return nil, e
	}
	parsed, e := parseSkill(text)
	if e != nil {
		return nil, e
	}
	return &candidate{dir: dir, parsed: parsed}, nil
}
func childSkills(dir string) ([]candidate, error) {
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return []candidate{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []candidate{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		c, e := parseDirectory(filepath.Join(dir, entry.Name()))
		if e != nil {
			return nil, e
		}
		if c != nil {
			out = append(out, *c)
		}
	}
	return out, nil
}
func discover(payload string) ([]candidate, error) {
	skills := filepath.Join(payload, "skills")
	out, e := childSkills(skills)
	if e != nil || len(out) > 0 {
		return out, e
	}
	entries, e := os.ReadDir(skills)
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			items, e := childSkills(filepath.Join(skills, entry.Name()))
			if e != nil {
				return nil, e
			}
			out = append(out, items...)
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	for _, p := range []string{".agents/skills", ".claude/skills", ".codex/skills", ".pi/agent/skills", ".prime/agent/skills"} {
		items, e := childSkills(filepath.Join(payload, filepath.FromSlash(p)))
		if e != nil {
			return nil, e
		}
		out = append(out, items...)
	}
	if len(out) > 0 {
		return out, nil
	}
	return findCandidates(payload, 0)
}
func findCandidates(dir string, depth int) ([]candidate, error) {
	if depth > 12 {
		return nil, validation("skill nesting exceeds limit")
	}
	c, e := parseDirectory(dir)
	if e != nil {
		return nil, e
	}
	if c != nil {
		return []candidate{*c}, nil
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil, e
	}
	out := []candidate{}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			items, e := findCandidates(filepath.Join(dir, entry.Name()), depth+1)
			if e != nil {
				return nil, e
			}
			out = append(out, items...)
		}
	}
	return out, nil
}

type publication struct {
	source, target, previous string
	hadPrevious, published   bool
}

func rollbackPublications(ops []publication, rename func(string, string) error) error {
	var result error
	for i := len(ops) - 1; i >= 0; i-- {
		op := ops[i]
		if op.published {
			result = errors.Join(result, os.RemoveAll(op.target))
		}
		if op.hadPrevious {
			result = errors.Join(result, rename(op.previous, op.target))
		}
	}
	return result
}

type archiveRequest struct {
	target                         domain.SkillTarget
	archive                        []byte
	source, skillName, destination string
}

func (m *Manager) InstallArchive(target domain.SkillTarget, archive []byte, source, skillName string) (domain.SkillInstallResult, error) {
	return m.installArchive(archiveRequest{target: target, archive: archive, source: source, skillName: skillName})
}
func (m *Manager) installArchive(request archiveRequest) (result domain.SkillInstallResult, err error) {
	target, archive, source, skillName := request.target, request.archive, request.source, request.skillName
	owner, repo, e := ParseSource(source)
	if e != nil {
		return result, e
	}
	source = owner + "/" + repo
	fsMu.Lock()
	defer fsMu.Unlock()
	root, e := m.root(target)
	if e != nil {
		return result, e
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return result, e
	}
	stage, e := os.MkdirTemp(root, ".xiaobai-skill-")
	if e != nil {
		return result, e
	}
	if e = fileops.SecureDirectory(stage); e != nil {
		os.RemoveAll(stage)
		return result, e
	}
	preserveStage := false
	defer func() {
		if preserveStage {
			return
		}
		cleanup := os.RemoveAll(stage)
		if cleanup != nil && err == nil {
			result.Warnings = append(result.Warnings, "Installation completed; staging cleanup failed: "+cleanup.Error())
		} else {
			err = errors.Join(err, cleanup)
		}
	}()
	extract := filepath.Join(stage, "extract")
	if e = os.Mkdir(extract, 0700); e != nil {
		return result, e
	}
	payload, e := extractArchive(archive, extract)
	if e != nil {
		return result, e
	}
	found, e := discover(payload)
	if e != nil {
		return result, e
	}
	selected := []candidate{}
	for _, c := range found {
		if skillName == "" || strings.EqualFold(skillName, c.parsed.name) || strings.EqualFold(skillName, filepath.Base(c.dir)) {
			selected = append(selected, c)
		}
	}
	if len(selected) == 0 {
		return result, validation("repository does not contain the requested valid skill")
	}
	ops := []publication{}
	seen := map[string]bool{}
	for i, c := range selected {
		name := strings.TrimSpace(c.parsed.name)
		if !safeComponent.MatchString(name) || seen[strings.ToLower(name)] {
			return result, validation("invalid or duplicate skill installation name")
		}
		seen[strings.ToLower(name)] = true
		dest := filepath.Join(root, name)
		if request.destination != "" {
			if len(selected) != 1 {
				return result, validation("update must select one skill")
			}
			dest = request.destination
		}
		op := publication{source: c.dir, target: dest, previous: filepath.Join(stage, "previous-"+strconv.Itoa(i))}
		if e = checkPath(root, dest, true); e != nil {
			return result, e
		}
		if info, statErr := os.Lstat(dest); statErr == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return result, validation("existing installation is not a directory")
			}
			manifest, _, e := managedManifest(root, dest)
			if e != nil {
				return result, validation("existing installation is unmanaged")
			}
			if normalizeSource(manifest.SourceRef) != normalizeSource(source) {
				return result, validation("existing skill belongs to another source")
			}
			disabledPath := filepath.Join(dest, disabledFile)
			if info, de := os.Lstat(disabledPath); de == nil {
				if !info.Mode().IsRegular() {
					return result, validation("disabled skill must be a regular file")
				}
				if _, ee := os.Lstat(filepath.Join(dest, enabledFile)); ee == nil {
					return result, validation("both enabled and disabled skill files exist")
				} else if !os.IsNotExist(ee) {
					return result, ee
				}
				if e = os.Rename(filepath.Join(c.dir, enabledFile), filepath.Join(c.dir, disabledFile)); e != nil {
					return result, e
				}
			} else if !os.IsNotExist(de) {
				return result, de
			}
		} else if !os.IsNotExist(statErr) {
			return result, statErr
		} else if request.destination != "" {
			return result, validation("installed skill was removed while its update was downloading")
		}
		manifest := installation{SourceRef: source, SkillName: &c.parsed.name, InstalledAt: time.Now().UTC().Format(time.RFC3339Nano)}
		raw, e := json.MarshalIndent(manifest, "", "  ")
		if e != nil {
			return result, e
		}
		if e = fileops.AtomicWrite(filepath.Join(c.dir, installManifest), raw); e != nil {
			return result, e
		}
		ops = append(ops, op)
	}
	for i := range ops {
		op := &ops[i]
		if _, e = os.Lstat(op.target); e == nil {
			if e = m.rename(op.target, op.previous); e != nil {
				rollbackErr := rollbackPublications(ops, m.rename)
				if rollbackErr != nil {
					preserveStage = true
					return result, fmt.Errorf("installation failed; recovery files retained at %s: %w", stage, errors.Join(e, rollbackErr))
				}
				return result, e
			}
			op.hadPrevious = true
		} else if !os.IsNotExist(e) {
			rollbackErr := rollbackPublications(ops, m.rename)
			if rollbackErr != nil {
				preserveStage = true
				return result, fmt.Errorf("installation failed; recovery files retained at %s: %w", stage, errors.Join(e, rollbackErr))
			}
			return result, e
		}
		if e = m.rename(op.source, op.target); e != nil {
			rollbackErr := rollbackPublications(ops, m.rename)
			if rollbackErr != nil {
				preserveStage = true
				return result, fmt.Errorf("installation failed; recovery files retained at %s: %w", stage, errors.Join(e, rollbackErr))
			}
			return result, e
		}
		op.published = true
		result.Names = append(result.Names, selected[i].parsed.name)
		result.Paths = append(result.Paths, op.target)
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	return result, nil
}
