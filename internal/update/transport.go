package update

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"time"

	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
)

// MyGo uses http.DefaultClient; install this stable delegate once before any checks.
var updaterHTTP = &settingsTransport{}
var installHTTP sync.Once

type settingsTransport struct {
	mu        sync.Mutex
	settings  func() (domain.AppSettings, error)
	previous  domain.AppSettings
	transport http.RoundTripper
}

func configureHTTP(get func() (domain.AppSettings, error)) error {
	updaterHTTP.mu.Lock()
	if updaterHTTP.settings != nil {
		updaterHTTP.mu.Unlock()
		return errors.New("update manager already owns HTTP transport")
	}
	updaterHTTP.settings = get
	updaterHTTP.transport = nil
	updaterHTTP.mu.Unlock()
	installHTTP.Do(func() { http.DefaultClient = &http.Client{Transport: updaterHTTP, CheckRedirect: redirect} })
	return nil
}
func safeURL(u *url.URL) error {
	if u == nil || u.User != nil || u.Fragment != "" {
		return errors.New("invalid update URL")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback())) {
		return errors.New("update URL requires HTTPS")
	}
	if u.Hostname() == "" {
		return errors.New("update URL requires host")
	}
	return nil
}
func redirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many update redirects")
	}
	if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
		return errors.New("update redirect downgraded HTTPS")
	}
	return safeURL(req.URL)
}
func (t *settingsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := safeURL(req.URL); err != nil {
		return nil, err
	}
	t.mu.Lock()
	if t.settings == nil {
		t.mu.Unlock()
		return nil, errors.New("update transport is closed")
	}
	settings, err := t.settings()
	if err != nil {
		t.mu.Unlock()
		return nil, err
	}
	if t.transport == nil || !reflect.DeepEqual(settings, t.previous) {
		client, e := netx.NewClient(settings, 30*time.Second)
		if e != nil {
			t.mu.Unlock()
			return nil, e
		}
		if old, ok := t.transport.(interface{ CloseIdleConnections() }); ok {
			old.CloseIdleConnections()
		}
		t.transport = client.Transport
		t.previous = settings
	}
	transport := t.transport
	t.mu.Unlock()
	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	response, err := transport.RoundTrip(req.Clone(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	response.Body = &cancelBody{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error { defer b.cancel(); return b.ReadCloser.Close() }

func closeHTTP() {
	updaterHTTP.mu.Lock()
	defer updaterHTTP.mu.Unlock()
	updaterHTTP.settings = nil
	if old, ok := updaterHTTP.transport.(interface{ CloseIdleConnections() }); ok {
		old.CloseIdleConnections()
	}
	updaterHTTP.transport = nil
}
