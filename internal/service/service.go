package service

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/clidetect"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"github.com/licoy/xiaobai-switch/internal/store"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func New(st *store.Store, opts Options) (*Service, error) {
	if st == nil {
		return nil, domain.NewError("validation_failed", "store is required")
	}
	var e error
	if opts.Home == "" {
		opts.Home, e = paths.HomeDir()
		if e != nil {
			return nil, e
		}
	}
	if opts.Root == "" {
		opts.Root, e = paths.AppDir()
		if e != nil {
			return nil, e
		}
	}
	if opts.Home, e = paths.ValidatePath(opts.Home); e != nil {
		return nil, e
	}
	if opts.Root, e = paths.ValidatePath(opts.Root); e != nil {
		return nil, e
	}
	if opts.Platform == "" {
		opts.Platform = runtime.GOOS
	}
	s := &Service{st: st, options: opts, pending: map[string]pending{}, writeJournal: saveServiceJournal}
	for _, name := range []string{"backups", "env", "locks"} {
		p, e := s.path(name)
		if e != nil {
			return nil, e
		}
		if e = os.MkdirAll(p, 0700); e != nil {
			return nil, e
		}
	}
	if e = s.recoverPending(); e != nil {
		return nil, e
	}
	env := clidetect.ProbeEnv{}
	if opts.CLIEnv != nil {
		env = *opts.CLIEnv
	} else if os.Getenv("XIAOBAI_DEV_HOME") == "" {
		actual, e := paths.HomeDir()
		if e != nil {
			return nil, e
		}
		if actual == opts.Home {
			env, e = clidetect.FromProcess()
			if e != nil {
				return nil, e
			}
		}
	}
	s.detector = clidetect.NewDetector(env, time.Minute)
	s.detector.SetOnChange(s.changed)
	return s, nil
}
func (s *Service) path(parts ...string) (string, error) {
	return paths.ValidatePath(filepath.Join(append([]string{s.options.Root}, parts...)...))
}
func (s *Service) changed() {
	if s.options.OnChange != nil {
		s.options.OnChange()
	}
}
func checkContext(ctx context.Context) error {
	if ctx == nil {
		return domain.NewError("validation_failed", "context is required")
	}
	return ctx.Err()
}
func publicError(err error, keys ...string) error {
	if err == nil {
		return nil
	}
	var app *domain.AppError
	code := "operation_failed"
	message := err.Error()
	var details *string
	if errors.As(err, &app) {
		code = app.Code
		message = app.Message
		if app.Details != nil {
			d := *app.Details
			details = &d
		}
	}
	for _, key := range keys {
		message = netx.Sanitize(message, key)
		if details != nil {
			d := netx.Sanitize(*details, key)
			details = &d
		}
	}
	if app == nil {
		for _, known := range []string{"validation_failed", "invalid_config", "invalid_binding", "config_changed", "lock_busy", "not_found", "backup_failed", "rollback failed"} {
			if strings.Contains(message, known) {
				code = strings.ReplaceAll(known, " ", "_")
				break
			}
		}
	}
	return &domain.AppError{Code: code, Message: message, Details: details}
}

func (s *Service) Close() error {
	if s.detector != nil {
		return s.detector.Close()
	}
	return nil
}
