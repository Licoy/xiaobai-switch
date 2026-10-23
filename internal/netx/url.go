package netx

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"net/url"
	"strings"
	"unicode"
)

func validHTTP(raw string) (*url.URL, error) {
	if strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return nil, domain.NewError("validation_failed", "URL must not contain whitespace")
	}
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || strings.ContainsAny(raw, "\r\n\\") {
		return nil, domain.NewError("validation_failed", "HTTP(S) URL without credentials is required")
	}
	return u, nil
}
func NormalizeURL(raw string) (domain.URLWritePreview, error) {
	raw = strings.TrimSpace(raw)
	u, e := validHTTP(raw)
	if e != nil {
		return domain.URLWritePreview{}, e
	}
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	base := strings.TrimRight(u.String(), "/")
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, "/v1/messages") {
		base = base[:len(base)-len("/v1/messages")] + "/v1"
	} else if strings.HasSuffix(lower, "/messages") {
		base = base[:len(base)-len("/messages")]
	}
	base = strings.TrimRight(base, "/")
	codex := base
	if !strings.HasSuffix(strings.ToLower(base), "/v1") {
		codex += "/v1"
	}
	return domain.URLWritePreview{ModelsURL: codex + "/models", ClaudeBaseURL: base, CodexBaseURL: codex}, nil
}

// NormalizeBaseURLs preserves route spelling and ordering, including private paths
func NormalizeBaseURLs(raw []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, e := validHTTP(v); e != nil {
			return nil, e
		}
		if !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	if len(out) == 0 {
		return nil, domain.NewError("validation_failed", "at least one base URL is required")
	}
	return out, nil
}
