package netx

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/buildinfo"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/secure"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var UserAgent = buildinfo.UserAgent

func NewClient(settings domain.AppSettings, timeout time.Duration) (*http.Client, error) {
	return NewClientWithTLS(settings, timeout, false)
}
func NewClientWithTLS(settings domain.AppSettings, timeout time.Duration, acceptInvalidCerts bool) (*http.Client, error) {
	if timeout <= 0 {
		return nil, domain.NewError("validation_failed", "positive HTTP timeout required")
	}
	resolved, e := ResolveProxy(settings)
	if e != nil {
		return nil, e
	}
	transport := &http.Transport{ForceAttemptHTTP2: true, MaxIdleConns: 100, IdleConnTimeout: 90 * time.Second, ExpectContinueTimeout: time.Second}
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = timeout
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: acceptInvalidCerts}
	// Legacy explicit Proxy::all ignores NO_PROXY; custom never bypasses or falls back
	if resolved.URL != "" {
		proxy, e := parseProxy(resolved.URL)
		if e != nil {
			return nil, e
		}
		transport.Proxy = http.ProxyURL(proxy)
	}
	return &http.Client{Transport: userAgentTransport{transport}, Timeout: timeout, CheckRedirect: checkRedirect}, nil
}

type userAgentTransport struct{ base http.RoundTripper }

func (t userAgentTransport) CloseIdleConnections() {
	if closer, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func (t userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	if cloned.Header.Get("User-Agent") == "" {
		cloned.Header.Set("User-Agent", UserAgent)
	}
	return t.base.RoundTrip(cloned)
}
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return domain.NewError("network", "too many redirects")
	}
	first := via[0]
	if (first.Header.Get("Authorization") != "" || first.Header.Get("x-api-key") != "") && (req.URL.Scheme != first.URL.Scheme || req.URL.Host != first.URL.Host) {
		return domain.NewError("network", "authenticated redirect changed origin")
	}
	return nil
}
func Sanitize(raw, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return raw
	}
	return strings.ReplaceAll(raw, key, secure.KeyPrefix(key))
}
func NetworkError(err error, key string) *domain.AppError {
	var app *domain.AppError
	if errors.As(err, &app) {
		return &domain.AppError{Code: app.Code, Message: Sanitize(app.Message, key)}
	}
	if errors.Is(err, context.Canceled) {
		return domain.NewError("cancelled", "request cancelled")
	}
	var n net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &n) && n.Timeout()) {
		return domain.NewError("timeout", "request timed out")
	}
	var cert *tls.CertificateVerificationError
	var unknown x509.UnknownAuthorityError
	if errors.As(err, &cert) || errors.As(err, &unknown) {
		return domain.NewError("ssl", "TLS certificate verification failed")
	}
	return domain.NewError("network", Sanitize(err.Error(), key))
}
func StatusError(status int, body []byte, key string) *domain.AppError {
	code := "network"
	switch status {
	case 401, 403:
		code = "unauthorized"
	case 404:
		code = "not_found"
	case 408, 504:
		code = "timeout"
	}
	message := fmt.Sprintf("HTTP %d", status)
	if m := ErrorMessage(body); m != "" {
		message = m
	}
	return domain.NewError(code, Sanitize(message, key))
}
func ReadBody(resp *http.Response, maxBytes int64) ([]byte, error) {
	if maxBytes < 1 {
		return nil, domain.NewError("internal", "invalid response size limit")
	}
	defer resp.Body.Close()
	body, e := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(body)) > maxBytes {
		return nil, domain.NewError("response_too_large", "HTTP response exceeds size limit")
	}
	return body, nil
}
