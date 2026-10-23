package webdav

import (
	"context"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/appbackup"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type RuntimeConfig struct {
	BaseURL, Username, Password, RemotePath string
	AcceptInvalidCerts                      bool
}
type Client struct {
	http            *http.Client
	config          RuntimeConfig
	root, directory *url.URL
}

func NewClient(config RuntimeConfig, settings domain.AppSettings) (*Client, error) {
	client, e := netx.NewClientWithTLS(settings, 300*time.Second, config.AcceptInvalidCerts)
	if e != nil {
		return nil, e
	}
	return NewClientWithHTTP(config, client)
}
func NewClientWithHTTP(config RuntimeConfig, client *http.Client) (*Client, error) {
	root, dir, e := ValidateURLs(config)
	if e != nil {
		return nil, e
	}
	if client == nil {
		return nil, domain.NewError("validation_failed", "WebDAV HTTP client is required")
	}
	copy := *client
	previous := client.CheckRedirect
	copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != root.Scheme || req.URL.Host != root.Host {
			return domain.NewError("webdav_auth_failed", "WebDAV redirect changed origin")
		}
		if len(via) >= 5 {
			return domain.NewError("network", "too many WebDAV redirects")
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	return &Client{http: &copy, config: config, root: root, directory: dir}, nil
}
func ValidateURLs(config RuntimeConfig) (*url.URL, *url.URL, error) {
	u, e := url.Parse(strings.TrimSpace(config.BaseURL))
	if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.TrimSpace(config.Username) == "" || strings.ContainsAny(config.BaseURL, "\r\n\\") {
		return nil, nil, domain.NewError("validation_failed", "valid WebDAV URL without embedded credentials and a username are required")
	}
	segments, e := remoteSegments(config.RemotePath)
	if e != nil {
		return nil, nil, e
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	if strings.Contains(u.Path, "/../") || strings.Contains(u.Path, "/./") {
		return nil, nil, domain.NewError("validation_failed", "WebDAV base path is invalid")
	}
	root := *u
	for _, s := range segments {
		u = appendSegment(u, s, true)
	}
	return &root, u, nil
}
func remoteSegments(raw string) ([]string, error) {
	out := []string{}
	for _, s := range strings.Split(strings.Trim(strings.TrimSpace(raw), "/"), "/") {
		if s == "" {
			continue
		}
		if s == "." || s == ".." || strings.ContainsAny(s, "\\%\x00\r\n") {
			return nil, domain.NewError("validation_failed", "invalid WebDAV remote path")
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, domain.NewError("validation_failed", "WebDAV remote path is required")
	}
	return out, nil
}
func appendSegment(base *url.URL, segment string, directory bool) *url.URL {
	u := *base
	u.Path = strings.TrimRight(u.Path, "/") + "/" + segment
	u.RawPath = ""
	if directory {
		u.Path += "/"
	}
	return &u
}
func validName(name string) bool {
	return strings.HasPrefix(name, appbackup.Prefix) && strings.HasSuffix(name, appbackup.Suffix) && path.Base(name) == name && !strings.Contains(name, "..") && !strings.ContainsAny(name, "/\\%\x00\r\n")
}
func ValidateInterval(n uint32) error {
	switch n {
	case 15, 30, 60, 120, 360, 720, 1440:
		return nil
	}
	return domain.NewError("validation_failed", "unsupported WebDAV sync interval")
}
func ValidateRetention(n uint32) error {
	if n < 1 || n > 100 {
		return domain.NewError("validation_failed", "WebDAV retention must be between 1 and 100")
	}
	return nil
}
func (c *Client) request(ctx context.Context, method string, u *url.URL, body io.Reader) (*http.Request, error) {
	req, e := http.NewRequestWithContext(ctx, method, u.String(), body)
	if e != nil {
		return nil, e
	}
	req.SetBasicAuth(c.config.Username, c.config.Password)
	return req, nil
}
func (c *Client) send(req *http.Request) (*http.Response, error) {
	resp, e := c.http.Do(req)
	if e != nil {
		return nil, netx.NetworkError(e, c.config.Password)
	}
	return resp, nil
}
func statusError(action string, status int) error {
	if status == 401 || status == 403 {
		return domain.NewError("webdav_auth_failed", "WebDAV authentication failed")
	}
	return domain.NewError("network", fmt.Sprintf("%s failed with HTTP %d", action, status))
}
func (c *Client) propfindStatus(ctx context.Context, u *url.URL) (int, error) {
	req, e := c.request(ctx, "PROPFIND", u, nil)
	if e != nil {
		return 0, e
	}
	req.Header.Set("Depth", "0")
	resp, e := c.send(req)
	if e != nil {
		return 0, e
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}
func (c *Client) Check(ctx context.Context) error {
	status, e := c.propfindStatus(ctx, c.directory)
	if e != nil {
		return e
	}
	if status == 200 || status == 207 {
		return nil
	}
	if status != 404 {
		return statusError("WebDAV connection check", status)
	}
	current := c.root
	segments, e := remoteSegments(c.config.RemotePath)
	if e != nil {
		return e
	}
	for _, s := range segments {
		current = appendSegment(current, s, true)
		req, e := c.request(ctx, "MKCOL", current, nil)
		if e != nil {
			return e
		}
		resp, e := c.send(req)
		if e != nil {
			return e
		}
		status = resp.StatusCode
		resp.Body.Close()
		switch status {
		case 200, 201, 204:
		case 400, 405, 409:
			verified, e := c.propfindStatus(ctx, current)
			if e != nil {
				return e
			}
			if verified != 200 && verified != 207 {
				return statusError("WebDAV directory verification", verified)
			}
		default:
			return statusError("WebDAV directory creation", status)
		}
	}
	return nil
}
func (c *Client) Upload(ctx context.Context, name, localPath string) error {
	if !validName(name) {
		return domain.NewError("validation_failed", "invalid remote backup name")
	}
	if e := c.Check(ctx); e != nil {
		return e
	}
	info, e := os.Lstat(localPath)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Size() > appbackup.MaxArchiveBytes {
		return domain.NewError("backup_invalid", "upload source is invalid or too large")
	}
	f, e := os.Open(localPath)
	if e != nil {
		return e
	}
	defer f.Close()
	req, e := c.request(ctx, "PUT", appendSegment(c.directory, name, false), f)
	if e != nil {
		return e
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/zip")
	resp, e := c.send(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200, 201, 204:
		return nil
	}
	return statusError("WebDAV upload", resp.StatusCode)
}
func (c *Client) Delete(ctx context.Context, name string) error {
	if !validName(name) {
		return domain.NewError("validation_failed", "invalid remote backup name")
	}
	req, e := c.request(ctx, "DELETE", appendSegment(c.directory, name, false), nil)
	if e != nil {
		return e
	}
	resp, e := c.send(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200, 204, 404:
		return nil
	}
	return statusError("WebDAV delete", resp.StatusCode)
}
