package quota

import (
	"math"
	"testing"
	"time"
)

func TestParseAndNormalizeTokenUnits(t *testing.T) {
	raw := classify(200, []byte(`{"code":true,"data":{"total_available":500000,"total_used":250000,"total_granted":750000}}`), "token")
	if raw.kind != "token" {
		t.Fatalf("token %+v", raw)
	}
	for _, tt := range []struct {
		json, unit string
		remaining  float64
	}{{`{"data":{"quota_display_type":"USD","quota_per_unit":500000}}`, "USD", 1}, {`{"data":{"quota_display_type":"CNY","quota_per_unit":500000,"usd_exchange_rate":7}}`, "CNY", 7}, {`{"data":{"quota_display_type":"TOKENS"}}`, "TOKENS", 500000}, {`{"data":{"quota_display_type":"CUSTOM","quota_per_unit":500000,"custom_currency_exchange_rate":2.5,"custom_currency_symbol":"CR"}}`, "CR", 2.5}, {`{"data":{}}`, "RAW_QUOTA", 500000}, {`{"data":{"display_in_currency":false}}`, "TOKENS", 500000}} {
		status := classify(200, []byte(tt.json), "status")
		got := normalizeToken(raw, status)
		if got.kind != "token" || got.token.unit != tt.unit || got.token.remaining == nil || math.Abs(*got.token.remaining-tt.remaining) > 1e-9 {
			t.Fatalf("normalize %+v from %s", got, tt.json)
		}
	}
}
func TestDisplayPrecedenceAndUnlimitedConsistency(t *testing.T) {
	display := classify(200, []byte(`{"data":{"display":{"remaining":7,"used":3,"total":10,"unit":"RMB"},"total_available":-1,"total_used":11,"total_granted":10,"unlimited_quota":true}}`), "token")
	display = normalizeToken(display, hit{})
	if display.kind != "token" || display.token.unit != "CNY" || display.token.unlimited || *display.token.remaining != 7 {
		t.Fatalf("display %+v", display)
	}
	partial := classify(200, []byte(`{"data":{"display":{"remaining":7,"unit":"CNY"},"total_available":2,"total_used":1,"total_granted":3}}`), "token")
	partial = normalizeToken(partial, hit{})
	if partial.token.unit != "RAW_QUOTA" || *partial.token.remaining != 2 {
		t.Fatalf("mixed units %+v", partial)
	}
	unlimited := classify(200, []byte(`{"data":{"total_available":-100,"total_used":110,"total_granted":10,"unlimited_quota":true}}`), "token")
	unlimited = normalizeToken(unlimited, hit{})
	q := interpret(round{token: unlimited, tokenURL: "token"}, 1, 1)
	if q.Status != "available" || !q.Unlimited || q.RemainingUSD != nil || q.TotalUSD != nil {
		t.Fatalf("unlimited %+v", q)
	}
	bad := classify(200, []byte(`{"data":{"total_available":0,"total_used":11,"total_granted":10}}`), "token")
	bad = normalizeToken(bad, hit{})
	if bad.kind != "invalid_data" {
		t.Fatalf("accepted inconsistent %+v", bad)
	}
}
func TestQuotaClassifications(t *testing.T) {
	for _, tt := range []struct {
		status               int
		body, expected, kind string
	}{{401, `{}`, "token", "unauthorized"}, {403, "<html>blocked</html>", "grants", "unsupported"}, {404, `{}`, "grants", "not_found"}, {408, `{}`, "grants", "error"}, {200, `{"success":false,"message":"invalid token"}`, "token", "unauthorized"}, {200, `{"success":false,"message":"internal"}`, "token", "error"}, {200, `{"data":{}}`, "token", "invalid_data"}, {200, `not-json`, "token", "unsupported"}, {200, `{"error":{"message":"failed"},"total_available":10}`, "grants", "error"}} {
		if got := classify(tt.status, []byte(tt.body), tt.expected); got.kind != tt.kind {
			t.Fatalf("classify %s %+v", tt.body, got)
		}
	}
}
func TestRoundSourceAndDiagnosticPrecedence(t *testing.T) {
	g := classify(200, []byte(`{"total_available":"8","total_used":2,"total_granted":10}`), "grants")
	sub := classify(200, []byte(`{"hard_limit_usd":20,"access_until":1767225600}`), "subscription")
	usage := classify(200, []byte(`{"total_usage":500}`), "usage")
	token := normalizeToken(classify(200, []byte(`{"data":{"display":{"remaining":7,"used":3,"total":10,"unit":"CNY"}}}`), "token"), hit{})
	q := interpret(round{grants: g, subscription: sub, usage: usage, token: token, tokenURL: "token"}, 1, 2)
	if q.Source == nil || *q.Source != "token_usage" {
		t.Fatalf("source %+v", q)
	}
	q = interpret(round{grants: g, subscription: sub, usage: usage}, 1, 2)
	if *q.Source != "credit_grants" || *q.RemainingUSD != 8 || *q.ExpiresAt != 1767225600 {
		t.Fatalf("grants %+v", q)
	}
	q = interpret(round{subscription: sub, usage: usage}, 1, 2)
	if *q.Source != "subscription_usage" || *q.UsedUSD != 5 || *q.RemainingUSD != 15 {
		t.Fatalf("usage %+v", q)
	}
	q = interpret(round{subscription: sub}, 1, 2)
	if *q.Source != "subscription_only" || q.Unlimited {
		t.Fatalf("subscription %+v", q)
	}
	q = interpret(round{usage: usage}, 1, 2)
	if *q.Source != "usage_only" {
		t.Fatalf("usage-only %+v", q)
	}
	invalid := quiet("invalid_data", nil, nil, 1, 1)
	unauthorized := quiet("unauthorized", nil, nil, 1, 1)
	if combine(invalid, unauthorized).Status != "invalid_data" {
		t.Fatal("diagnostic priority")
	}
}
func TestBillingDateRangeAndPrivateRoots(t *testing.T) {
	_, _, usage := billingURLs("https://example.test/private/v1", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	if usage != "https://example.test/private/v1/dashboard/billing/usage?start_date=2026-12-01&end_date=2027-01-01" {
		t.Fatal(usage)
	}
	bases := publicBases("https://example.test/private/v1")
	if len(bases) != 2 || bases[0] != "https://example.test/private" || bases[1] != "https://example.test" {
		t.Fatal(bases)
	}
}
