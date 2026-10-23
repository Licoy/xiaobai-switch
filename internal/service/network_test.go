package service

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchModelsPreservesTargetAndOldCacheOnFailure(t *testing.T) {
	s, st, site := fixtureService(t)
	applyFixture(t, s, site, domain.ClaudeCode)
	binding, _ := st.GetBinding(domain.ClaudeCode)
	path := binding.ManagedPaths[0]
	before, _ := os.ReadFile(path)
	bad := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-private-key-long" {
			t.Error("wrong request key")
		}
		if bad {
			http.Error(w, "denied fixture-private-key-long", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"from-network"}]}`))
	}))
	defer server.Close()
	if _, e := st.UpdateSite(site.ID, domain.UpdateSiteInput{BaseURL: &server.URL}); e != nil {
		t.Fatal(e)
	}
	f, e := s.FetchModels(context.Background(), site.ID, *site.ActiveAPIKeyID)
	if e != nil || len(f.Models) != 2 || f.Models[0].ModelID != "fixture-model" || f.Models[1].ModelID != "from-network" {
		t.Fatalf("fetch %v %v", f, e)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("fetch changed applied target")
	}
	bad = true
	if _, e = s.FetchModels(context.Background(), site.ID, *site.ActiveAPIKeyID); e == nil || strings.Contains(e.Error(), "fixture-private-key-long") {
		t.Fatalf("failure hidden or leaked key %v", e)
	}
	cached, e := st.ListModelsForKey(site.ID, *site.ActiveAPIKeyID)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, m := range cached {
		found = found || m.ModelID == "from-network"
	}
	if !found {
		t.Fatal("failed fetch erased prior cache")
	}
}
func TestFetchRejectsConcurrentKeyChangeWithoutCachePollution(t *testing.T) {
	s, st, site := fixtureService(t)
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.Write([]byte(`{"data":[{"id":"old-key-result"}]}`))
	}))
	defer server.Close()
	if _, e := st.UpdateSite(site.ID, domain.UpdateSiteInput{BaseURL: &server.URL}); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { _, e := s.FetchModels(context.Background(), site.ID, *site.ActiveAPIKeyID); done <- e }()
	<-started
	secret := "rotated-secret"
	if _, e := st.UpdateSiteAPIKey(site.ID, *site.ActiveAPIKeyID, domain.UpdateSiteApiKeyInput{APIKey: &secret}); e != nil {
		t.Fatal(e)
	}
	close(release)
	if e := <-done; e == nil {
		t.Fatal("stale network response accepted")
	}
	models, e := st.ListModelsForKey(site.ID, *site.ActiveAPIKeyID)
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range models {
		if m.ModelID == "old-key-result" {
			t.Fatal("old key polluted rotated key cache")
		}
	}
}
func TestSwitchKeyOnlySynchronizesAfterSuccessfulFetch(t *testing.T) {
	s, st, site := fixtureService(t)
	applyFixture(t, s, site, domain.ClaudeCode)
	b, _ := st.GetBinding(domain.ClaudeCode)
	path := b.ManagedPaths[0]
	before, _ := os.ReadFile(path)
	bad := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bad {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Write([]byte(`{"data":[{"id":"fixture-model"}]}`))
	}))
	defer server.Close()
	if _, e := st.UpdateSite(site.ID, domain.UpdateSiteInput{BaseURL: &server.URL}); e != nil {
		t.Fatal(e)
	}
	site, e := st.AddSiteAPIKey(site.ID, domain.AddSiteApiKeyInput{APIKey: "new-key-secret"})
	if e != nil {
		t.Fatal(e)
	}
	newKey := ""
	for _, k := range site.APIKeys {
		if k.ID != *site.ActiveAPIKeyID {
			newKey = k.ID
		}
	}
	out, e := s.SwitchKey(context.Background(), site.ID, newKey)
	if e == nil || out.Fetch.Ok {
		t.Fatal("failed fetch reported success")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("failed key fetch rewrote target")
	}
	current, _ := st.GetSite(site.ID)
	if *current.ActiveAPIKeyID != newKey {
		t.Fatal("key was not activated")
	}
	bad = false
	out, e = s.SwitchKey(context.Background(), site.ID, newKey)
	if e != nil || len(out.Results) != 1 || !out.Results[0].Ok {
		t.Fatalf("switch %v %v", out, e)
	}
	after, _ = os.ReadFile(path)
	if !strings.Contains(string(after), "new-key-secret") {
		t.Fatal("successful switch did not synchronize target")
	}
}
func TestRouteRewritePreservesLaterUserModel(t *testing.T) {
	s, st, site := fixtureService(t)
	urls := []string{"https://example.test", "https://route.test"}
	if _, e := st.UpdateSite(site.ID, domain.UpdateSiteInput{BaseURLs: &urls}); e != nil {
		t.Fatal(e)
	}
	applyFixture(t, s, site, domain.ClaudeCode)
	settings, _ := st.GetSettings()
	home, _ := s.targetHome(domain.ClaudeCode, settings)
	path := filepath.Join(home, "settings.json")
	raw, _ := os.ReadFile(path)
	raw = []byte(strings.Replace(string(raw), "fixture-model", "user-later-model", 1))
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	result, e := s.SwitchRoute(context.Background(), site.ID, "https://route.test")
	if e != nil || len(result.Results) != 1 || !result.Results[0].Ok {
		t.Fatalf("route %v %v", result, e)
	}
	raw, _ = os.ReadFile(path)
	if !strings.Contains(string(raw), "user-later-model") || !strings.Contains(string(raw), "https://route.test") {
		t.Fatal("route rewrite changed unrelated model")
	}
}
func TestUpdateSiteRefreshesChangedBoundNonactiveKeyOnly(t *testing.T) {
	s, st, site := fixtureService(t)
	active := *site.ActiveAPIKeyID
	site, e := st.AddSiteAPIKey(site.ID, domain.AddSiteApiKeyInput{APIKey: "nonactive-old"})
	if e != nil {
		t.Fatal(e)
	}
	other := ""
	for _, k := range site.APIKeys {
		if k.ID != active {
			other = k.ID
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer nonactive-new" {
			t.Error("refreshed unchanged active key instead of changed bound key")
		}
		w.Write([]byte(`{"data":[{"id":"fixture-model"}]}`))
	}))
	defer server.Close()
	if _, e = st.UpdateSite(site.ID, domain.UpdateSiteInput{BaseURL: &server.URL}); e != nil {
		t.Fatal(e)
	}
	if e = st.ReplaceModels(site.ID, other, []domain.SiteModelDTO{{ModelID: "fixture-model", DisplayName: "fixture-model"}}); e != nil {
		t.Fatal(e)
	}
	req := applyRequest(site, domain.ClaudeCode)
	req.APIKeyID = other
	p, e := s.PrepareApply(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Apply(context.Background(), p.ID); e != nil {
		t.Fatal(e)
	}
	items := []domain.UpsertSiteApiKeyInput{{ID: &active, APIKey: "fixture-private-key-long"}, {ID: &other, APIKey: "nonactive-new"}}
	if _, e = s.UpdateSite(context.Background(), site.ID, domain.UpdateSiteInput{APIKeys: &items}); e != nil {
		t.Fatal(e)
	}
	b, _ := st.GetBinding(domain.ClaudeCode)
	raw, _ := os.ReadFile(b.ManagedPaths[0])
	if !strings.Contains(string(raw), "nonactive-new") {
		t.Fatal("bound nonactive key was not rewritten")
	}
	current, _ := st.GetSite(site.ID)
	if *current.ActiveAPIKeyID != active {
		t.Fatal("editing nonactive key changed active key")
	}
}
