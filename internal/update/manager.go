package update

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

func New(opts Options) (*Manager, error) {
	if opts.Settings == nil {
		return nil, errors.New("update settings provider is required")
	}
	if _, err := opts.Settings(); err != nil {
		return nil, err
	}
	opts.Disabled = opts.Disabled || os.Getenv("XIAOBAI_DEV_HOME") != ""
	enabled := !opts.Disabled && mygo.Updater.Enabled()
	if enabled {
		if err := configureHTTP(opts.Settings); err != nil {
			return nil, err
		}
	}
	b := backend{enabled: mygo.Updater.Enabled, check: func(ctx context.Context) (*candidate, error) {
		u, e := mygo.Updater.Check(ctx)
		if e != nil || u == nil {
			return nil, e
		}
		return &candidate{version: u.Version, notes: u.Notes, date: u.Date, install: u.Install}, nil
	}}
	m := newManager(opts, b)
	m.httpInstalled = enabled
	return m, nil
}
func newManager(opts Options, b backend) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{opts: opts, backend: b, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1)}
	m.state.Enabled = !opts.Disabled && b.enabled()
	m.workers.Add(1)
	go m.schedule()
	return m
}
func (m *Manager) Snapshot() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := m.state
	if snapshot.Error != nil {
		message := *snapshot.Error
		snapshot.Error = &message
	}
	return snapshot
}
func (m *Manager) notify() {
	if m.opts.OnChange != nil {
		m.opts.OnChange(m.Snapshot())
	}
}
func (m *Manager) SettingsChanged() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}
func (m *Manager) Cancel() {
	m.mu.Lock()
	cancel := m.operationCancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
func (m *Manager) Close() error {
	m.closeOnce.Do(func() {
		m.mu.Lock()
		m.closed = true
		m.mu.Unlock()
		m.cancel()
		m.Cancel()
		m.workers.Wait()
		if m.httpInstalled {
			closeHTTP()
		}
	})
	return nil
}

func cleanError(err error) string {
	var u *url.Error
	if errors.As(err, &u) {
		return u.Op + ": " + cleanError(u.Err)
	}
	// Proxy URLs may include credentials, never expose raw URL-bearing network errors.
	raw := err.Error()
	if strings.Contains(raw, "://") {
		return "update request failed"
	}
	return raw
}
func (m *Manager) begin(parent context.Context, install bool) (context.Context, func(), error) {
	if parent == nil {
		return nil, nil, errors.New("update context is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, nil, errors.New("update manager closed")
	}
	if !m.state.Enabled {
		return nil, nil, mygo.ErrUpdatesDisabled
	}
	if m.state.Checking || m.state.Installing {
		return nil, nil, errors.New("update operation already running")
	}
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(m.ctx, cancel)
	m.operationCancel = cancel
	m.state.Error = nil
	m.state.Checking = !install
	m.state.Installing = install
	if install {
		m.state.Downloaded = 0
		m.state.Total = 0
	}
	m.workers.Add(1)
	return ctx, func() {
		stop()
		cancel()
		m.mu.Lock()
		m.operationCancel = nil
		m.state.Checking = false
		m.state.Installing = false
		m.mu.Unlock()
		m.workers.Done()
		m.notify()
	}, nil
}
func (m *Manager) Check(parent context.Context) (err error) {
	ctx, finish, err := m.begin(parent, false)
	if err != nil {
		return err
	}
	defer finish()
	m.notify()
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, err := m.backend.check(bounded)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		message := cleanError(err)
		m.state.Error = &message
		return errors.New(message)
	}
	m.candidate = c
	m.state.Available = c != nil
	m.state.Installed = false
	if c != nil {
		m.state.Version = c.version
		m.state.Notes = c.notes
		m.state.Date = c.date
	} else {
		m.state.Version = ""
		m.state.Notes = ""
	}
	return nil
}
func (m *Manager) Install(parent context.Context) (err error) {
	ctx, finish, err := m.begin(parent, true)
	if err != nil {
		return err
	}
	defer finish()
	m.mu.Lock()
	c := m.candidate
	m.mu.Unlock()
	if c == nil {
		m.mu.Lock()
		message := "no update available"
		m.state.Error = &message
		m.mu.Unlock()
		return errors.New(message)
	}
	m.notify()
	err = c.install(ctx, func(n, total int64) {
		m.mu.Lock()
		m.state.Downloaded = n
		m.state.Total = total
		m.mu.Unlock()
		m.notify()
	})
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		message := cleanError(err)
		m.state.Error = &message
		return errors.New(message)
	}
	m.state.Installed = true
	m.state.Available = false
	m.candidate = nil
	return nil
}
