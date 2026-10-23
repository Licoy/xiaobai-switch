package clidetect

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func shim(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix fixture scripts")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(dir, name)
	tmp := dest + ".writing"
	if e := os.WriteFile(tmp, []byte("#!/bin/sh\n"+body+"\n"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Rename(tmp, dest); e != nil {
		t.Fatal(e)
	}
	return dest
}
func TestSearchOrderingNvmAliasAndPrimePreference(t *testing.T) {
	home := t.TempDir()
	for _, p := range []string{".nvm/versions/node/v20.1.0/bin", ".nvm/versions/node/v22.1.0/bin", ".nvm/alias", ".local/share/fnm/node-versions/v23/installation/bin"} {
		if e := os.MkdirAll(filepath.Join(home, p), 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(home, ".nvm/alias/default"), []byte("20.1.0"), 0600); e != nil {
		t.Fatal(e)
	}
	dirs, e := ExtraBinDirs(home)
	if e != nil {
		t.Fatal(e)
	}
	positions := map[string]int{}
	for i, p := range dirs {
		positions[p] = i
	}
	if positions[filepath.Join(home, ".nvm/versions/node/v20.1.0/bin")] >= positions[filepath.Join(home, ".nvm/versions/node/v22.1.0/bin")] {
		t.Fatal("lost nvm default precedence")
	}
	pathDir, extra := t.TempDir(), t.TempDir()
	onPath := shim(t, pathDir, "codex", "echo 'codex-cli 1.2.3'")
	shim(t, extra, "codex", "echo 'codex-cli 9.9.9'")
	env := ProbeEnv{PathDirs: []string{pathDir}, ExtraDirs: []string{extra}}
	got, e := FindBinary("codex", env)
	if e != nil || got != onPath {
		t.Fatalf("search %s %v", got, e)
	}
	shim(t, extra, "prime", "echo 'prime 1.0.0'")
	prime := shim(t, extra, "prime-agent", "echo 'prime-agent 2.0.0'")
	info, e := ProbeWith(context.Background(), domain.Prime, env)
	if e != nil || info.Path == nil || *info.Path != prime {
		t.Fatalf("prime %+v %v", info, e)
	}
}
func TestFoundBinaryVersionFailureAndNodeShimPATH(t *testing.T) {
	dir := t.TempDir()
	shim(t, dir, "codex", "echo 'codex-cli 1.2.3'\nexit 1")
	env := ProbeEnv{ExtraDirs: []string{dir}}
	info, e := ProbeWith(context.Background(), domain.Codex, env)
	if e != nil || !info.Installed || info.Version == nil || *info.Version != "codex-cli 1.2.3" {
		t.Fatalf("nonzero version %+v %v", info, e)
	}
	shim(t, dir, "codex", "echo 'Error: ENOENT' >&2\nexit 1")
	info, e = ProbeWith(context.Background(), domain.Codex, env)
	if e != nil || !info.Installed || info.Version != nil {
		t.Fatalf("failed version %+v %v", info, e)
	}
	shim(t, dir, "node", "echo node-ok")
	shim(t, dir, "claude", "command -v node >/dev/null && echo 'Claude Code 2.3.4'")
	info, e = ProbeWith(context.Background(), domain.ClaudeCode, env)
	if e != nil || info.Version == nil {
		t.Fatalf("node PATH %+v %v", info, e)
	}
}
func TestCacheIsNonblockingAndCancellationIsBounded(t *testing.T) {
	dir := t.TempDir()
	shim(t, dir, "codex", "/bin/sleep 2\necho 'codex-cli 1.2.3'")
	d := NewDetector(ProbeEnv{ExtraDirs: []string{dir}}, time.Minute)
	start := time.Now()
	state := d.Snapshot()
	if time.Since(start) > 100*time.Millisecond || !state.Refreshing || len(state.Tools) != 0 {
		t.Fatalf("blocking snapshot %+v", state)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	info, e := ProbeWith(ctx, domain.Codex, ProbeEnv{ExtraDirs: []string{dir}})
	if e == nil || !info.Installed || time.Since(start) > time.Second {
		t.Fatalf("unbounded cancellation %+v %v", info, e)
	}
	if e = d.Close(); e != nil {
		t.Fatal(e)
	}
}

func TestRefreshNotificationMayCloseDetectorWithoutDeadlock(t *testing.T) {
	dir := t.TempDir()
	shim(t, dir, "codex", "echo 'codex-cli 1.2.3'")
	d := NewDetector(ProbeEnv{ExtraDirs: []string{dir}}, time.Minute)
	done := make(chan struct{})
	d.SetOnChange(func() {
		if state := d.Snapshot(); state.Refreshing || len(state.Tools) != 4 {
			t.Error("notification preceded completed cache")
		}
		if e := d.Close(); e != nil {
			t.Error(e)
		}
		close(done)
	})
	d.Snapshot()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("notification deadlocked on Close")
	}
}
func TestCloseSuppressesPendingRefreshNotification(t *testing.T) {
	dir := t.TempDir()
	shim(t, dir, "codex", "/bin/sleep 1\necho 'codex-cli 1.2.3'")
	d := NewDetector(ProbeEnv{ExtraDirs: []string{dir}}, time.Minute)
	notifications := make(chan struct{}, 1)
	d.SetOnChange(func() { notifications <- struct{}{} })
	d.Snapshot()
	if e := d.Close(); e != nil {
		t.Fatal(e)
	}
	select {
	case <-notifications:
		t.Fatal("notification after detector was closed")
	default:
	}
}
