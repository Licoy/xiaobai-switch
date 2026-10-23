package deeplink

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/store"
	"net/url"
	"strings"
	"testing"
)

func TestParseRoutesAliasesAndCapabilities(t *testing.T) {
	raw := "xiaobaiswitch:///sites?name=Relay&baseurl=https://first.test/private&baseurls=https://second.test|https://third.test&apikey=secret-api-key&type=openai&keyname=Personal&codex-compact=1&claude-extra=yes"
	in, e := Parse(raw)
	if e != nil || len(in.BaseURLs) != 3 || in.BaseURLs[0] != "https://first.test/private" || in.Protocol == nil || *in.Protocol != "openai_compatible" || in.KeyName == nil || *in.KeyName != "Personal" || in.Capabilities == nil || !(*in.Capabilities)["codex-compact"] || (*in.Capabilities)["codex-search"] || !(*in.Capabilities)["claude-extra"] {
		t.Fatalf("parse %+v %v", in, e)
	}
	in, e = Parse("xiaobaiswitch://sites?name=Relay&baseurls=https://api.test")
	if e != nil || in.APIKey != "" || in.Capabilities != nil {
		t.Fatalf("optional key %+v %v", in, e)
	}
	if _, e = Validate(in); e == nil {
		t.Fatal("import accepted missing key")
	}
}
func TestRejectInvalidDeepLinks(t *testing.T) {
	for _, raw := range []string{"https://api.test", "xiaobaiswitch://apply?name=X", "xiaobaiswitch://sites?name=&baseurl=https://api.test", "xiaobaiswitch://sites?name=X&baseurl=ftp://api.test", "xiaobaiswitch://sites?name=X&baseurl=https://user:secret@api.test", "xiaobaiswitch://sites?name=X&baseurl=https://api.test&protocol=gemini", "xiaobaiswitch://sites?name=X&baseurl=https://api.test&apikey=bad%0Akey", "xiaobaiswitch://sites?name=" + url.QueryEscape(strings.Repeat("名", 129)) + "&baseurl=https://api.test"} {
		if _, e := Parse(raw); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestImportReusesSiteAndKeyWithoutChangingActiveKey(t *testing.T) {
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	input := domain.DeepLinkSiteImportInput{Name: "Original", BaseURLs: []string{"https://a.test", "https://b.test"}, APIKey: "secret-api-key"}
	created, e := Import(s, input)
	if e != nil || !created.Created || !created.ActivatedAPIKey {
		t.Fatalf("create %+v %v", created, e)
	}
	active := *created.Site.ActiveAPIKeyID
	input.Name = "Renamed"
	input.BaseURLs = []string{"https://b.test", "https://a.test"}
	input.APIKey = "new-secret-key"
	caps := domain.SiteCapabilities{"codex-search": true, "claude-extra": true}
	input.Capabilities = &caps
	added, e := Import(s, input)
	if e != nil || added.Created || !added.AddedAPIKey || added.ActivatedAPIKey || *added.Site.ActiveAPIKeyID != active {
		t.Fatalf("add %+v %v", added, e)
	}
	same, e := Import(s, input)
	if e != nil || !same.ReusedAPIKey || same.AddedAPIKey || len(same.Site.APIKeys) != 2 || same.Site.Name != "Renamed" {
		t.Fatalf("reuse %+v %v", same, e)
	}
	input.APIKey = "third-secret-key"
	input.Name = "MustRollback"
	label := "K 1"
	input.KeyName = &label
	if _, e = Import(s, input); e == nil {
		t.Fatal("accepted duplicate label")
	}
	unchanged, e := s.GetSite(created.Site.ID)
	if e != nil || unchanged.Name != "Renamed" || len(unchanged.APIKeys) != 2 {
		t.Fatalf("partial import %+v %v", unchanged, e)
	}
}

func TestEmptyProtocolDoesNotUseTypeAlias(t *testing.T) {
	v, e := Parse("xiaobaiswitch://sites?name=X&baseurls=https://api.test&protocol=&type=anthropic")
	if e != nil || v.Protocol == nil || *v.Protocol != "openai_compatible" {
		t.Fatalf("query precedence %+v %v", v, e)
	}
}
