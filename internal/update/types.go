package update

import (
	"context"
	"sync"
	"time"

	"github.com/licoy/xiaobai-switch/internal/domain"
)

type Options struct {
	Settings func() (domain.AppSettings, error)
	// OnChange runs outside locks; queue UI work and return, never call Close synchronously.
	OnChange func(State)
	Disabled bool
}
type State struct {
	Enabled    bool      `json:"enabled"`
	Checking   bool      `json:"checking"`
	Installing bool      `json:"installing"`
	Available  bool      `json:"available"`
	Installed  bool      `json:"installed"`
	Version    string    `json:"version"`
	Notes      string    `json:"notes"`
	Date       time.Time `json:"date"`
	Downloaded int64     `json:"downloaded"`
	Total      int64     `json:"total"`
	Error      *string   `json:"error"`
}
type candidate struct {
	version, notes string
	date           time.Time
	install        func(context.Context, func(int64, int64)) error
}
type backend struct {
	enabled func() bool
	check   func(context.Context) (*candidate, error)
}
type Manager struct {
	mu              sync.Mutex
	state           State
	opts            Options
	backend         backend
	ctx             context.Context
	cancel          context.CancelFunc
	operationCancel context.CancelFunc
	wake            chan struct{}
	workers         sync.WaitGroup
	closeOnce       sync.Once
	closed          bool
	candidate       *candidate
	httpInstalled   bool
}
