package skills

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (m *Manager) Detail(target domain.SkillTarget, source string) (domain.SkillDetail, error) {
	fsMu.Lock()
	defer fsMu.Unlock()
	root, e := m.validateSource(target, source)
	if e != nil {
		return domain.SkillDetail{}, e
	}
	info, e := infoAt(target, root, source)
	if e != nil {
		return domain.SkillDetail{}, e
	}
	content, e := readText(source)
	if e != nil {
		return domain.SkillDetail{}, e
	}
	files, e := detailFiles(filepath.Dir(source), filepath.Dir(source), 0)
	return domain.SkillDetail{Info: info, Content: content, Files: files}, e
}
func detailFiles(root, dir string, depth int) ([]string, error) {
	if depth > 12 {
		return nil, validation("skill detail nesting exceeds limit")
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		p := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			nested, e := detailFiles(root, p, depth+1)
			if e != nil {
				return nil, e
			}
			out = append(out, nested...)
		} else if entry.Type().IsRegular() {
			rel, e := filepath.Rel(root, p)
			if e != nil {
				return nil, e
			}
			out = append(out, rel)
		}
		if len(out) > 4096 {
			return nil, validation("too many skill files")
		}
	}
	sort.Strings(out)
	return out, nil
}
func (m *Manager) SetEnabled(target domain.SkillTarget, source string, enabled bool) error {
	fsMu.Lock()
	defer fsMu.Unlock()
	if _, e := m.validateSource(target, source); e != nil {
		return e
	}
	if (filepath.Base(source) == enabledFile) == enabled {
		return nil
	}
	name := disabledFile
	if enabled {
		name = enabledFile
	}
	dest := filepath.Join(filepath.Dir(source), name)
	if _, e := os.Lstat(dest); e == nil {
		return validation("destination skill file already exists")
	} else if !os.IsNotExist(e) {
		return e
	}
	return os.Rename(source, dest)
}
func (m *Manager) Uninstall(target domain.SkillTarget, source string) error {
	fsMu.Lock()
	defer fsMu.Unlock()
	root, e := m.root(target)
	if e != nil {
		return e
	}
	if e = checkPath(root, source, true); e != nil {
		return e
	}
	if _, e := os.Lstat(source); os.IsNotExist(e) {
		return nil
	} else if e != nil {
		return e
	}
	root, e = m.validateSource(target, source)
	if e != nil {
		return e
	}
	dir := filepath.Dir(source)
	if dir == root {
		return validation("cannot remove a skills root")
	}
	remove := dir
	for current := dir; current != root; current = filepath.Dir(current) {
		p := filepath.Join(current, installManifest)
		if _, e := os.Lstat(p); os.IsNotExist(e) {
			continue
		} else if e != nil {
			return e
		}
		if _, e = readInstallation(p); e != nil {
			return e
		}
		if current == dir {
			remove = current
			break
		}
		items, e := scan(target, root, current, 0)
		if e != nil {
			return e
		}
		if len(items) == 1 && items[0].DirectoryPath == dir {
			remove = current
		}
		break
	}
	return os.RemoveAll(remove)
}
