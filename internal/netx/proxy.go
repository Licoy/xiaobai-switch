package netx

import (
	"context"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

type ResolvedProxy struct {
	Disabled bool
	URL      string
}

func CustomProxyURL(protocol, host string, port uint16) (string, error) {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	host = strings.TrimSpace(host)
	if protocol != "http" && protocol != "https" && protocol != "socks5" {
		return "", domain.NewError("validation_failed", "unsupported proxy protocol")
	}
	if host == "" || strings.ContainsAny(host, " /\\\t\r\n@?#") || port == 0 {
		return "", domain.NewError("validation_failed", "proxy host and port are required")
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return "", domain.NewError("validation_failed", "proxy host must not include a port")
	}
	return protocol + "://" + net.JoinHostPort(host, fmt.Sprint(port)), nil
}
func DetectEnvProxy() string {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}
func detectSystemProxy() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	platform, e := platformProxy(ctx)
	if e != nil {
		return "", e
	}
	return chooseSystemProxy(platform, DetectEnvProxy()), nil
}
func chooseSystemProxy(platform, environment string) string {
	if platform != "" {
		return platform
	}
	return environment
}
func ResolveProxy(settings domain.AppSettings) (ResolvedProxy, error) {
	switch settings.ProxyMode {
	case "none":
		return ResolvedProxy{Disabled: true}, nil
	case "custom":
		var port uint16
		host := ""
		if settings.ProxyPort != nil {
			port = *settings.ProxyPort
		}
		if settings.ProxyHost != nil {
			host = *settings.ProxyHost
		}
		v, e := CustomProxyURL(settings.ProxyProtocol, host, port)
		return ResolvedProxy{URL: v}, e
	case "system", "":
		v, e := detectSystemProxy()
		return ResolvedProxy{URL: v}, e
	default:
		return ResolvedProxy{}, domain.NewError("validation_failed", "unknown proxy mode")
	}
}
func parseProxy(raw string) (*url.URL, error) {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" && u.Scheme != "socks5h") {
		return nil, domain.NewError("validation_failed", "invalid proxy URL")
	}
	return u, nil
}
func ParseScutilProxy(text string) string {
	values := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		key, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(v)
		}
	}
	for _, kind := range []string{"HTTPS", "HTTP", "SOCKS"} {
		if values[kind+"Enable"] != "1" || values[kind+"Proxy"] == "" || values[kind+"Port"] == "" {
			continue
		}
		scheme := "http"
		if kind == "SOCKS" {
			scheme = "socks5"
		}
		return scheme + "://" + net.JoinHostPort(values[kind+"Proxy"], values[kind+"Port"])
	}
	return ""
}
func ParseWindowsProxyServer(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	withScheme := func(scheme, v string) string {
		if strings.Contains(v, "://") {
			return v
		}
		return scheme + "://" + v
	}
	if !strings.Contains(raw, "=") {
		return withScheme("http", raw)
	}
	m := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		key, v, ok := strings.Cut(part, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		v = strings.TrimSpace(v)
		if ok && v != "" && m[key] == "" {
			m[key] = v
		}
	}
	for _, key := range []string{"https", "http", "socks", "socks5"} {
		if m[key] != "" {
			scheme := "http"
			if strings.HasPrefix(key, "socks") {
				scheme = "socks5"
			}
			return withScheme(scheme, m[key])
		}
	}
	return ""
}
