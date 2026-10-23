package update

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/licoy/xiaobai-switch/internal/domain"
)

func TestScheduleHourlyAndSettingsChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		settings := domain.DefaultSettings()
		settings.UpdateCheckInterval = 1 // Legacy values must not shorten the hour.
		var checks atomic.Int32
		opts := Options{Settings: func() (domain.AppSettings, error) { return settings, nil }}
		m := newManager(opts, backend{enabled: func() bool { return true }, check: func(context.Context) (*candidate, error) {
			checks.Add(1)
			return nil, nil
		}})
		defer m.Close()
		synctest.Wait()
		if checks.Load() != 1 {
			t.Fatalf("startup checks = %d", checks.Load())
		}
		time.Sleep(30 * time.Minute)
		m.SettingsChanged()
		synctest.Wait()
		if checks.Load() != 1 {
			t.Fatal("unrelated settings caused another check")
		}
		time.Sleep(30 * time.Minute)
		synctest.Wait()
		if checks.Load() != 2 {
			t.Fatalf("hourly check postponed by settings change: %d", checks.Load())
		}
		settings.AutoCheckUpdate = false
		m.SettingsChanged()
		synctest.Wait()
		time.Sleep(2 * time.Hour)
		synctest.Wait()
		if checks.Load() != 2 {
			t.Fatal("automatic checks continued after disabling")
		}
		if err := m.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
		settings.AutoCheckUpdate = true
		m.SettingsChanged()
		synctest.Wait()
		if checks.Load() != 4 {
			t.Fatalf("manual check or re-enable check missing: %d", checks.Load())
		}
		time.Sleep(time.Hour)
		synctest.Wait()
		if checks.Load() != 5 {
			t.Fatal("hourly check did not resume")
		}
		m.Close()
		time.Sleep(2 * time.Hour)
		if checks.Load() != 5 {
			t.Fatal("closed scheduler continued checking")
		}
	})
}

func TestScheduleCloseCancelsAutomaticCheck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		opts := Options{Settings: func() (domain.AppSettings, error) { return domain.DefaultSettings(), nil }}
		cancelled := false
		m := newManager(opts, backend{enabled: func() bool { return true }, check: func(ctx context.Context) (*candidate, error) {
			<-ctx.Done()
			cancelled = true
			return nil, ctx.Err()
		}})
		synctest.Wait()
		if !m.Snapshot().Checking {
			t.Fatal("automatic check did not start")
		}
		m.Close()
		if !cancelled || m.Snapshot().Checking {
			t.Fatal("close did not finish the automatic check")
		}
	})
}

func TestScheduleDisabledStartupAndInstalledState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		settings := domain.DefaultSettings()
		settings.AutoCheckUpdate = false
		var checks atomic.Int32
		m := newManager(Options{Settings: func() (domain.AppSettings, error) { return settings, nil }}, backend{
			enabled: func() bool { return true },
			check: func(context.Context) (*candidate, error) {
				checks.Add(1)
				return &candidate{version: "1.0.1", install: func(context.Context, func(int64, int64)) error { return nil }}, nil
			},
		})
		defer m.Close()
		synctest.Wait()
		time.Sleep(time.Hour)
		synctest.Wait()
		if checks.Load() != 0 {
			t.Fatal("disabled startup checked automatically")
		}
		settings.AutoCheckUpdate = true
		m.SettingsChanged()
		synctest.Wait()
		if checks.Load() != 1 {
			t.Fatal("enabling automatic checks did not check immediately")
		}
		if err := m.Install(context.Background()); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()
		if checks.Load() != 1 || !m.Snapshot().Installed {
			t.Fatal("automatic check discarded pending restart")
		}
	})
}
