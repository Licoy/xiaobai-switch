package quota

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

type amounts struct{ remaining, used, total *float64 }
type tokenUsage struct {
	amounts
	unit               string
	unlimited, display bool
	expires            *int64
}
type metadata struct {
	perUnit, usdRate, customRate *float64
	display, symbol              string
}
type hit struct {
	kind string
	amounts
	limit   *float64
	expires *int64
	usage   float64
	token   tokenUsage
	meta    metadata
	message string
}

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func number(v any) *float64 {
	var n float64
	switch v := v.(type) {
	case float64:
		n = v
	case string:
		var e error
		n, e = strconv.ParseFloat(strings.TrimSpace(v), 64)
		if e != nil {
			return nil
		}
	default:
		return nil
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	return &n
}
func positive(v any) *float64 {
	n := number(v)
	if n == nil || *n <= 0 {
		return nil
	}
	return n
}
func expiration(v any) *int64 {
	n := number(v)
	if n == nil || *n <= 0 || *n >= math.MaxInt64 {
		return nil
	}
	value := int64(*n)
	return &value
}
func flag(v any) bool   { b, _ := v.(bool); return b }
func text(v any) string { s, _ := v.(string); return strings.TrimSpace(s) }
func failedFlag(v map[string]any) bool {
	b, ok := v["success"].(bool)
	if ok && !b {
		return true
	}
	b, ok = v["code"].(bool)
	return ok && !b
}
func responseFailed(v map[string]any) bool {
	if failedFlag(v) {
		return true
	}
	switch e := v["error"].(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(e) != ""
	case bool:
		return e
	case []any:
		return len(e) > 0
	case map[string]any:
		return len(e) > 0
	default:
		return true
	}
}
func parseGrants(v map[string]any) hit {
	a := amounts{number(v["total_available"]), number(v["total_used"]), number(v["total_granted"])}
	if a.remaining == nil && a.used == nil && a.total == nil {
		return hit{kind: "unsupported"}
	}
	if a.remaining == nil && a.total != nil && a.used != nil {
		n := *a.total - *a.used
		a.remaining = &n
	}
	if a.used == nil && a.total != nil && a.remaining != nil {
		n := *a.total - *a.remaining
		a.used = &n
	}
	return hit{kind: "grants", amounts: a}
}
func parseSubscription(v map[string]any) hit {
	limit := number(v["hard_limit_usd"])
	if limit == nil {
		limit = number(v["system_hard_limit_usd"])
	}
	if limit == nil {
		limit = number(v["soft_limit_usd"])
	}
	expires := expiration(v["access_until"])
	if limit == nil && expires == nil {
		return hit{kind: "unsupported"}
	}
	return hit{kind: "subscription", limit: limit, expires: expires}
}
func normalizeUnit(raw string) string {
	switch s := strings.ToUpper(strings.TrimSpace(raw)); s {
	case "RMB", "CNY", "¥", "元":
		return "CNY"
	case "USD", "$", "":
		return "USD"
	default:
		return s
	}
}
func parseToken(v map[string]any) hit {
	data := obj(v["data"])
	if data == nil {
		return hit{kind: "unsupported"}
	}
	display := obj(data["display"])
	a := amounts{number(display["remaining"]), number(display["used"]), number(display["total"])}
	_, hasUnit := display["unit"].(string)
	hasDisplay := a.remaining != nil && a.used != nil && a.total != nil && hasUnit
	unit := "quota"
	unlimited := flag(data["unlimited_quota"])
	if hasDisplay {
		unit = normalizeUnit(text(display["unit"]))
		unlimited = false
	} else {
		a = amounts{number(data["total_available"]), number(data["total_used"]), number(data["total_granted"])}
	}
	if a.remaining == nil && a.used == nil && a.total == nil {
		return hit{kind: "invalid_data", message: "invalid quota data"}
	}
	return hit{kind: "token", token: tokenUsage{amounts: a, unit: unit, unlimited: unlimited, display: hasDisplay, expires: expiration(data["expires_at"])}}
}
func parseMetadata(v map[string]any) hit {
	data := obj(v["data"])
	if data == nil {
		return hit{kind: "unsupported"}
	}
	display := "RAW"
	if raw, ok := data["quota_display_type"].(string); ok {
		switch s := strings.ToUpper(strings.TrimSpace(raw)); s {
		case "USD", "CNY", "TOKENS", "CUSTOM":
			display = s
		}
	} else if enabled, ok := data["display_in_currency"].(bool); ok {
		display = "TOKENS"
		if enabled {
			display = "USD"
		}
	}
	return hit{kind: "status", meta: metadata{perUnit: positive(data["quota_per_unit"]), usdRate: positive(data["usd_exchange_rate"]), customRate: positive(data["custom_currency_exchange_rate"]), display: display, symbol: text(data["custom_currency_symbol"])}}
}
func classify(status int, body []byte, expected string) hit {
	html := strings.ToLower(strings.TrimSpace(string(body)))
	isHTML := strings.HasPrefix(html, "<!doctype") || strings.HasPrefix(html, "<html")
	switch status {
	case 408:
		return hit{kind: "error", message: "request timed out"}
	case 401:
		return hit{kind: "unauthorized"}
	case 403:
		if isHTML {
			return hit{kind: "unsupported"}
		}
		return hit{kind: "unauthorized"}
	case 404, 405, 501:
		return hit{kind: "not_found"}
	}
	if status < 200 || status >= 300 {
		return hit{kind: "error", message: "HTTP " + strconv.Itoa(status)}
	}
	if isHTML {
		return hit{kind: "unsupported"}
	}
	var v map[string]any
	if json.Unmarshal(body, &v) != nil || v == nil {
		return hit{kind: "unsupported"}
	}
	if expected == "token" && failedFlag(v) {
		message := strings.ToLower(text(v["message"]))
		for _, fragment := range []string{"authorization", "bearer", "token not found", "invalid token", "unauthorized", "forbidden"} {
			if strings.Contains(message, fragment) {
				return hit{kind: "unauthorized"}
			}
		}
	}
	if responseFailed(v) {
		return hit{kind: "error", message: "upstream response indicated failure"}
	}
	switch expected {
	case "grants":
		return parseGrants(v)
	case "subscription":
		return parseSubscription(v)
	case "usage":
		if n := number(v["total_usage"]); n != nil {
			return hit{kind: "usage", usage: *n}
		}
	case "token":
		return parseToken(v)
	case "status":
		return parseMetadata(v)
	}
	return hit{kind: "unsupported"}
}
