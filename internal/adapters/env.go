package adapters

import (
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/envinject"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"strconv"
	"strings"
)

func envValue(content []byte, key string) (string, bool, error) {
	for _, line := range strings.Split(string(content), "\n") {
		s := strings.TrimSpace(line)
		s = strings.TrimPrefix(s, "export ")
		name, raw, ok := strings.Cut(s, "=")
		if !ok || strings.TrimSpace(name) != key {
			continue
		}
		raw = strings.TrimSpace(raw)
		if strings.HasPrefix(raw, "'") {
			raw = strings.ReplaceAll(raw, "'\"'\"'", "\x00")
			if len(raw) < 2 || raw[len(raw)-1] != '\'' {
				return "", false, errors.New("invalid_config: unsupported env quoting")
			}
			return strings.ReplaceAll(raw[1:len(raw)-1], "\x00", "'"), true, nil
		}
		if strings.HasPrefix(raw, "\"") {
			v, e := strconv.Unquote(raw)
			if e != nil {
				return "", false, errors.New("invalid_config: unsupported env quoting")
			}
			return v, true, nil
		}
		if strings.ContainsAny(raw, "$` \\;") {
			return "", false, errors.New("invalid_config: unsupported env value expression")
		}
		return raw, true, nil
	}
	return "", false, nil
}
func envPath(b domain.TargetBinding) string {
	for _, path := range b.ManagedPaths {
		if strings.HasSuffix(path, "codex.env") {
			return path
		}
	}
	return ""
}
func rewriteEnv(p *Plan, b domain.TargetBinding, value string, remove bool) error {
	path := envPath(b)
	if path == "" {
		return errors.New("invalid_binding: no managed Codex env file")
	}
	c, e := fileops.Snapshot(path, nil)
	if e != nil {
		return e
	}
	after := c.Before
	for _, key := range b.ManagedEnvKeys {
		live, exists, e := envValue(after, key)
		if e != nil {
			return e
		}
		if remove && (!exists || fingerprint(live) != b.KeyFingerprint) {
			continue
		}
		after, e = envinject.Upsert(after, key, value, remove)
		if e != nil {
			return e
		}
	}
	c.After = after
	appendChange(p, c)
	return nil
}
func revertEnv(p *Plan, b domain.TargetBinding) error { return rewriteEnv(p, b, "", true) }
