package app

import (
	"github.com/licoy/xiaobai-switch/internal/netx"
	"net/url"
	"regexp"
)

var credentialValue = regexp.MustCompile(`(?i)(authorization|x-api-key|api[_-]?key|token|secret|password)(["'\s:=]+)(?:bearer\s+)?[^\s,;}"']+`)
var bearerValue = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/-]+=*`)
var keyToken = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]+`)
var errorURL = regexp.MustCompile(`https?://[^\s<>"']+`)

// Service errors already redact active credentials; scrub transient editor values and credential-shaped text again at the UI boundary
func (s *Shell) safeErrorDetail(detail string) string {
	if s.Editor != nil {
		detail = netx.Sanitize(detail, s.Editor.Key)
		for _, key := range s.Editor.ExtraKeys {
			detail = netx.Sanitize(detail, key)
		}
	}
	detail = netx.Sanitize(detail, s.BackupUI.Password)
	if s.KeyEditor != nil {
		detail = netx.Sanitize(detail, s.KeyEditor.Key)
	}
	detail = errorURL.ReplaceAllStringFunc(detail, func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil {
			return "[redacted-url]"
		}
		u.User = nil
		u.RawQuery = ""
		u.ForceQuery = false
		u.Fragment = ""
		return u.String()
	})
	detail = credentialValue.ReplaceAllString(detail, "$1$2[redacted]")
	detail = bearerValue.ReplaceAllString(detail, "Bearer [redacted]")
	return keyToken.ReplaceAllString(detail, "[redacted]")
}
