package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/licoy/xiaobai-switch/internal/clidetect"
	"github.com/licoy/xiaobai-switch/internal/domain"
)

func cliStatusService(t *testing.T, name, script string) (*Service, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX CLI fixture")
	}
	s, _, _ := fixtureService(t)
	if err := s.detector.Close(); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	path := filepath.Join(bin, name)
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	s.detector = clidetect.NewDetector(clidetect.ProbeEnv{PathDirs: []string{bin}}, time.Minute)
	return s, path
}

func settledCLIStatuses(t *testing.T, s *Service) []TargetStatus {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		rows, err := s.Status(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !rows[0].CLIRefreshing {
			return rows
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("CLI detection did not settle")
	return nil
}

func TestManualStatusRefreshSeesCLIChanges(t *testing.T) {
	for i, name := range []string{"claude", "codex", "pi", "prime-agent"} {
		t.Run(name, func(t *testing.T) {
			s, path := cliStatusService(t, name, "#!/bin/sh\necho 'CLI 1.0.0'\n")
			rows := settledCLIStatuses(t, s)
			if rows[i].Version == nil || *rows[i].Version != "CLI 1.0.0" {
				t.Fatalf("initial version: %+v", rows[i])
			}
			if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'CLI 2.0.0'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			cached, err := s.Status(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if cached[i].Version == nil || *cached[i].Version != "CLI 1.0.0" {
				t.Fatal("ordinary status bypassed CLI cache")
			}
			refreshCLIStatuses(t, s)
			rows = settledCLIStatuses(t, s)
			if rows[i].Version == nil || *rows[i].Version != "CLI 2.0.0" {
				t.Fatalf("manual refresh retained cached version: %+v", rows[i])
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			refreshCLIStatuses(t, s)
			rows = settledCLIStatuses(t, s)
			if rows[i].Installed || rows[i].Version != nil {
				t.Fatalf("removed CLI remains installed: %+v", rows[i])
			}
			if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'CLI 3.0.0'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			refreshCLIStatuses(t, s)
			rows = settledCLIStatuses(t, s)
			if !rows[i].Installed || rows[i].Version == nil || *rows[i].Version != "CLI 3.0.0" {
				t.Fatalf("installed CLI not detected: %+v", rows[i])
			}
		})
	}
}

func refreshCLIStatuses(t *testing.T, s *Service) {
	t.Helper()
	if _, err := s.RefreshStatus(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCLIDetectionErrorStaysWithFailingTarget(t *testing.T) {
	s, path := cliStatusService(t, "claude", "#!/xiaobai-test-missing-interpreter\n")
	for _, row := range settledCLIStatuses(t, s) {
		if row.Target == domain.ClaudeCode {
			if row.CLIError == nil {
				t.Fatal("Claude detection error missing")
			}
		} else if row.CLIError != nil {
			t.Errorf("%s received Claude detection error: %s", row.Target, *row.CLIError)
		}
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'Claude 1.0.0'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	refreshCLIStatuses(t, s)
	row := settledCLIStatuses(t, s)[0]
	if row.CLIError != nil || row.Version == nil || *row.Version != "Claude 1.0.0" {
		t.Fatalf("manual refresh did not clear repaired CLI error: %+v", row)
	}
}
