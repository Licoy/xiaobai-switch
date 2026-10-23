package clidetect

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"maps"
	"sync"
	"time"
)

type State struct {
	Tools      []domain.CliToolInfo         `json:"tools"`
	Refreshing bool                         `json:"refreshing"`
	UpdatedAt  int64                        `json:"updatedAt"`
	Error      *string                      `json:"error"`
	Errors     map[domain.TargetKind]string `json:"errors"`
}
type Detector struct {
	onChange func()
	mu       sync.Mutex
	state    State
	env      ProbeEnv
	ttl      time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	closed   bool
	wg       sync.WaitGroup
}

func NewDetector(env ProbeEnv, ttl time.Duration) *Detector {
	if ttl <= 0 {
		ttl = time.Minute
	}
	ctx, cancel := context.WithCancel(context.Background())
	env.PathDirs = append([]string{}, env.PathDirs...)
	env.ExtraDirs = append([]string{}, env.ExtraDirs...)
	return &Detector{env: env, ttl: ttl, ctx: ctx, cancel: cancel, state: State{Tools: []domain.CliToolInfo{}}}
}

// Snapshot returns immediately and performs expired version probes in a worker
func (d *Detector) Snapshot() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed && !d.state.Refreshing && (d.state.UpdatedAt == 0 || time.Since(time.UnixMilli(d.state.UpdatedAt)) >= d.ttl) {
		d.state.Refreshing = true
		d.wg.Add(1)
		go d.refresh()
	}
	return cloneState(d.state)
}

// Refresh explicitly bypasses the TTL; concurrent refresh requests share the worker
func (d *Detector) Refresh() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed && !d.state.Refreshing {
		d.state.Refreshing = true
		d.wg.Add(1)
		go d.refresh()
	}
	return cloneState(d.state)
}

func cloneState(s State) State {
	s.Errors = maps.Clone(s.Errors)
	s.Tools = append([]domain.CliToolInfo{}, s.Tools...)
	for i := range s.Tools {
		if p := s.Tools[i].Path; p != nil {
			v := *p
			s.Tools[i].Path = &v
		}
		if p := s.Tools[i].Version; p != nil {
			v := *p
			s.Tools[i].Version = &v
		}
	}
	if s.Error != nil {
		value := *s.Error
		s.Error = &value
	}
	return s
}

// SetOnChange runs notifications without locks after publishing the cache
// Already dispatched callbacks may run after Close; consumers must check their lifecycle
// Callbacks may call Close, which waits for workers rather than callback completion
func (d *Detector) SetOnChange(callback func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.onChange = callback
	}
}
func (d *Detector) refresh() {
	tools, targetErrors, e := detect(d.ctx, d.env)
	d.mu.Lock()
	d.state = State{Tools: tools, Errors: targetErrors, UpdatedAt: time.Now().UnixMilli()}
	if e != nil {
		message := e.Error()
		d.state.Error = &message
	}
	d.mu.Unlock()
	// Complete worker accounting before notification so callbacks may call Close
	d.wg.Done()
	d.mu.Lock()
	callback := d.onChange
	closed := d.closed
	d.mu.Unlock()
	if callback != nil && !closed {
		callback()
	}
}

func (d *Detector) Close() error {
	d.mu.Lock()
	d.closed = true
	d.onChange = nil
	d.cancel()
	d.mu.Unlock()
	d.wg.Wait()
	return nil
}
