package service

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) validatePath(path string) (string, error) {
	valid, e := paths.ValidatePath(path)
	if e != nil {
		return "", e
	}
	actual, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	if os.Getenv("XIAOBAI_DEV_HOME") == "" && filepath.Clean(actual) != filepath.Clean(s.options.Home) {
		inside := func(root string) bool {
			rel, e := filepath.Rel(root, valid)
			return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
		}
		if !inside(s.options.Home) && !inside(s.options.Root) {
			return "", domain.NewError("validation_failed", "path escapes explicit service home")
		}
	}
	return valid, nil
}
func (s *Service) validateBinding(b *domain.TargetBinding) error {
	if b == nil {
		return nil
	}
	for _, path := range b.ManagedPaths {
		if _, e := s.validatePath(path); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) validateTargetFiles(target domain.TargetKind, home string) error {
	names := []string{"settings.json"}
	switch target {
	case domain.Codex:
		names = []string{"config.toml", "xiaobai-model-catalog.json"}
	case domain.Pi, domain.Prime:
		names = []string{"models.json", "auth.json", "settings.json"}
	}
	for _, name := range names {
		if _, e := s.validatePath(filepath.Join(home, name)); e != nil {
			return e
		}
	}
	env, e := s.path("env", "codex.env")
	if e != nil {
		return e
	}
	if _, e = s.validatePath(env); e != nil {
		return e
	}
	if target == domain.Codex {
		for _, name := range []string{".zshrc", ".bashrc", ".profile"} {
			if _, e = s.validatePath(filepath.Join(s.options.Home, name)); e != nil {
				return e
			}
		}
	}
	return nil
}
