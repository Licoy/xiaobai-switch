package webdav

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/appbackup"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func (c *Client) List(ctx context.Context) ([]domain.RemoteBackupInfo, error) {
	if e := c.Check(ctx); e != nil {
		return nil, e
	}
	req, e := c.request(ctx, "PROPFIND", c.directory, strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:prop><D:getcontentlength/><D:getlastmodified/><D:resourcetype/></D:prop></D:propfind>`))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	resp, e := c.send(req)
	if e != nil {
		return nil, e
	}
	raw, e := netx.ReadBody(resp, 4*1024*1024)
	if e != nil {
		return nil, netx.NetworkError(e, c.config.Password)
	}
	if resp.StatusCode != 207 && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return nil, statusError("WebDAV list", resp.StatusCode)
	}
	return c.parseListing(raw)
}

type davProp struct {
	Length   string `xml:"getcontentlength"`
	Modified string `xml:"getlastmodified"`
	Resource struct {
		Collection *struct{} `xml:"collection"`
	} `xml:"resourcetype"`
}
type davResponse struct {
	Href  string `xml:"href"`
	Props []struct {
		Prop   davProp `xml:"prop"`
		Status string  `xml:"status"`
	} `xml:"propstat"`
}

func (c *Client) parseListing(raw []byte) ([]domain.RemoteBackupInfo, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	depth := 0
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, domain.NewError("invalid_response", "invalid WebDAV XML")
		}
		switch v := token.(type) {
		case xml.StartElement:
			depth++
			if depth > 64 {
				return nil, domain.NewError("invalid_response", "WebDAV XML nesting exceeds limit")
			}
		case xml.EndElement:
			depth--
		case xml.Directive:
			if strings.Contains(strings.ToUpper(string(v)), "DOCTYPE") {
				return nil, domain.NewError("invalid_response", "WebDAV XML document types are not allowed")
			}
		}
	}
	var listing struct {
		XMLName   xml.Name      `xml:"multistatus"`
		Responses []davResponse `xml:"response"`
	}
	if e := xml.Unmarshal(raw, &listing); e != nil {
		return nil, domain.NewError("invalid_response", "invalid WebDAV multi-status")
	}
	out := []domain.RemoteBackupInfo{}
	seen := map[string]bool{}
	for _, r := range listing.Responses {
		href, e := url.Parse(strings.TrimSpace(r.Href))
		if e != nil {
			return nil, domain.NewError("invalid_response", "invalid WebDAV response href")
		}
		u := c.directory.ResolveReference(href)
		if u.Scheme != c.directory.Scheme || u.Host != c.directory.Host || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(u.Path, "\\\x00") {
			return nil, domain.NewError("invalid_response", "WebDAV href changed origin or contains unsafe data")
		}
		if strings.TrimRight(u.Path, "/") == strings.TrimRight(c.directory.Path, "/") {
			continue
		}
		name := filepath.Base(u.Path)
		if !validName(name) {
			continue
		}
		if strings.TrimRight(filepath.ToSlash(filepath.Dir(u.Path)), "/") != strings.TrimRight(c.directory.Path, "/") {
			return nil, domain.NewError("invalid_response", "WebDAV backup href escaped its directory")
		}
		if seen[name] {
			return nil, domain.NewError("invalid_response", "duplicate WebDAV backup href")
		}
		seen[name] = true
		info := domain.RemoteBackupInfo{FileName: name, DeviceName: appbackup.ParseDeviceFromFilename(name)}
		collection := false
		for _, p := range r.Props {
			if p.Status != "" && !strings.Contains(p.Status, " 200 ") {
				continue
			}
			collection = collection || p.Prop.Resource.Collection != nil
			if p.Prop.Length != "" {
				size, e := strconv.ParseUint(strings.TrimSpace(p.Prop.Length), 10, 64)
				if e != nil {
					return nil, domain.NewError("invalid_response", "invalid WebDAV content length")
				}
				info.Size = size
			}
			if p.Prop.Modified != "" {
				info.LastModified = p.Prop.Modified
			}
		}
		if !collection {
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FileName > out[j].FileName })
	return out, nil
}
func (c *Client) Download(ctx context.Context, name, destination string) (err error) {
	if !validName(name) {
		return domain.NewError("validation_failed", "invalid remote backup name")
	}
	destination, e := paths.ValidatePath(destination)
	if e != nil {
		return e
	}
	req, e := c.request(ctx, "GET", appendSegment(c.directory, name, false), nil)
	if e != nil {
		return e
	}
	resp, e := c.send(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError("WebDAV download", resp.StatusCode)
	}
	if resp.ContentLength > appbackup.MaxArchiveBytes {
		return domain.NewError("backup_invalid", "remote backup exceeds size limit")
	}
	out, e := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.Remove(destination))
		}
	}()
	if e = fileops.Secure(destination); e != nil {
		out.Close()
		return e
	}
	n, e := io.Copy(out, io.LimitReader(resp.Body, appbackup.MaxArchiveBytes+1))
	err = errors.Join(e, out.Sync(), out.Close())
	if err != nil {
		return netx.NetworkError(err, c.config.Password)
	}
	if n > appbackup.MaxArchiveBytes {
		return domain.NewError("backup_invalid", "remote backup exceeds size limit")
	}
	return nil
}
func (c *Client) Cleanup(ctx context.Context, maxCopies uint32) (int, error) {
	if e := ValidateRetention(maxCopies); e != nil {
		return 0, e
	}
	backups, e := c.List(ctx)
	if e != nil {
		return 0, e
	}
	own := []domain.RemoteBackupInfo{}
	for _, b := range backups {
		if b.DeviceName == appbackup.DeviceName() {
			own = append(own, b)
		}
	}
	removed := 0
	for _, b := range own[min(int(maxCopies), len(own)):] {
		if e = c.Delete(ctx, b.FileName); e != nil {
			return removed, e
		}
		removed++
	}
	return removed, nil
}
