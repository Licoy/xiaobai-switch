package deeplink

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"github.com/licoy/xiaobai-switch/internal/store"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var capabilityKey = regexp.MustCompile(`^[a-z][a-z0-9]{0,15}-[a-z0-9-]{1,48}$`)

func Parse(raw string) (domain.DeepLinkSiteImportInput, error) {
	var in domain.DeepLinkSiteImportInput
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "xiaobaiswitch" || u.User != nil || u.Fragment != "" {
		return in, domain.NewError("validation_failed", "invalid site deep link")
	}
	target := u.Host
	if target == "" {
		target = strings.Split(strings.TrimLeft(u.Path, "/"), "/")[0]
	}
	if target != "sites" {
		return in, domain.NewError("validation_failed", "unsupported deep link target")
	}
	values, e := url.ParseQuery(u.RawQuery)
	if e != nil {
		return in, domain.NewError("validation_failed", "invalid deep link query")
	}
	in.Name = values.Get("name")
	in.APIKey = values.Get("apikey")
	notes := values.Get("notes")
	if notes != "" {
		in.Notes = &notes
	}
	keyName := values.Get("keyname")
	if keyName != "" {
		in.KeyName = &keyName
	}
	protocol := values.Get("protocol")
	if _, exists := values["protocol"]; !exists {
		protocol = values.Get("type")
	}
	in.Protocol = &protocol
	caps := domain.SiteCapabilities{}
	accepted := 0
	present := false
	for _, part := range strings.Split(u.RawQuery, "&") {
		k, v, _ := strings.Cut(part, "=")
		key, ke := url.QueryUnescape(k)
		value, ve := url.QueryUnescape(v)
		if ke != nil || ve != nil {
			return in, domain.NewError("validation_failed", "invalid deep link encoding")
		}
		if key == "baseurl" || key == "baseurls" {
			for _, route := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '|' }) {
				in.BaseURLs = append(in.BaseURLs, route)
			}
		} else if capabilityKey.MatchString(key) {
			present = true
			if accepted < 32 {
				switch strings.ToLower(strings.TrimSpace(value)) {
				case "1", "true", "on", "yes":
					caps[key] = true
				default:
					caps[key] = false
				}
				accepted++
			}
		}
	}
	if present {
		for _, key := range []string{"codex-compact", "codex-vision", "codex-imagegen", "codex-search"} {
			if _, ok := caps[key]; !ok {
				caps[key] = false
			}
		}
		in.Capabilities = &caps
	}
	return normalize(in, false)
}
func Validate(in domain.DeepLinkSiteImportInput) (domain.DeepLinkSiteImportInput, error) {
	return normalize(in, true)
}
func normalize(in domain.DeepLinkSiteImportInput, requireKey bool) (domain.DeepLinkSiteImportInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.APIKey = strings.TrimSpace(in.APIKey)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 128 {
		return in, domain.NewError("validation_failed", "site name is required and limited to 128 characters")
	}
	if strings.ContainsAny(in.APIKey, "\r\n") || (requireKey && in.APIKey == "") {
		return in, domain.NewError("validation_failed", "valid API key is required")
	}
	for _, route := range in.BaseURLs {
		if utf8.RuneCountInString(route) > 2048 {
			return in, domain.NewError("validation_failed", "base URL exceeds 2048 characters")
		}
	}
	urls, e := netx.NormalizeBaseURLs(in.BaseURLs)
	if e != nil {
		return in, e
	}
	if len(urls) > 20 {
		return in, domain.NewError("validation_failed", "at most 20 base URLs are allowed")
	}
	in.BaseURLs = urls
	protocol := "openai_compatible"
	if in.Protocol != nil {
		switch strings.ToLower(strings.TrimSpace(*in.Protocol)) {
		case "", "openai", "openai_compatible":
		case "anthropic":
			protocol = "anthropic"
		default:
			return in, domain.NewError("validation_failed", "unsupported site protocol")
		}
	}
	in.Protocol = &protocol
	if in.Notes != nil {
		value := strings.TrimSpace(*in.Notes)
		in.Notes = nil
		if utf8.RuneCountInString(value) > 2000 {
			return in, domain.NewError("validation_failed", "notes exceed 2000 characters")
		}
		if value != "" {
			in.Notes = &value
		}
	}
	if in.KeyName != nil {
		value := strings.TrimSpace(*in.KeyName)
		in.KeyName = nil
		if value != "" {
			in.KeyName = &value
		}
	}
	return in, nil
}
func Import(s *store.Store, input domain.DeepLinkSiteImportInput) (domain.DeepLinkSiteImportResult, error) {
	in, e := Validate(input)
	if e != nil {
		return domain.DeepLinkSiteImportResult{}, e
	}
	return s.ImportDeepLink(in)
}
