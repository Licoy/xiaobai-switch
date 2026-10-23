package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"testing"
	"time"
)

func TestQuotaUnknownUnlimitedAndCacheIdentity(t *testing.T) {
	s := testShell(t)
	site := fixtureSite(t, s)
	s.recordQuota(site, domain.SiteQuota{Status: "available", Unlimited: true})
	draw := func(c *ui.Context) { s.quotaView(c, site) }
	tester := ui.NewTester(draw, 640, 240)
	if !tester.HasText(s.text("unlimited")) {
		t.Fatal("unlimited quota appeared as zero")
	}
	s.recordQuota(site, domain.SiteQuota{Status: "available"})
	tester.Frame()
	if !tester.HasText(s.text("unknown")) {
		t.Fatal("missing quota appeared as zero")
	}
	amount := 42.0
	s.recordQuota(site, domain.SiteQuota{Status: "available", RemainingUSD: &amount})
	s.recordQuota(site, domain.SiteQuota{Status: "error"})
	tester.Frame()
	if !hasExact(tester, "42") || !tester.HasText(s.text("quota_error")) {
		t.Fatal("last successful quota or latest error was lost")
	}
	original := site
	site.ActiveAPIKeyID = pointer("different-key")
	tester.Frame()
	if hasExact(tester, "42") {
		t.Fatal("quota leaked across key change")
	}
	site = original
	site.BaseURL = "https://different.test"
	tester.Frame()
	if hasExact(tester, "42") {
		t.Fatal("quota leaked across route change")
	}
}

func TestQuotaRevisionInvalidatesOnlyCredentialChanges(t *testing.T) {
	s := testShell(t)
	site := fixtureSite(t, s)
	oldKey := quotaKey(site)
	oldProbe := probeResultKey(site, "model")
	name := "Renamed"
	notes := "note"
	if _, err := s.Service.UpdateSite(s.Context, site.ID, domain.UpdateSiteInput{Name: &name, Notes: &notes}); err != nil {
		t.Fatal(err)
	}
	fresh, err := s.Store.GetSite(site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if quotaKey(fresh) != oldKey {
		t.Fatal("metadata discarded balance cache")
	}
	secret := "fixture-not-a-real-key-new"
	if _, err = s.Store.UpdateSiteAPIKey(site.ID, value(site.ActiveAPIKeyID), domain.UpdateSiteApiKeyInput{APIKey: &secret}); err != nil {
		t.Fatal(err)
	}
	fresh, err = s.Store.GetSite(site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if quotaKey(fresh) == oldKey || probeResultKey(fresh, "model") == oldProbe {
		t.Fatal("secret replacement reused cached result")
	}
}

func TestExpiredQuotaRemainsVisible(t *testing.T) {
	s := testShell(t)
	site := fixtureSite(t, s)
	s.recordQuota(site, domain.SiteQuota{Status: "available", ExpiresAt: pointer(int64(1700000000))})
	tester := ui.NewTester(func(c *ui.Context) { s.quotaView(c, site) }, 640, 240)
	if !tester.HasText(s.text("quota_expired") + ": " + time.Unix(1700000000, 0).Format("2006-01-02")) {
		t.Fatal("expired quota date hidden")
	}
}

func hasExact(tester *ui.Tester, text string) bool {
	_, ok := tester.Find(text)
	return ok
}
