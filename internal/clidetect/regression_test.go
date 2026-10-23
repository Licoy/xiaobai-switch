package clidetect

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/licoy/xiaobai-switch/internal/domain"
)

func TestNvmAndFnmPreferNewestNumericVersion(t *testing.T) {
	for _, root := range []string{".nvm/versions/node", ".local/share/fnm/node-versions"} {
		t.Run(root, func(t *testing.T) {
			home := t.TempDir()
			for _, version := range []string{"v9.9.0", "v22.9.0", "v22.10.0"} {
				dir := filepath.Join(home, root, version)
				if root == ".nvm/versions/node" {
					dir = filepath.Join(dir, "bin")
				} else {
					dir = filepath.Join(dir, "installation", "bin")
				}
				shim(t, dir, "codex", "echo 'codex-cli 1.2.3'")
			}
			dirs, err := ExtraBinDirs(home)
			if err != nil {
				t.Fatal(err)
			}
			got, err := FindBinary("codex", ProbeEnv{ExtraDirs: dirs})
			expected := filepath.Join(home, root, "v22.10.0", "bin", "codex")
			if root != ".nvm/versions/node" {
				expected = filepath.Join(home, root, "v22.10.0", "installation", "bin", "codex")
			}
			if err != nil || got != expected {
				t.Fatalf("picked older Node installation: %s, %v", got, err)
			}
		})
	}
}

func TestNvmMajorAliasPrefersLatestMinor(t *testing.T) {
	home := t.TempDir()
	for _, version := range []string{"v22.9.0", "v22.10.0"} {
		shim(t, filepath.Join(home, ".nvm/versions/node", version, "bin"), "codex", "echo 'codex-cli 1.2.3'")
	}
	if err := os.MkdirAll(filepath.Join(home, ".nvm/alias"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".nvm/alias/default"), []byte("22"), 0600); err != nil {
		t.Fatal(err)
	}
	dirs, err := ExtraBinDirs(home)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FindBinary("codex", ProbeEnv{ExtraDirs: dirs})
	if err != nil || filepath.Base(filepath.Dir(filepath.Dir(got))) != "v22.10.0" {
		t.Fatalf("major alias picked older minor: %s, %v", got, err)
	}
}

func TestSuccessfulExitDoesNotTurnErrorIntoVersion(t *testing.T) {
	dir := t.TempDir()
	shim(t, dir, "codex", "echo 'Error: spawn node v22.1.0 ENOENT'")
	info, err := ProbeWith(context.Background(), domain.Codex, ProbeEnv{ExtraDirs: []string{dir}})
	if err != nil || !info.Installed || info.Version != nil {
		t.Fatalf("invalid version accepted: %+v, %v", info, err)
	}
}

func TestExplicitRefreshIgnoresTTLAndKeepsErrorsOnTheirTarget(t *testing.T) {
	dir := t.TempDir()
	shim(t, dir, "codex", "echo 'codex-cli 1.2.3'")
	d := NewDetector(ProbeEnv{ExtraDirs: []string{dir}}, time.Hour)
	defer d.Close()
	done := make(chan struct{}, 4)
	d.SetOnChange(func() { done <- struct{}{} })
	d.Snapshot()
	awaitRefresh(t, done)
	shim(t, dir, "codex", "echo 'codex-cli 2.0.0'")
	state := d.Snapshot()
	if *state.Tools[1].Version != "codex-cli 1.2.3" {
		t.Fatal("ordinary snapshot bypassed TTL")
	}
	d.Refresh()
	awaitRefresh(t, done)
	state = d.Snapshot()
	if *state.Tools[1].Version != "codex-cli 2.0.0" {
		t.Fatal("explicit refresh retained stale version")
	}
	shim(t, dir, "codex", "exit 0")
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/missing/interpreter\n"), 0700); err != nil {
		t.Fatal(err)
	}
	d.Refresh()
	awaitRefresh(t, done)
	state = d.Snapshot()
	if state.Error == nil || len(state.Errors) != 1 || state.Errors[domain.Codex] == "" {
		t.Fatalf("target error lost: %+v", state)
	}
	state.Errors[domain.Prime] = "changed outside detector"
	if d.Snapshot().Errors[domain.Prime] != "" {
		t.Fatal("snapshot exposed mutable error cache")
	}
}

func awaitRefresh(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("CLI refresh did not finish")
	}
}

func TestWindowsSearchPrefersRunnableShims(t *testing.T) {
	for _, tc := range []struct {
		name     string
		files    []string
		expected string
	}{
		{"npm cmd beside Unix shim", []string{"codex", "codex.cmd"}, "codex.cmd"},
		{"native executable first", []string{"codex", "codex.exe", "codex.cmd", "codex.bat"}, "codex.exe"},
		{"batch beside Unix shim", []string{"codex", "codex.bat"}, "codex.bat"},
		{"cmd before batch", []string{"codex.cmd", "codex.bat"}, "codex.cmd"},
		{"bare compatibility", []string{"codex"}, "codex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := findBinary("codex", ProbeEnv{PathDirs: []string{dir}}, "windows")
			if err != nil || got != filepath.Join(dir, tc.expected) {
				t.Fatalf("got %q, %v; want %q", got, err, tc.expected)
			}
		})
	}
}

func TestWindowsSearchPreservesDirectoryPriority(t *testing.T) {
	first, later, extra := t.TempDir(), t.TempDir(), t.TempDir()
	for _, path := range []string{filepath.Join(first, "codex.cmd"), filepath.Join(later, "codex.exe"), filepath.Join(extra, "codex.exe")} {
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := findBinary("codex", ProbeEnv{PathDirs: []string{first, later}, ExtraDirs: []string{extra}}, "windows")
	if err != nil || got != filepath.Join(first, "codex.cmd") {
		t.Fatalf("directory priority changed: %q, %v", got, err)
	}
}

func TestUnixSearchStillSelectsBareExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix fixture permissions")
	}
	dir := t.TempDir()
	for _, name := range []string{"codex", "codex.exe", "codex.cmd", "codex.bat"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	got, err := findBinary("codex", ProbeEnv{PathDirs: []string{dir}}, "linux")
	if err != nil || got != filepath.Join(dir, "codex") {
		t.Fatalf("Unix search changed: %q, %v", got, err)
	}
}
