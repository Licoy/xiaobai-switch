package netx

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/buildinfo"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func direct(t *testing.T) *http.Client {
	t.Helper()
	s := domain.DefaultSettings()
	s.ProxyMode = "none"
	c, e := NewClient(s, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestNormalizeURLsPreservesProtocolPaths(t *testing.T) {
	for _, tt := range []struct{ raw, claude, codex string }{{"https://api.test", "https://api.test", "https://api.test/v1"}, {"https://api.test/v1/", "https://api.test/v1", "https://api.test/v1"}, {"https://api.test/private/v1/messages?query=1#part", "https://api.test/private/v1", "https://api.test/private/v1"}, {"https://api.test/anthropic/messages", "https://api.test/anthropic", "https://api.test/anthropic/v1"}} {
		p, e := NormalizeURL(tt.raw)
		if e != nil || p.ClaudeBaseURL != tt.claude || p.CodexBaseURL != tt.codex || p.ModelsURL != tt.codex+"/models" {
			t.Fatalf("normalize %s %+v %v", tt.raw, p, e)
		}
	}
	for _, raw := range []string{"https://user:secret@api.test", "ftp://api.test", "https://a.test bad", "https:///no-host"} {
		if _, e := NormalizeURL(raw); e == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	urls, e := NormalizeBaseURLs([]string{" https://a.test/ ", "", "https://a.test/", "https://b.test/private"})
	if e != nil || len(urls) != 2 || urls[0] != "https://a.test/" {
		t.Fatalf("routes %+v %v", urls, e)
	}
}
func TestProxyParsersAndPrecedence(t *testing.T) {
	text := "HTTPEnable : 1\nHTTPProxy : localhost\nHTTPPort : 7890\nHTTPSEnable : 1\nHTTPSProxy : localhost\nHTTPSPort : 7891"
	if p := ParseScutilProxy(text); p != "http://localhost:7891" {
		t.Fatal(p)
	}
	if p := ParseWindowsProxyServer("http=one:80;https=two:81;socks=three:82"); p != "http://two:81" {
		t.Fatal(p)
	}
	if p := ParseWindowsProxyServer("socks=localhost:1080"); p != "socks5://localhost:1080" {
		t.Fatal(p)
	}
	if got := chooseSystemProxy("http://system:80", "http://env:80"); got != "http://system:80" {
		t.Fatal(got)
	}
	if got := chooseSystemProxy("", "http://env:80"); got != "http://env:80" {
		t.Fatal(got)
	}
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(k, "")
	}
	t.Setenv("ALL_PROXY", "http://all:80")
	t.Setenv("HTTP_PROXY", "http://http:80")
	t.Setenv("HTTPS_PROXY", "http://https:80")
	if DetectEnvProxy() != "http://https:80" {
		t.Fatal("environment precedence")
	}
	if _, e := CustomProxyURL("http", "host:123", 80); e == nil {
		t.Fatal("accepted host port")
	}
	if p, e := CustomProxyURL("socks5", "::1", 1080); e != nil || p != "socks5://[::1]:1080" {
		t.Fatalf("IPv6 proxy %s %v", p, e)
	}
}
func TestExplicitProxyDoesNotBypassNOProxyOrFallback(t *testing.T) {
	var directHits, proxyHits atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { directHits.Add(1); io.WriteString(w, "direct") }))
	defer origin.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		if r.URL.Host == "" {
			t.Error("proxy request not absolute")
		}
		io.WriteString(w, "proxy")
	}))
	defer proxy.Close()
	u, _ := url.Parse(proxy.URL)
	port, _ := strconv.Atoi(u.Port())
	host := u.Hostname()
	p := uint16(port)
	s := domain.DefaultSettings()
	s.ProxyMode = "custom"
	s.ProxyHost = &host
	s.ProxyPort = &p
	t.Setenv("NO_PROXY", "*")
	c, e := NewClient(s, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	resp, e := c.Get(origin.URL)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if proxyHits.Load() != 1 || directHits.Load() != 0 {
		t.Fatal("custom proxy bypassed")
	}
	proxy.Close()
	if _, e = c.Get(origin.URL); e == nil {
		t.Fatal("dead custom proxy silently connected")
	}
	if directHits.Load() != 0 {
		t.Fatal("direct fallback occurred")
	}
	c = direct(t)
	resp, e = c.Get(origin.URL)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if directHits.Load() != 1 {
		t.Fatal("none mode did not connect directly")
	}
}
func TestBoundsCancellationAndAuthenticationRedirect(t *testing.T) {
	c := direct(t)
	large := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, strings.Repeat("x", 512*1024+1)) }))
	defer large.Close()
	if _, e := FetchText(context.Background(), c, large.URL); e == nil {
		t.Fatal("accepted excessive body")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := FetchText(ctx, c, large.URL); e == nil {
		t.Fatal("ignored cancellation")
	} else {
		var app *domain.AppError
		if !errors.As(e, &app) || app.Code != "cancelled" {
			t.Fatalf("cancellation %v", e)
		}
	}
	var leaked atomic.Int32
	end := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer end.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, end.URL, http.StatusFound) }))
	defer redirect.Close()
	req, _ := http.NewRequest("GET", redirect.URL, nil)
	req.Header.Set("x-api-key", "private-api-key")
	if _, e := c.Do(req); e == nil {
		t.Fatal("accepted authenticated cross-origin redirect")
	}
	if leaked.Load() != 0 {
		t.Fatal("credential request reached other origin")
	}
}
func TestUserAgentAndStatusMapping(t *testing.T) {
	c := direct(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != buildinfo.UserAgent {
			t.Error("incorrect user agent")
		}
		w.WriteHeader(403)
	}))
	defer srv.Close()
	r, e := ProbeURLs(context.Background(), c, []string{srv.URL})
	if e != nil || len(r) != 1 || !r[0].Ok || r[0].Status == nil || *r[0].Status != 403 {
		t.Fatalf("reachable status %+v %v", r, e)
	}
	err := StatusError(401, []byte(`{"error":{"message":"bad secret-api-key"}}`), "secret-api-key")
	if err.Code != "unauthorized" || strings.Contains(err.Message, "secret-api-key") {
		t.Fatalf("status leak %+v", err)
	}
}
