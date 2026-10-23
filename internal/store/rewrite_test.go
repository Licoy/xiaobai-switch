package store

import (
	"github.com/licoy/xiaobai-switch/internal/domain"
	"testing"
)

func TestCommitRewritePreservesPresetAndSupportsExactRecordLookup(t *testing.T) {
	s := openTest(t)
	site := createTestSite(t, s)
	key := *site.ActiveAPIKeyID
	preset := domain.TargetPreset{SiteID: site.ID, APIKeyID: key, Target: domain.Codex, Options: domain.ApplyRequest{ModelID: "saved-custom-options"}}
	if e := s.SaveTargetPreset(preset); e != nil {
		t.Fatal(e)
	}
	id := domain.NewID()
	record := domain.ApplyRecordDTO{ID: id, SiteID: &site.ID, Target: "codex", Status: "applied", ModelID: "rewritten-model", SiteAPIKeyID: &key}
	binding := domain.TargetBinding{Target: domain.Codex, SiteID: &site.ID, ModelID: record.ModelID, ApplyRecordID: &id, APIKey: domain.BindingApiKeySnapshot{ID: &key}}
	if e := s.CommitRewrite(binding, record, domain.TouchedKeys{}); e != nil {
		t.Fatal(e)
	}
	stored, e := s.GetRecord(id)
	if e != nil || stored == nil || stored.ModelID != "rewritten-model" {
		t.Fatalf("record %+v %v", stored, e)
	}
	missing, e := s.GetRecord("missing")
	if e != nil || missing != nil {
		t.Fatalf("missing %+v %v", missing, e)
	}
	saved, e := s.GetTargetPreset(site.ID, key, domain.Codex)
	if e != nil || saved == nil || saved.Options.ModelID != "saved-custom-options" {
		t.Fatalf("rewrite replaced preset %+v %v", saved, e)
	}
	nextID := domain.NewID()
	record.ID = nextID
	missingSite := "missing-site"
	binding.SiteID = &missingSite
	if e = s.CommitRewrite(binding, record, domain.TouchedKeys{}); e == nil {
		t.Fatal("accepted invalid binding")
	}
	stored, e = s.GetRecord(nextID)
	if e != nil || stored != nil {
		t.Fatalf("partial rewrite record %+v %v", stored, e)
	}
}
func TestEmptyWebDAVPasswordPreservesEncryptedSecret(t *testing.T) {
	s := openTest(t)
	password := "private-password"
	in := domain.SaveWebDavConfigInput{BaseURL: "https://dav.test", Username: "user", Password: &password, RemotePath: "backups", SyncIntervalMinutes: 60, MaxRemoteBackups: 10}
	if _, e := s.SaveWebDAVConfig(in); e != nil {
		t.Fatal(e)
	}
	empty := ""
	in.Password = &empty
	if _, e := s.SaveWebDAVConfig(in); e != nil {
		t.Fatal(e)
	}
	got, e := s.GetWebDAVPassword()
	if e != nil || got != password {
		t.Fatal("empty password changed saved secret", e)
	}
}
