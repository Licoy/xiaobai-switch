package update

import (
	"context"
	"errors"
	"time"
)

func (m *Manager) schedule() {
	defer m.workers.Done()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	checkDue, wasEnabled := true, false
	for {
		settings, err := m.opts.Settings()
		enabled := err == nil && settings.AutoCheckUpdate && m.Snapshot().Enabled
		if enabled && !wasEnabled {
			checkDue = true
			ticker.Reset(time.Hour)
		}
		wasEnabled = enabled
		if err != nil {
			m.mu.Lock()
			message := cleanError(err)
			m.state.Error = &message
			m.mu.Unlock()
			m.notify()
		}
		if enabled && checkDue {
			// Check exposes the error in State for both foreground and background callers.
			snapshot := m.Snapshot()
			if !snapshot.Checking && !snapshot.Installing && !snapshot.Installed {
				if checkErr := m.Check(m.ctx); checkErr != nil && !errors.Is(checkErr, context.Canceled) && m.ctx.Err() == nil {
					m.mu.Lock()
					message := cleanError(checkErr)
					m.state.Error = &message
					m.mu.Unlock()
					m.notify()
				}
			}
		}
		select {
		case <-m.ctx.Done():
			return
		case <-m.wake:
			checkDue = false
		case <-ticker.C:
			checkDue = true
		}
	}
}
