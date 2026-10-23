package webdav

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/appbackup"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/store"
	"sync"
	"time"
)

type Manager struct {
	root          string
	store         *store.Store
	backups       *appbackup.Manager
	mu            sync.Mutex
	gate          chan struct{}
	wake          chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
	currentCancel context.CancelFunc
	wg            sync.WaitGroup
	closed        bool
	next          int64
	runtimeError  error
	onChange      func()
}

func NewManager(root string, s *store.Store, onChange func()) (*Manager, error) {
	b, e := appbackup.New(root, s)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{root: root, store: s, backups: b, gate: make(chan struct{}, 1), wake: make(chan struct{}, 1), ctx: ctx, cancel: cancel, onChange: onChange}
	if e = m.Reschedule(); e != nil {
		cancel()
		return nil, e
	}
	m.wg.Add(1)
	go m.scheduler()
	return m, nil
}
func (m *Manager) begin(ctx context.Context, reschedule bool) (context.Context, func(), error) {
	select {
	case m.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-m.ctx.Done():
		return nil, nil, domain.NewError("cancelled", "backup manager is closed")
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		<-m.gate
		return nil, nil, domain.NewError("cancelled", "backup manager is closed")
	}
	op, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(m.ctx, cancel)
	m.currentCancel = cancel
	m.wg.Add(1)
	m.mu.Unlock()
	done := func() {
		cancel()
		stop()
		m.mu.Lock()
		m.currentCancel = nil
		m.mu.Unlock()
		<-m.gate
		defer m.wg.Done()
		m.mu.Lock()
		closed := m.closed
		m.mu.Unlock()
		if reschedule && !closed {
			if e := m.Reschedule(); e != nil {
				m.reportSchedulerError(e)
			}
		}
		m.mu.Lock()
		closed = m.closed
		m.mu.Unlock()
		if m.onChange != nil && !closed {
			m.onChange()
		}
	}
	return op, done, nil
}
func (m *Manager) Cancel() {
	m.mu.Lock()
	cancel := m.currentCancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	m.next = 0
	m.cancel()
	m.mu.Unlock()
	m.wg.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runtimeError
}
func NextScheduledAt(last *int64, now, interval int64) int64 {
	if last == nil {
		return now + interval
	}
	return max(*last+interval, now)
}
func (m *Manager) Reschedule() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return domain.NewError("cancelled", "backup manager is closed")
	}
	m.wg.Add(1)
	m.mu.Unlock()
	defer m.wg.Done()
	config, e := m.store.StoredWebDAVConfig()
	if e != nil {
		return e
	}
	sync, e := m.store.GetWebDAVSyncStatus()
	if e != nil {
		return e
	}
	next := int64(0)
	if config != nil && config.View.AutoSyncEnabled {
		if e = ValidateInterval(config.View.SyncIntervalMinutes); e != nil {
			return e
		}
		next = NextScheduledAt(sync.LastAttemptAt, time.Now().UnixMilli(), int64(config.View.SyncIntervalMinutes)*60000)
	}
	m.mu.Lock()
	if !m.closed {
		m.next = next
	}
	m.mu.Unlock()
	select {
	case m.wake <- struct{}{}:
	default:
	}
	return nil
}
func (m *Manager) scheduler() {
	defer m.wg.Done()
	for {
		m.mu.Lock()
		next := m.next
		m.mu.Unlock()
		var timer *time.Timer
		var wait <-chan time.Time
		if next > 0 {
			timer = time.NewTimer(time.Until(time.UnixMilli(next)))
			wait = timer.C
		}
		select {
		case <-m.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-m.wake:
			if timer != nil {
				timer.Stop()
			}
			continue
		case <-wait:
			_, e := m.Backup(m.ctx, "webdav", "scheduled")
			if e != nil && !errors.Is(e, context.Canceled) {
				m.reportSchedulerError(e)
			}
		}
	}
}
func (m *Manager) reportSchedulerError(e error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	message := e.Error()
	_, recordErr := m.store.RecordWebDAVSyncStatus("failed", &message, false)
	if recordErr != nil {
		m.mu.Lock()
		m.next = 0
		m.runtimeError = errors.Join(e, recordErr)
		m.mu.Unlock()
	}
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if m.onChange != nil && !closed {
		m.onChange()
	}
}

func (m *Manager) Overview() (domain.BackupOverview, error) {
	var v domain.BackupOverview
	m.mu.Lock()
	runtimeErr := m.runtimeError
	m.mu.Unlock()
	if runtimeErr != nil {
		return v, runtimeErr
	}
	local, e := m.backups.List()
	if e != nil {
		return v, e
	}
	if len(local) > 0 {
		v.LatestLocalBackupAt = &local[0].CreatedAt
	}
	config, e := m.store.StoredWebDAVConfig()
	if e != nil {
		return v, e
	}
	v.WebdavConfigured = config != nil
	if config != nil {
		v.WebdavAutoSyncEnabled = config.View.AutoSyncEnabled
	}
	v.WebdavSync, e = m.store.GetWebDAVSyncStatus()
	if e != nil {
		return v, e
	}
	m.mu.Lock()
	if m.next > 0 {
		next := m.next
		v.NextScheduledAt = &next
	}
	m.mu.Unlock()
	return v, nil
}
