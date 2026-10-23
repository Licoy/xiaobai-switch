package models

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T) *http.Client {
	t.Helper()
	settings := domain.DefaultSettings()
	settings.ProxyMode = "none"
	c, e := netx.NewClient(settings, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestFetchProtocolsHeadersAndKeyIsolation(t *testing.T) {
	for _, protocol := range []string{"openai_compatible", "anthropic"} {
		t.Run(protocol, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/private/v1/models" {
					t.Errorf("path %s", r.URL.Path)
				}
				if protocol == "anthropic" {
					if r.Header.Get("x-api-key") != "secret-api-key" || r.Header.Get("anthropic-version") != "2023-06-01" {
						t.Error("Anthropic headers")
					}
				} else if r.Header.Get("Authorization") != "Bearer secret-api-key" {
					t.Error("Bearer header")
				}
				io.WriteString(w, `{"data":[{"id":"a","owned_by":"provider","display_name":"A"}]}`)
			}))
			defer srv.Close()
			keyID := "key-a"
			site := domain.SiteDTO{ID: "site-a", BaseURL: srv.URL + "/private", Protocol: protocol, ActiveAPIKeyID: &keyID}
			result, e := Fetch(context.Background(), testClient(t), site, "secret-api-key")
			if e != nil || len(result.Models) != 1 || result.APIKeyID != keyID || result.Models[0].APIKeyID != keyID || result.Models[0].SiteID != site.ID {
				t.Fatalf("fetch %+v %v", result, e)
			}
			if protocol == "anthropic" && result.Models[0].DisplayName != "A" {
				t.Fatal("lost display name")
			}
		})
	}
}
func TestFetchRejectsMissingDataHTTPErrorAndOversize(t *testing.T) {
	for _, tt := range []struct {
		status     int
		body, code string
	}{{200, `{}`, "invalid_response"}, {200, `{"data":[{"id":"a"},{"id":"a"}]}`, "invalid_response"}, {401, `{"error":{"message":"invalid secret-api-key"}}`, "unauthorized"}, {404, `{}`, "not_found"}, {200, strings.Repeat(" ", maxModelsBody+1), "response_too_large"}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status); io.WriteString(w, tt.body) }))
		_, e := Fetch(context.Background(), testClient(t), domain.SiteDTO{BaseURL: srv.URL, Protocol: "openai_compatible"}, "secret-api-key")
		srv.Close()
		var app *domain.AppError
		if !errors.As(e, &app) || app.Code != tt.code || strings.Contains(app.Message, "secret-api-key") {
			t.Fatalf("error %+v", e)
		}
	}
}
func TestProbeCompatibilityRetryAndRequestBody(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Error(r.URL.Path)
		}
		var body map[string]any
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		n := calls.Add(1)
		if n == 1 {
			if body["max_tokens"] != float64(16) || body["stream"] != false {
				t.Error("initial body")
			}
			w.WriteHeader(400)
			io.WriteString(w, `{"error":{"message":"use max_completion_tokens"}}`)
			return
		}
		if _, ok := body["max_tokens"]; ok || body["max_completion_tokens"] != float64(16) {
			t.Error("retry body")
		}
		io.WriteString(w, `{"choices":[{"message":{"content":""}}]}`)
	}))
	defer srv.Close()
	r, e := Probe(context.Background(), testClient(t), domain.SiteDTO{BaseURL: srv.URL, Protocol: "openai_compatible"}, "secret-api-key", "o3")
	if e != nil || !r.Ok || calls.Load() != 2 {
		t.Fatalf("probe %+v %v calls=%d", r, e, calls.Load())
	}
}
func TestProbeAnthropicAndRedaction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anthropic/v1/messages" || r.Header.Get("x-api-key") != "secret-api-key" {
			t.Error("Anthropic request")
		}
		w.WriteHeader(429)
		io.WriteString(w, `{"error":{"message":"rate limit secret-api-key"}}`)
	}))
	defer srv.Close()
	r, e := Probe(context.Background(), testClient(t), domain.SiteDTO{BaseURL: srv.URL + "/anthropic", Protocol: "anthropic"}, "secret-api-key", "a")
	if e != nil || r.Ok || r.Error == nil || strings.Contains(*r.Error, "secret-api-key") {
		t.Fatalf("redaction %+v %v", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Probe(ctx, testClient(t), domain.SiteDTO{BaseURL: srv.URL, Protocol: "anthropic"}, "secret-api-key", "a"); e == nil {
		t.Fatal("ignored context cancellation")
	}
}
