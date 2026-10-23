package update

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func testOptions() Options {
	return Options{Settings: func() (domain.AppSettings, error) {
		s := domain.DefaultSettings()
		s.AutoCheckUpdate = false
		return s, nil
	}}
}
func TestCheckInstallAndCancellation(t *testing.T) {
	var installs atomic.Int32
	b := backend{enabled: func() bool { return true }, check: func(context.Context) (*candidate, error) {
		return &candidate{version: "1.2.0", install: func(ctx context.Context, p func(int64, int64)) error { installs.Add(1); p(2, 4); return nil }}, nil
	}}
	m := newManager(testOptions(), b)
	defer m.Close()
	if err := m.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !m.Snapshot().Available {
		t.Fatal("missing available update")
	}
	if err := m.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !m.Snapshot().Installed || installs.Load() != 1 || m.Snapshot().Total != 4 {
		t.Fatal(m.Snapshot())
	}
	b.check = func(ctx context.Context) (*candidate, error) { <-ctx.Done(); return nil, ctx.Err() }
	m.backend = b
	done := make(chan error, 1)
	go func() { done <- m.Check(context.Background()) }()
	for !m.Snapshot().Checking {
		time.Sleep(time.Millisecond)
	}
	if e := m.Check(context.Background()); e == nil {
		t.Fatal("concurrent check accepted")
	}
	m.Cancel()
	if e := <-done; e == nil {
		t.Fatal("cancel hidden")
	}
	if m.Snapshot().Error == nil {
		t.Fatal("background error invisible")
	}
}
func TestDisabledAndErrors(t *testing.T) {
	opts := testOptions()
	opts.Disabled = true
	m := newManager(opts, backend{enabled: func() bool { return true }, check: func(context.Context) (*candidate, error) { t.Fatal("disabled check ran"); return nil, nil }})
	if e := m.Check(context.Background()); e == nil {
		t.Fatal("disabled succeeded")
	}
	m.Close()
	if e := m.Install(context.Background()); e == nil {
		t.Fatal("closed succeeded")
	}
	secret := errors.New("proxy https://user:password@proxy.test failed")
	if got := cleanError(&url.Error{Op: "Get", URL: "https://token@x", Err: secret}); got != "Get: update request failed" {
		t.Fatal(got)
	}
}
func TestSafeURLAndRedirect(t *testing.T) {
	for _, raw := range []string{"http://example.com/feed", "file:///tmp/a", "https://user:key@x/a", "https://x/a#b"} {
		u, _ := url.Parse(raw)
		if safeURL(u) == nil {
			t.Fatal(raw)
		}
	}
	for _, raw := range []string{"https://example.com/feed", "http://127.0.0.1/feed", "http://[::1]/feed"} {
		u, _ := url.Parse(raw)
		if e := safeURL(u); e != nil {
			t.Fatal(e)
		}
	}
	first, _ := http.NewRequest("GET", "https://example.test", nil)
	next, _ := http.NewRequest("GET", "http://127.0.0.1", nil)
	if redirect(next, []*http.Request{first}) == nil {
		t.Fatal("HTTPS downgrade accepted")
	}
}
func TestDevHomeNeverEnables(t *testing.T) {
	t.Setenv("XIAOBAI_DEV_HOME", t.TempDir())
	m, e := New(testOptions())
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if m.Snapshot().Enabled {
		t.Fatal("dev updater enabled")
	}
}

func TestChecksAreBoundedAndCloseCancelsInstall(t *testing.T) {
	entered := make(chan struct{})
	m := newManager(testOptions(), backend{enabled: func() bool { return true }, check: func(ctx context.Context) (*candidate, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 15*time.Second {
			t.Error("check has no 15 second bound")
		}
		return &candidate{version: "1.0.1", install: func(ctx context.Context, p func(int64, int64)) error { close(entered); <-ctx.Done(); return ctx.Err() }}, nil
	}})
	if e := m.Check(context.Background()); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- m.Install(context.Background()) }()
	<-entered
	if e := m.Close(); e != nil {
		t.Fatal(e)
	}
	if e := <-done; e == nil {
		t.Fatal("close hid cancelled install")
	}
	if m.Snapshot().Installed {
		t.Fatal("failed upgrade marked installed")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestTransportBoundsRequestAndObservesCancellation(t *testing.T) {
	settings := domain.DefaultSettings()
	settings.ProxyMode = "none"
	transport := &settingsTransport{settings: func() (domain.AppSettings, error) { return settings, nil }, previous: settings, transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		deadline, ok := req.Context().Deadline()
		if !ok || time.Until(deadline) > 30*time.Second {
			t.Error("transport dropped client timeout")
		}
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://example.test", nil)
	if _, e := transport.RoundTrip(req); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
}
