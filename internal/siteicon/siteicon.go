// Package siteicon finds the icon of a site as the 0.0.x Tauri version did: the icons its home page links,
// best first, then /favicon.ico of the origin of its base URL.
package siteicon

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/licoy/xiaobai-switch/internal/netx"
)

// maxBody truncates pages and icons as the Tauri fetch_http_text/fetch_http_bytes commands did
const maxBody = 512 * 1024

var (
	bodyHTML = regexp.MustCompile(`(?i)<!doctype\s+html|<html[\s>]|<head[\s>]`)
	linkTag  = regexp.MustCompile(`(?is)<link\b([^>]*)>`)
	attrPair = regexp.MustCompile(`(?s)([^\s=/>"']+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+)))?`)
	digits   = regexp.MustCompile(`\d+`)
)

// Origin is the scheme, host and port of an http(s) base URL, as URL.origin; false for anything else
func Origin(baseURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", false
	}
	scheme, host, port := strings.ToLower(u.Scheme), strings.ToLower(u.Hostname()), u.Port()
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		host += ":" + port
	}
	return scheme + "://" + host, true
}

// IsHTML tells a page from an API answer by its content type, or by its first bytes when that says nothing
func IsHTML(contentType, body string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml") {
		return true
	}
	if strings.Contains(ct, "json") || strings.Contains(ct, "javascript") || strings.Contains(ct, "image/") {
		return false
	}
	if len(body) > 2000 {
		body = body[:2000]
	}
	return bodyHTML.MatchString(body)
}

func priority(rel, sizes string) float64 {
	tokens := strings.Fields(strings.ToLower(rel))
	has := func(name string) bool {
		for _, t := range tokens {
			if t == name {
				return true
			}
		}
		return false
	}
	score := 0.0
	apple := has("apple-touch-icon") || has("apple-touch-icon-precomposed")
	if has("icon") && !apple {
		score += 40
	}
	if !has("shortcut") {
		score += 5
	}
	if apple {
		score += 10
	}
	if sizes != "" && sizes != "any" {
		n := 0
		for _, part := range strings.Fields(sizes) {
			if m := digits.FindString(part); m != "" {
				if v, err := strconv.Atoi(m); err == nil && v > n {
					n = v
				}
			}
		}
		score += float64(min(n, 256)) / 16
	}
	return score
}

func attributes(tag string) map[string]*string {
	out := map[string]*string{}
	for _, m := range attrPair.FindAllStringSubmatchIndex(tag, -1) {
		name := strings.ToLower(tag[m[2]:m[3]])
		if _, seen := out[name]; seen {
			continue
		}
		var v *string
		for g := 4; g <= 8; g += 2 {
			if m[g] >= 0 {
				s := html.UnescapeString(tag[m[g]:m[g+1]])
				v = &s
				break
			}
		}
		if v == nil {
			empty := ""
			v = &empty
		}
		out[name] = v
	}
	return out
}

// ParseIconHrefs lists the icons a page links, absolute against its URL, best first; mask icons are skipped
func ParseIconHrefs(page, pageURL string) []string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	type ranked struct {
		href  string
		score float64
	}
	rows := []ranked{}
	for _, m := range linkTag.FindAllStringSubmatch(page, -1) {
		attrs := attributes(m[1])
		rel, href := attrs["rel"], attrs["href"]
		if rel == nil || href == nil || strings.TrimSpace(*href) == "" {
			continue
		}
		tokens := strings.Fields(strings.ToLower(*rel))
		icon := false
		for _, t := range tokens {
			if t == "mask-icon" {
				icon = false
				break
			}
			icon = icon || t == "icon" || t == "apple-touch-icon" || t == "apple-touch-icon-precomposed"
		}
		if !icon {
			continue
		}
		abs, err := base.Parse(strings.TrimSpace(*href))
		if err != nil {
			continue
		}
		sizes := ""
		if attrs["sizes"] != nil {
			sizes = *attrs["sizes"]
		}
		rows = append(rows, ranked{abs.String(), priority(*rel, sizes)})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].score > rows[j].score })
	out, seen := []string{}, map[string]bool{}
	for _, r := range rows {
		if !seen[r.href] {
			seen[r.href] = true
			out = append(out, r.href)
		}
	}
	return out
}

// LooksImage accepts what the Tauri version turned into an image data URL
func LooksImage(status int, contentType string, body []byte) bool {
	if status < 200 || status >= 400 || len(body) == 0 {
		return false
	}
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return strings.HasPrefix(ct, "image/") || strings.Contains(ct, "icon") || ct == "application/octet-stream" || ct == ""
}

type response struct {
	status      int
	contentType string
	finalURL    string
	body        []byte
}

func fetch(ctx context.Context, client *http.Client, raw, accept string) (response, bool) {
	req, err := netx.Request(ctx, client, raw)
	if err != nil {
		return response{}, false
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := client.Do(req)
	if err != nil {
		return response{}, false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	return response{resp.StatusCode, resp.Header.Get("Content-Type"), resp.Request.URL.String(), body}, true
}

// Result is the icon found, nil for none; Reached is false when no request got an answer, so none is not final
type Result struct {
	Icon    []byte
	Reached bool
}

// Resolve tries the linked icons of the home page and then /favicon.ico; usable rejects bytes the caller can't show
func Resolve(ctx context.Context, client *http.Client, origin string, usable func([]byte) bool) Result {
	reached := false
	probe := func(raw string) []byte {
		r, ok := fetch(ctx, client, raw, "")
		reached = reached || ok
		if ok && LooksImage(r.status, r.contentType, r.body) && usable(r.body) {
			return r.body
		}
		return nil
	}
	if page, ok := fetch(ctx, client, origin+"/", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8"); ok {
		reached = true
		if page.status >= 200 && page.status < 400 && IsHTML(page.contentType, string(page.body)) {
			base := page.finalURL
			if base == "" {
				base = origin + "/"
			}
			for _, href := range ParseIconHrefs(string(page.body), base) {
				if ctx.Err() != nil {
					return Result{Reached: false}
				}
				if data := probe(href); data != nil {
					return Result{data, true}
				}
			}
		}
	}
	if ctx.Err() != nil {
		return Result{}
	}
	data := probe(origin + "/favicon.ico")
	return Result{data, reached}
}
