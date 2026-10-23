package quota

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

func publicBases(codex string) []string {
	base := withoutV1(codex)
	out := []string{}
	if base != "" {
		out = append(out, base)
	}
	u, e := url.Parse(codex)
	if e == nil && u.Host != "" {
		origin := u.Scheme + "://" + u.Host
		if origin != base {
			out = append(out, origin)
		}
	}
	return out
}
func withoutV1(raw string) string {
	raw = strings.TrimRight(raw, "/")
	if strings.HasSuffix(strings.ToLower(raw), "/v1") {
		return strings.TrimRight(raw[:len(raw)-3], "/")
	}
	return ""
}
func billingURLs(base string, today time.Time) (string, string, string) {
	base = strings.TrimRight(base, "/")
	start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	end := today.AddDate(0, 0, 1).Format("2006-01-02")
	prefix := base + "/dashboard/billing/"
	return prefix + "credit_grants", prefix + "subscription", prefix + "usage?start_date=" + start + "&end_date=" + end
}
func fetchHit(ctx context.Context, client *http.Client, endpoint, key, expected string, authenticated bool) hit {
	req, e := netx.Request(ctx, client, endpoint)
	if e != nil {
		return hit{kind: "error", message: netx.Sanitize(e.Error(), key)}
	}
	req.Header.Set("Accept", "application/json")
	if authenticated {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, e := client.Do(req)
	if e != nil {
		return hit{kind: "error", message: netx.NetworkError(e, key).Message}
	}
	body, e := netx.ReadBody(resp, 64*1024)
	if e != nil {
		return hit{kind: "error", message: netx.NetworkError(e, key).Message}
	}
	return classify(resp.StatusCode, body, expected)
}
func fetchPublicPair(ctx context.Context, client *http.Client, bases []string, key string) (hit, string, hit) {
	for i, base := range bases {
		status := fetchHit(ctx, client, base+"/api/status", key, "status", false)
		if i > 0 && status.kind != "status" {
			continue
		}
		endpoint := base + "/api/usage/token"
		token := fetchHit(ctx, client, endpoint, key, "token", true)
		if token.kind != "not_found" && token.kind != "unsupported" {
			return token, endpoint, status
		}
	}
	return hit{kind: "unsupported"}, "", hit{kind: "unsupported"}
}
func fetchRound(ctx context.Context, client *http.Client, base string, bases []string, key string) round {
	g, s, u := billingURLs(base, time.Now().UTC())
	r := round{grantsURL: g, subscriptionURL: s, usageURL: u}
	var wg sync.WaitGroup
	wg.Add(4)
	go func() { defer wg.Done(); r.grants = fetchHit(ctx, client, g, key, "grants", true) }()
	go func() { defer wg.Done(); r.subscription = fetchHit(ctx, client, s, key, "subscription", true) }()
	go func() { defer wg.Done(); r.usage = fetchHit(ctx, client, u, key, "usage", true) }()
	go func() { defer wg.Done(); r.token, r.tokenURL, r.status = fetchPublicPair(ctx, client, bases, key) }()
	wg.Wait()
	return r
}
func Probe(ctx context.Context, client *http.Client, site domain.SiteDTO, key string) (domain.SiteQuota, error) {
	start := time.Now()
	fetched := start.UnixMilli()
	if strings.TrimSpace(key) == "" {
		return quiet("unsupported", nil, nil, fetched, 0), nil
	}
	if strings.ContainsAny(key, "\r\n") {
		return domain.SiteQuota{}, domain.NewError("validation_failed", "invalid API key")
	}
	if client == nil {
		return domain.SiteQuota{}, domain.NewError("validation_failed", "HTTP client is required")
	}
	preview, e := netx.NormalizeURL(site.BaseURL)
	if e != nil {
		return domain.SiteQuota{}, e
	}
	r := fetchRound(ctx, client, preview.CodexBaseURL, publicBases(preview.CodexBaseURL), key)
	if ctx.Err() != nil {
		return domain.SiteQuota{}, netx.NetworkError(ctx.Err(), key)
	}
	q := interpret(r, fetched, uint64(time.Since(start).Milliseconds()))
	// Existing billing endpoints may live beside /v1; keep its explicit compatibility round
	if sourceRank(q) < 4 {
		if base := withoutV1(preview.CodexBaseURL); base != "" && base != preview.CodexBaseURL {
			r = fetchRound(ctx, client, base, nil, key)
			if ctx.Err() != nil {
				return domain.SiteQuota{}, netx.NetworkError(ctx.Err(), key)
			}
			q = combine(q, interpret(r, fetched, uint64(time.Since(start).Milliseconds())))
		}
	}
	if q.Error != nil {
		v := netx.Sanitize(*q.Error, key)
		q.Error = &v
	}
	q.LatencyMS = uint64(time.Since(start).Milliseconds())
	return q, nil
}
