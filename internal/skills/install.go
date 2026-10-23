package skills

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const maxArchiveBytes int64 = 32 * 1024 * 1024

var safeComponent = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.-]*$`)

type installation struct {
	SourceRef   string  `json:"source_ref"`
	SkillName   *string `json:"skill_name,omitempty"`
	InstalledAt string  `json:"installed_at"`
}

func readInstallation(path string) (installation, error) {
	var v installation
	info, e := os.Lstat(path)
	if e != nil {
		return v, e
	}
	if !info.Mode().IsRegular() || info.Size() > 64*1024 {
		return v, validation("installation manifest must be a regular file within size limit")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return v, e
	}
	if e = json.Unmarshal(b, &v); e != nil {
		return v, e
	}
	if _, _, e = ParseSource(v.SourceRef); e != nil {
		return v, e
	}
	return v, nil
}
func ParseSource(raw string) (string, string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if strings.Contains(raw, "://") {
		u, e := url.Parse(raw)
		if e != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return "", "", validation("skill source must be an HTTPS GitHub repository")
		}
		raw = strings.Trim(u.Path, "/")
	}
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return "", "", validation("skill source must identify owner/repo")
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git")
	if !safeComponent.MatchString(owner) || !safeComponent.MatchString(repo) || owner == "." || owner == ".." || repo == "." || repo == ".." {
		return "", "", validation("invalid GitHub repository source")
	}
	return owner, repo, nil
}
func normalizeSource(raw string) string {
	owner, repo, e := ParseSource(raw)
	if e != nil {
		return ""
	}
	return strings.ToLower(owner + "/" + repo)
}
func archiveCandidates(owner, repo string) []string {
	return []string{"https://github.com/" + owner + "/" + repo + "/archive/HEAD.zip", "https://codeload.github.com/" + owner + "/" + repo + "/zip/refs/heads/main", "https://codeload.github.com/" + owner + "/" + repo + "/zip/refs/heads/master", "https://api.github.com/repos/" + owner + "/" + repo + "/zipball"}
}
func (m *Manager) download(ctx context.Context, source string) ([]byte, []string, error) {
	owner, repo, e := ParseSource(source)
	if e != nil {
		return nil, nil, e
	}
	warnings := []string{}
	var archive []byte
	var failures []error
	for _, candidate := range archiveCandidates(owner, repo) {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		req, e := netx.Request(ctx, m.client, candidate)
		if e != nil {
			return nil, nil, e
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, e := m.client.Do(req)
		if e == nil {
			archive, e = netx.ReadBody(resp, maxArchiveBytes)
			if e == nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
				e = netx.StatusError(resp.StatusCode, archive, "")
			}
			if e == nil && len(archive) == 0 {
				e = validation("skill archive is empty")
			}
		}
		if e == nil {
			break
		}
		failures = append(failures, e)
		warnings = append(warnings, "GitHub archive candidate failed: "+candidate+": "+e.Error())
		archive = nil
	}
	if archive == nil {
		return nil, nil, errors.Join(failures...)
	}
	return archive, warnings, nil
}
func (m *Manager) Install(ctx context.Context, source string, target domain.SkillTarget, skillName string) (domain.SkillInstallResult, error) {
	fsMu.Lock()
	_, e := m.root(target)
	fsMu.Unlock()
	if e != nil {
		return domain.SkillInstallResult{}, e
	}
	archive, warnings, e := m.download(ctx, source)
	if e != nil {
		return domain.SkillInstallResult{}, e
	}
	result, e := m.InstallArchive(target, archive, source, skillName)
	result.Warnings = append(warnings, result.Warnings...)
	return result, e
}

func extractArchive(archive []byte, root string) (string, error) {
	if int64(len(archive)) > maxArchiveBytes {
		return "", validation("skill archive is too large")
	}
	z, e := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if e != nil {
		return "", e
	}
	if len(z.File) == 0 || len(z.File) > 4096 {
		return "", validation("invalid archive entry count")
	}
	top := ""
	seen := map[string]bool{}
	total := uint64(0)
	for _, entry := range z.File {
		name := strings.TrimSuffix(entry.Name, "/")
		parts := strings.Split(name, "/")
		if name == "" || len(parts) > 12 || path.IsAbs(name) || strings.ContainsAny(name, "\\:\x00") {
			return "", validation("unsafe ZIP path")
		}
		for _, part := range parts {
			if part == "" || part == "." || part == ".." {
				return "", validation("non-normalized ZIP path")
			}
		}
		if top == "" {
			top = parts[0]
		} else if top != parts[0] {
			return "", validation("archive must have one top-level directory")
		}
		key := strings.ToLower(name)
		if seen[key] {
			return "", validation("duplicate ZIP path")
		}
		seen[key] = true
		if !entry.FileInfo().IsDir() && !entry.Mode().IsRegular() {
			return "", validation("archive contains a link or special file")
		}
		if entry.UncompressedSize64 > uint64(maxArchiveBytes) || entry.UncompressedSize64 > 128*1024*1024-total {
			return "", validation("archive extraction exceeds size limit")
		}
		total += entry.UncompressedSize64
	}
	for _, entry := range z.File {
		dest := filepath.Join(root, filepath.FromSlash(entry.Name))
		if entry.FileInfo().IsDir() {
			if e = os.MkdirAll(dest, 0700); e != nil {
				return "", e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return "", e
		}
		r, e := entry.Open()
		if e != nil {
			return "", e
		}
		f, e := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			r.Close()
			return "", e
		}
		n, copyErr := io.Copy(f, io.LimitReader(r, maxArchiveBytes+1))
		e = errors.Join(copyErr, f.Sync(), f.Close(), r.Close())
		if e != nil {
			return "", e
		}
		if n > maxArchiveBytes || uint64(n) != entry.UncompressedSize64 {
			return "", validation("ZIP entry size mismatch")
		}
		if e = os.Chmod(dest, 0600|entry.Mode().Perm()&0111); e != nil {
			return "", e
		}
	}
	return filepath.Join(root, top), nil
}
