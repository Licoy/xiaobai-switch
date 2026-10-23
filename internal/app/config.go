package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/licoy/xiaobai-switch/internal/buildinfo"
	"github.com/licoy/xiaobai-switch/internal/l10n"
	"github.com/licoy/xiaobai-switch/internal/paths"
)

type Config struct {
	Home        string
	DataDir     string
	Development bool
	UseRealHome bool
	Messages    l10n.Catalog
}
type LaunchOptions struct {
	DevHome     string
	UseRealHome bool
}

func LoadConfig() (Config, error) { return LoadConfigWithOptions(LaunchOptions{}) }
func LoadConfigWithOptions(options LaunchOptions) (Config, error) {
	devHome := os.Getenv("XIAOBAI_DEV_HOME")
	if options.UseRealHome && (options.DevHome != "" || devHome != "") {
		return Config{}, errors.New("--use-real-home conflicts with --dev-home and XIAOBAI_DEV_HOME")
	}
	if options.DevHome != "" {
		if devHome != "" && devHome != options.DevHome {
			return Config{}, errors.New("--dev-home conflicts with XIAOBAI_DEV_HOME")
		}
		if err := os.Setenv("XIAOBAI_DEV_HOME", options.DevHome); err != nil {
			return Config{}, err
		}
		devHome = options.DevHome
	}
	development := strings.Contains(buildinfo.Version, "-dev") || devHome != "" || options.UseRealHome
	home, err := paths.HomeDir()
	if err != nil {
		return Config{}, err
	}
	messages, err := l10n.Load(os.Getenv("LANG"))
	if err != nil {
		return Config{}, err
	}
	return Config{Home: home, DataDir: filepath.Join(home, ".xiaobai-switch"), Development: development, UseRealHome: devHome == "", Messages: messages}, nil
}
func (s *Shell) developmentNotice() string {
	if s.Config.UseRealHome {
		return s.text("real_home_development_notice")
	}
	return s.text("isolation")
}
