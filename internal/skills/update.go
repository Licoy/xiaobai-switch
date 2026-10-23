package skills

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
)

func managedManifest(root, dir string) (installation, string, error) {
	for current := dir; current != root; current = filepath.Dir(current) {
		path := filepath.Join(current, installManifest)
		if _, e := os.Lstat(path); os.IsNotExist(e) {
			continue
		} else if e != nil {
			return installation{}, "", e
		}
		manifest, e := readInstallation(path)
		return manifest, current, e
	}
	return installation{}, "", validation("existing installation is unmanaged")
}

// Update uses the installed source manifest and publishes at the existing directory
func (m *Manager) Update(ctx context.Context, target domain.SkillTarget, sourcePath string) (domain.SkillInstallResult, error) {
	fsMu.Lock()
	root, e := m.validateSource(target, sourcePath)
	if e != nil {
		fsMu.Unlock()
		return domain.SkillInstallResult{}, e
	}
	info, e := infoAt(target, root, sourcePath)
	if e != nil {
		fsMu.Unlock()
		return domain.SkillInstallResult{}, e
	}
	manifest, _, e := managedManifest(root, filepath.Dir(sourcePath))
	fsMu.Unlock()
	if e != nil {
		return domain.SkillInstallResult{}, e
	}
	archive, warnings, e := m.download(ctx, manifest.SourceRef)
	if e != nil {
		return domain.SkillInstallResult{}, e
	}
	result, e := m.installArchive(archiveRequest{target: target, archive: archive, source: manifest.SourceRef, skillName: info.Name, destination: filepath.Dir(sourcePath)})
	result.Warnings = append(warnings, result.Warnings...)
	return result, e
}
