package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDevelopmentHomeIsolation(t *testing.T) {
	root := canonicalTemp(t)
	t.Setenv("XIAOBAI_DEV_HOME", root)
	t.Setenv("CODEX_HOME", "/live/codex")
	t.Setenv("PI_CODING_AGENT_DIR", "/live/pi")
	for _, fn := range []func(string) (string, error){ResolveClaudeHome, ResolveCodexHome, ResolvePiAgentDir, ResolvePrimeAgentDir} {
		got, e := fn("")
		if e != nil || !inside(root, got) {
			t.Fatalf("path escaped %q %v", got, e)
		}
		if _, e = fn("/live/config"); e == nil {
			t.Fatal("accepted outside override")
		}
		want := filepath.Join(root, "override")
		got, e = fn(want)
		if e != nil || got != want {
			t.Fatalf("override %q %v", got, e)
		}
	}
	if _, e := EnsureAppDirs(); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(root, ".xiaobai-switch", "backups", "app")); e != nil {
		t.Fatal(e)
	}
}
func TestRejectRelativeRealAndSymlinkEscape(t *testing.T) {
	t.Setenv("XIAOBAI_DEV_HOME", "relative")
	if _, e := HomeDir(); e == nil {
		t.Fatal("accepted relative home")
	}
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("XIAOBAI_DEV_HOME", home)
	if _, e = HomeDir(); e == nil {
		t.Fatal("accepted system home")
	}
	root := canonicalTemp(t)
	outside := t.TempDir()
	t.Setenv("XIAOBAI_DEV_HOME", root)
	link := filepath.Join(root, "escape")
	if e = os.Symlink(outside, link); e != nil {
		t.Fatal(e)
	}
	if _, e = ResolveCodexHome(filepath.Join(link, "new")); e == nil {
		t.Fatal("accepted symlink escape")
	}
}
func TestConfiguredPathBeatsEnvironment(t *testing.T) {
	t.Setenv("XIAOBAI_DEV_HOME", "")
	base := canonicalTemp(t)
	for _, tt := range []struct {
		env string
		fn  func(string) (string, error)
	}{{"CODEX_HOME", ResolveCodexHome}, {"PI_CODING_AGENT_DIR", ResolvePiAgentDir}, {"PRIME_AGENT_CODING_AGENT_DIR", ResolvePrimeAgentDir}} {
		env := filepath.Join(base, "env")
		setting := filepath.Join(base, "setting")
		t.Setenv(tt.env, env)
		got, e := tt.fn(setting)
		if e != nil || got != setting {
			t.Fatalf("setting %q %v", got, e)
		}
		got, e = tt.fn("")
		if e != nil || got != env {
			t.Fatalf("env %q %v", got, e)
		}
	}
}

func canonicalTemp(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestRejectRootAndSystemHomeAncestor(t *testing.T) {
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	for _, root := range []string{filepath.VolumeName(home) + string(filepath.Separator), filepath.Dir(home)} {
		t.Setenv("XIAOBAI_DEV_HOME", root)
		if _, e := HomeDir(); e == nil {
			t.Fatalf("accepted dangerous development home %q", root)
		}
	}
}
