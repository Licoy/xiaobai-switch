package quota

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"math"
)

func consistent(a amounts, unlimited bool) bool {
	if unlimited {
		return a.used == nil || *a.used >= 0
	}
	for _, n := range []*float64{a.remaining, a.used, a.total} {
		if n != nil && (*n < 0 || math.IsNaN(*n) || math.IsInf(*n, 0)) {
			return false
		}
	}
	if a.total != nil {
		tol := math.Abs(*a.total)*1e-6 + 1e-6
		if (a.used != nil && *a.used-*a.total > tol) || (a.remaining != nil && *a.remaining-*a.total > tol) {
			return false
		}
	}
	if a.remaining == nil || a.used == nil || a.total == nil {
		return true
	}
	tol := max(math.Abs(*a.total), math.Abs(*a.remaining), math.Abs(*a.used))*1e-6 + 1e-6
	return math.Abs(*a.total-*a.remaining-*a.used) <= tol
}
func normalizeToken(h, status hit) hit {
	if h.kind != "token" {
		return h
	}
	t := h.token
	if !t.display {
		scale := 1.0
		unit := "RAW_QUOTA"
		if status.kind == "status" {
			m := status.meta
			switch m.display {
			case "TOKENS":
				unit = "TOKENS"
			case "USD":
				if m.perUnit != nil {
					scale = 1 / *m.perUnit
					unit = "USD"
				}
			case "CNY":
				if m.perUnit != nil && m.usdRate != nil {
					scale = *m.usdRate / *m.perUnit
					unit = "CNY"
				}
			case "CUSTOM":
				if m.perUnit != nil && m.customRate != nil {
					scale = *m.customRate / *m.perUnit
					unit = m.symbol
					if unit == "" {
						unit = "CUSTOM"
					}
				}
			}
		}
		t.remaining = scaled(t.remaining, scale)
		t.used = scaled(t.used, scale)
		t.total = scaled(t.total, scale)
		t.unit = unit
	}
	if !consistent(t.amounts, t.unlimited) {
		return hit{kind: "invalid_data", message: "inconsistent quota data"}
	}
	h.token = t
	return h
}
func scaled(v *float64, scale float64) *float64 {
	if v == nil {
		return nil
	}
	n := *v * scale
	return &n
}
func quiet(status string, message *string, endpoint *string, fetched int64, latency uint64) domain.SiteQuota {
	return domain.SiteQuota{Status: domain.QuotaProbeStatus(status), Error: message, Endpoint: endpoint, FetchedAt: fetched, LatencyMS: latency}
}
func available(source, unit, endpoint string, a amounts, unlimited bool, expires *int64, fetched int64, latency uint64) (domain.SiteQuota, bool) {
	if !consistent(a, unlimited) {
		return domain.SiteQuota{}, false
	}
	if unlimited {
		a.remaining = nil
		a.total = nil
	}
	s := domain.QuotaSource(source)
	return domain.SiteQuota{Status: "available", RemainingUSD: a.remaining, UsedUSD: a.used, TotalUSD: a.total, Unlimited: unlimited, Unit: &unit, ExpiresAt: expires, Source: &s, Endpoint: &endpoint, FetchedAt: fetched, LatencyMS: latency}, true
}
func sourceRank(q domain.SiteQuota) int {
	if q.Source == nil {
		return 0
	}
	return map[domain.QuotaSource]int{"token_usage": 5, "credit_grants": 4, "subscription_usage": 3, "subscription_only": 2, "usage_only": 1}[*q.Source]
}
func combine(first, second domain.SiteQuota) domain.SiteQuota {
	if first.Status == "available" && second.Status == "available" {
		if sourceRank(first) >= sourceRank(second) {
			return first
		}
		return second
	}
	if first.Status == "available" {
		return first
	}
	if second.Status == "available" {
		return second
	}
	rank := map[domain.QuotaProbeStatus]int{"invalid_data": 4, "unauthorized": 3, "error": 2, "unsupported": 1}
	if rank[first.Status] >= rank[second.Status] {
		return first
	}
	return second
}

type round struct {
	grants, subscription, usage, token, status     hit
	grantsURL, subscriptionURL, usageURL, tokenURL string
}

func interpret(r round, fetched int64, latency uint64) domain.SiteQuota {
	invalid, invalidURL := "", ""
	token := normalizeToken(r.token, r.status)
	if token.kind == "invalid_data" {
		invalid = token.message
		invalidURL = r.tokenURL
	}
	expires := r.subscription.expires
	if token.kind == "token" {
		t := token.token
		exp := t.expires
		if exp == nil {
			exp = expires
		}
		if q, ok := available("token_usage", t.unit, r.tokenURL, t.amounts, t.unlimited, exp, fetched, latency); ok {
			return q
		}
		invalid = "inconsistent quota data"
		invalidURL = r.tokenURL
	}
	if r.grants.kind == "grants" {
		if q, ok := available("credit_grants", "USD", r.grantsURL, r.grants.amounts, false, expires, fetched, latency); ok {
			return q
		}
		if invalid == "" {
			invalid = "inconsistent quota data"
			invalidURL = r.grantsURL
		}
	}
	sub, usage := r.subscription, r.usage
	a := amounts{}
	source, endpoint := "", ""
	if sub.kind == "subscription" && usage.kind == "usage" {
		used := usage.usage / 100
		a.used = &used
		a.total = sub.limit
		if sub.limit != nil {
			n := *sub.limit - used
			a.remaining = &n
		}
		source = "subscription_usage"
		endpoint = r.subscriptionURL
	} else if sub.kind == "subscription" && sub.limit != nil {
		a.total = sub.limit
		source = "subscription_only"
		endpoint = r.subscriptionURL
	} else if usage.kind == "usage" {
		used := usage.usage / 100
		a.used = &used
		source = "usage_only"
		endpoint = r.usageURL
	}
	if source != "" {
		if q, ok := available(source, "USD", endpoint, a, false, expires, fetched, latency); ok {
			return q
		}
		if invalid == "" {
			invalid = "inconsistent quota data"
			invalidURL = endpoint
		}
	}
	if invalid != "" {
		return quiet("invalid_data", &invalid, &invalidURL, fetched, latency)
	}
	hits := []hit{r.grants, sub, usage, token}
	for _, h := range hits {
		if h.kind == "unauthorized" {
			return quiet("unauthorized", nil, nil, fetched, latency)
		}
	}
	for _, h := range hits {
		if h.kind == "error" {
			return quiet("error", &h.message, nil, fetched, latency)
		}
	}
	return quiet("unsupported", nil, nil, fetched, latency)
}
