package service

import (
	"github.com/licoy/xiaobai-switch/internal/store"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestDevHomeDoesNotProbeProcessCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture, Windows covered by explicit empty ProbeEnv")
	}
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if e := os.Mkdir(bin, 0700); e != nil {
		t.Fatal(e)
	}
	marker := filepath.Join(home, "cli-ran")
	if e := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho invoked > '"+marker+"'\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("XIAOBAI_DEV_HOME", home)
	t.Setenv("PATH", bin)
	st, e := store.Open(filepath.Join(home, ".xiaobai-switch"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	s, e := New(st, Options{Root: filepath.Join(home, ".xiaobai-switch"), Home: home})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	state := s.detector.Snapshot()
	deadline := time.Now().Add(time.Second)
	for state.Refreshing && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		state = s.detector.Snapshot()
	}
	if state.Refreshing {
		t.Fatal("CLI detection stayed pending")
	}
	for _, tool := range state.Tools {
		if tool.Installed {
			t.Fatal("development detected a real CLI")
		}
	}
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatalf("process CLI invoked: %v", e)
	}
}
