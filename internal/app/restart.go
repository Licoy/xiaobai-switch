package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

func (s *Shell) restart() {
	if s.Config.Development {
		s.RestartRequested = true
		mygo.App.Quit()
		return
	}
	mygo.App.Relaunch()
}
func developmentRestartCommand(config Config, executable string, environment []string) (*exec.Cmd, error) {
	if !config.Development || !filepath.IsAbs(config.Home) || !filepath.IsAbs(executable) {
		return nil, errors.New("development restart requires an absolute home and executable")
	}
	args := []string{"--dev-home", config.Home}
	if config.UseRealHome {
		args = []string{"--use-real-home"}
	}
	cmd := exec.Command(executable, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "XIAOBAI_DEV_HOME=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	if !config.UseRealHome {
		cmd.Env = append(cmd.Env, "XIAOBAI_DEV_HOME="+config.Home)
	}
	return cmd, nil
}

// Run's final defer launches only after every store, manager and instance lock has closed successfully
func launchDevelopment(config Config) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	cmd, err := developmentRestartCommand(config, executable, os.Environ())
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
