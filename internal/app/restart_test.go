package app

import (
	"os"

	"github.com/licoy/xiaobai-switch/internal/buildinfo"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testUserHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XIAOBAI_DEV_HOME", "")
	return home
}

func TestDevelopmentRestartPreservesIsolationWithoutParentEnvironment(t *testing.T) {
	s := testShell(t)
	executable := filepath.Join(s.Config.Home, "XiaoBaiSwitch")
	for _, environment := range [][]string{nil, {"PATH=/usr/bin", "XIAOBAI_DEV_HOME=/wrong"}} {
		cmd, err := developmentRestartCommand(s.Config, executable, environment)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(cmd.Args, []string{executable, "--dev-home", s.Config.Home}) {
			t.Fatal("restart lost explicit isolated home")
		}
		count := 0
		for _, entry := range cmd.Env {
			if strings.HasPrefix(entry, "XIAOBAI_DEV_HOME=") {
				count++
				if entry != "XIAOBAI_DEV_HOME="+s.Config.Home {
					t.Fatal("restart retained wrong environment")
				}
			}
		}
		if count != 1 {
			t.Fatal("restart did not provide one explicit development root")
		}
	}
}
func TestDevelopmentBuildDefaultsToRealHomeBeforeStore(t *testing.T) {
	testBuildVersion(t, "1.0.0-dev")
	expected := testUserHome(t)
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !config.Development || !config.UseRealHome || config.Home != expected || config.DataDir != filepath.Join(expected, ".xiaobai-switch") {
		t.Fatalf("default development mode did not use existing user data: %+v", config)
	}
	if _, err := os.Stat(config.DataDir); !os.IsNotExist(err) {
		t.Fatalf("loading config unexpectedly created data directory: %v", err)
	}
}
func TestExplicitDevelopmentHomeRetainsIsolation(t *testing.T) {
	testUserHome(t)
	isolated, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfigWithOptions(LaunchOptions{DevHome: isolated})
	if err != nil {
		t.Fatal(err)
	}
	if !config.Development || config.UseRealHome || config.Home != isolated || config.DataDir != filepath.Join(isolated, ".xiaobai-switch") {
		t.Fatalf("explicit development home lost isolation: %+v", config)
	}
	if os.Getenv("XIAOBAI_DEV_HOME") != isolated {
		t.Fatal("explicit development home was not propagated to path resolution")
	}
}
func TestDevelopmentRestartRejectsInvalidConfiguration(t *testing.T) {
	for _, config := range []Config{{Home: "/tmp/isolation"}, {Development: true, Home: "relative"}} {
		if _, err := developmentRestartCommand(config, "/tmp/executable", os.Environ()); err == nil {
			t.Fatal("unsafe restart configuration accepted")
		}
	}
}

func TestRealHomeDevelopmentCompatibilityAndConflictsAreRejected(t *testing.T) {
	expected := testUserHome(t)
	config, err := LoadConfigWithOptions(LaunchOptions{UseRealHome: true})
	if err != nil {
		t.Fatal(err)
	}
	if !config.Development || !config.UseRealHome || config.Home != expected || config.DataDir != filepath.Join(expected, ".xiaobai-switch") {
		t.Fatal("explicit real-home mode lost development semantics")
	}
	if _, err := LoadConfigWithOptions(LaunchOptions{UseRealHome: true, DevHome: "/tmp/isolated"}); err == nil {
		t.Fatal("conflicting home flags accepted")
	}
	t.Setenv("XIAOBAI_DEV_HOME", "/tmp/isolated")
	if _, err := LoadConfigWithOptions(LaunchOptions{UseRealHome: true}); err == nil {
		t.Fatal("real-home flag accepted isolated environment")
	}
	if _, err := LoadConfigWithOptions(LaunchOptions{DevHome: "/tmp/different"}); err == nil {
		t.Fatal("different isolated roots accepted")
	}
}
func TestDefaultDevelopmentRestartUsesRealHomeWithoutIsolationEnvironment(t *testing.T) {
	testBuildVersion(t, "1.0.0-dev")
	testUserHome(t)
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(config.Home, "XiaoBaiSwitch")
	cmd, err := developmentRestartCommand(config, executable, []string{"PATH=/usr/bin", "XIAOBAI_DEV_HOME=/stale"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cmd.Args, []string{executable, "--use-real-home"}) {
		t.Fatal("restart lost default real-home mode")
	}
	for _, entry := range cmd.Env {
		if strings.HasPrefix(entry, "XIAOBAI_DEV_HOME=") {
			t.Fatal("real-home restart retained isolated environment")
		}
	}
}

func testBuildVersion(t *testing.T, version string) {
	t.Helper()
	previous := buildinfo.Version
	buildinfo.Version = version
	t.Cleanup(func() { buildinfo.Version = previous })
}

func TestReleaseBuildUsesProductionModeBeforeStore(t *testing.T) {
	testBuildVersion(t, "1.0.0")
	expected := testUserHome(t)
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Development || !config.UseRealHome || config.Home != expected || config.DataDir != filepath.Join(expected, ".xiaobai-switch") {
		t.Fatalf("release build did not use production mode: %+v", config)
	}
	if _, err := os.Stat(config.DataDir); !os.IsNotExist(err) {
		t.Fatalf("loading config unexpectedly created data directory: %v", err)
	}
}
