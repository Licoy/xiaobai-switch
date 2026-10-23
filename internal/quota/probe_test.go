package quota

import (
	"context"
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

func client(t *testing.T) *http.Client {
	t.Helper()
	s := domain.DefaultSettings()
	s.ProxyMode = "none"
	c, e := netx.NewClient(s, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestProbePrioritizesPublicTokenAndDoesNotAuthenticateStatus(t *testing.T) {
	var public atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/status") {
			public.Add(1)
			if r.Header.Get("Authorization") != "" {
				t.Error("authenticated public metadata")
			}
			io.WriteString(w, `{"success":true,"data":{"quota_display_type":"USD","quota_per_unit":500000}}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret-api-key" {
			t.Error("missing bearer")
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/usage/token"):
			io.WriteString(w, `{"code":true,"data":{"total_available":1000000,"total_used":500000,"total_granted":1500000}}`)
		case strings.HasSuffix(r.URL.Path, "/credit_grants"):
			io.WriteString(w, `{"total_available":8,"total_used":2,"total_granted":10}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	q, e := Probe(context.Background(), client(t), domain.SiteDTO{BaseURL: srv.URL + "/private"}, "secret-api-key")
	if e != nil || q.Status != "available" || q.Source == nil || *q.Source != "token_usage" || q.Unit == nil || *q.Unit != "USD" || *q.RemainingUSD != 2 || public.Load() != 1 {
		t.Fatalf("quota %+v %v", q, e)
	}
}
func TestProbeFallbackAndUnauthorizedDiagnostics(t *testing.T) {
	for _, unauthorized := range []bool{false, true} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/v1/") {
				if unauthorized {
					w.WriteHeader(401)
					return
				}
				if strings.HasSuffix(r.URL.Path, "subscription") {
					io.WriteString(w, `{"hard_limit_usd":20}`)
					return
				}
				if strings.HasSuffix(r.URL.Path, "usage") {
					io.WriteString(w, `{"total_usage":2500}`)
					return
				}
			}
			if !unauthorized && r.URL.Path == "/dashboard/billing/credit_grants" {
				io.WriteString(w, `{"total_available":8,"total_used":2,"total_granted":10}`)
				return
			}
			w.WriteHeader(404)
		}))
		q, e := Probe(context.Background(), client(t), domain.SiteDTO{BaseURL: srv.URL}, "secret-api-key")
		srv.Close()
		if e != nil {
			t.Fatal(e)
		}
		if unauthorized {
			if q.Status != "unauthorized" {
				t.Fatalf("unauthorized %+v", q)
			}
		} else if q.Status != "available" || q.Source == nil || *q.Source != "credit_grants" || *q.RemainingUSD != 8 {
			t.Fatalf("fallback %+v", q)
		}
	}
}
func TestRootPublicTokenRequiresStatusSupportAndCancellation(t *testing.T) {
	var rootToken atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/usage/token" {
			rootToken.Add(1)
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()
	q, e := Probe(context.Background(), client(t), domain.SiteDTO{BaseURL: srv.URL + "/private"}, "secret-api-key")
	if e != nil || q.Status != "unsupported" || rootToken.Load() != 0 {
		t.Fatalf("unguarded origin token probe %+v %v", q, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Probe(ctx, client(t), domain.SiteDTO{BaseURL: srv.URL}, "secret-api-key"); e == nil {
		t.Fatal("ignored cancellation")
	}
	q, e = Probe(context.Background(), nil, domain.SiteDTO{}, "")
	if e != nil || q.Status != "unsupported" {
		t.Fatalf("empty key %+v %v", q, e)
	}
}
