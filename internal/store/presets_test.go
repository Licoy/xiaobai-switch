package store

import (
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsMergePreservesUnknownAndMigratesColor(t *testing.T) {
	s := openTest(t)
	_, e := s.db.Exec(`INSERT INTO settings(id,json) VALUES(1,?)`, `{"primaryColor":"#1677ff","future":{"v":42},"startInTray":true}`)
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.MergeSettings(map[string]json.RawMessage{"language": json.RawMessage(`"en-US"`)})
	if e != nil {
		t.Fatal(e)
	}
	if v.PrimaryColor != "#0C76F8" || v.StartInTray || v.LaunchAtLoginHidden == nil || !*v.LaunchAtLoginHidden {
		t.Fatalf("startup/color %+v", v)
	}
	v, e = s.GetSettings()
	if e != nil {
		t.Fatal(e)
	}
	if string(v.Extra["future"]) != `{"v":42}` {
		t.Fatalf("lost unknown %s", v.Extra["future"])
	}
	v.PrimaryColor = "#112233"
	if e = s.SaveSettings(v); e != nil {
		t.Fatal(e)
	}
	v, e = s.GetSettings()
	if e != nil || v.PrimaryColor != "#112233" {
		t.Fatalf("custom color %q %v", v.PrimaryColor, e)
	}
}
func TestThinkingAndTargetPresetSeparation(t *testing.T) {
	s := openTest(t)
	v := createTestSite(t, s)
	key := *v.ActiveAPIKeyID
	for _, target := range []domain.TargetKind{domain.Pi, domain.Prime} {
		level := "medium"
		if target == domain.Prime {
			level = "high"
		}
		p := domain.SiteThinkingPreset{SiteID: v.ID, Target: target, DefaultLevel: &level, Models: map[string]domain.ModelThinkingConfig{"a": {Reasoning: true}}}
		if e := s.SaveThinkingPreset(p); e != nil {
			t.Fatal(e)
		}
	}
	v, e := s.AddSiteAPIKey(v.ID, domain.AddSiteApiKeyInput{APIKey: "second-secret"})
	if e != nil {
		t.Fatal(e)
	}
	second := v.APIKeys[1].ID
	for _, id := range []string{key, second} {
		p := domain.TargetPreset{SiteID: v.ID, APIKeyID: id, Target: domain.Pi, Options: domain.ApplyRequest{ModelID: id}}
		if e = s.SaveTargetPreset(p); e != nil {
			t.Fatal(e)
		}
	}
	p, e := s.GetThinkingPreset(v.ID, domain.Pi)
	if e != nil || p.DefaultLevel == nil || *p.DefaultLevel != "medium" {
		t.Fatalf("Pi preset %+v %v", p, e)
	}
	p, e = s.GetThinkingPreset(v.ID, domain.Prime)
	if e != nil || *p.DefaultLevel != "high" {
		t.Fatalf("Prime preset %+v %v", p, e)
	}
	for _, id := range []string{key, second} {
		p, e := s.GetTargetPreset(v.ID, id, domain.Pi)
		if e != nil || p == nil || p.Options.ModelID != id || p.Options.APIKeyID != id {
			t.Fatalf("key preset %+v %v", p, e)
		}
	}
	if e = s.DeleteSite(v.ID); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"site_thinking_presets", "target_presets"} {
		var n int
		if e = s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatalf("cascade %s %d %v", table, n, e)
		}
	}
}
func TestApplicationCommitAtomicAndDeleteOrphans(t *testing.T) {
	s := openTest(t)
	v := createTestSite(t, s)
	key := *v.ActiveAPIKeyID
	recordID := domain.NewID()
	b := domain.TargetBinding{Target: domain.Codex, SiteID: &v.ID, SiteNameSnapshot: v.Name, ModelID: "a", APIKey: domain.BindingApiKeySnapshot{ID: &key}, ApplyRecordID: &recordID, AppliedAt: now()}
	r := domain.ApplyRecordDTO{ID: recordID, SiteID: &v.ID, Target: "codex", SiteNameSnapshot: v.Name, ModelID: "a", Status: "applied", AppliedAt: b.AppliedAt, SiteAPIKeyID: &key}
	p := domain.TargetPreset{SiteID: v.ID, APIKeyID: key, Target: domain.Codex, Options: domain.ApplyRequest{ModelID: "a"}}
	if e := s.CommitApplication(b, r, domain.TouchedKeys{}, p); e != nil {
		t.Fatal(e)
	}
	binding, e := s.GetBinding(domain.Codex)
	if e != nil || binding == nil || *binding.ApplyRecordID != recordID {
		t.Fatalf("binding %+v %v", binding, e)
	}
	r2 := r
	r2.ID = domain.NewID()
	p.APIKeyID = "wrong-key"
	b.APIKey.ID = &p.APIKeyID
	r2.SiteAPIKeyID = &p.APIKeyID
	if e = s.CommitApplication(b, r2, domain.TouchedKeys{}, p); e == nil {
		t.Fatal("committed invalid FK")
	}
	records, e := s.ListRecords(20)
	if e != nil || len(records) != 1 {
		t.Fatalf("partial commit %+v %v", records, e)
	}
	if e = s.DeleteSite(v.ID); e != nil {
		t.Fatal(e)
	}
	binding, e = s.GetBinding(domain.Codex)
	if e != nil || binding == nil || !binding.Orphan || binding.SiteID != nil {
		t.Fatalf("deleted site binding %+v %v", binding, e)
	}
	records, e = s.ListRecords(20)
	if e != nil || len(records) != 1 || records[0].SiteID != nil {
		t.Fatalf("record snapshot %+v %v", records, e)
	}
}
func TestWebDAVEncryptionAndStatus(t *testing.T) {
	s := openTest(t)
	password := "password-private"
	in := domain.SaveWebDavConfigInput{BaseURL: "https://dav.example.test", Username: "user", Password: &password, RemotePath: "xiaobai-switch", SyncIntervalMinutes: 60, MaxRemoteBackups: 10}
	v, e := s.SaveWebDAVConfig(in)
	if e != nil || !v.HasPassword {
		t.Fatalf("save %+v %v", v, e)
	}
	in.Password = nil
	in.Username = "new"
	if _, e = s.SaveWebDAVConfig(in); e != nil {
		t.Fatal(e)
	}
	plain, e := s.GetWebDAVPassword()
	if e != nil || plain != password {
		t.Fatalf("password %q %v", plain, e)
	}
	var cipher string
	if e = s.db.QueryRow("SELECT password_encrypted FROM webdav_config").Scan(&cipher); e != nil || strings.Contains(cipher, password) {
		t.Fatalf("plaintext secret storage %v", e)
	}
	status, e := s.RecordWebDAVSyncStatus("success", nil, true)
	if e != nil || status.LastSuccessAt == nil {
		t.Fatalf("sync %+v %v", status, e)
	}
	success := *status.LastSuccessAt
	status, e = s.RecordWebDAVSyncStatus("failed", ptr("network"), false)
	if e != nil || status.LastSuccessAt == nil || *status.LastSuccessAt != success || status.Error == nil {
		t.Fatalf("failed sync %+v %v", status, e)
	}
}
func TestV1SnapshotIncludesWAL(t *testing.T) {
	s := openTest(t)
	createTestSite(t, s)
	if _, e := s.db.Exec("PRAGMA journal_mode=WAL; PRAGMA user_version=1;"); e != nil {
		t.Fatal(e)
	}
	v, e := s.CreateSite(domain.CreateSiteInput{Name: "WAL", BaseURL: "https://wal.test", APIKey: "wal-secret"})
	if e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(t.TempDir(), "snapshot.db")
	if e = s.SnapshotTo(dest); e != nil {
		t.Fatal(e)
	}
	copy, e := sql.Open("sqlite", dest)
	if e != nil {
		t.Fatal(e)
	}
	defer copy.Close()
	var name string
	if e = copy.QueryRow("SELECT name FROM sites WHERE id=?", v.ID).Scan(&name); e != nil || name != "WAL" {
		t.Fatalf("WAL lost %s %v", name, e)
	}
	if e = s.SnapshotTo(dest); e == nil {
		t.Fatal("overwrote existing snapshot")
	}
	if _, e = os.Stat(dest); e != nil {
		t.Fatal(e)
	}
}

func TestSettingsFreshSaveStillPreservesUnknown(t *testing.T) {
	s := openTest(t)
	if _, e := s.MergeSettings(map[string]json.RawMessage{"future": json.RawMessage(`{"enabled":true}`)}); e != nil {
		t.Fatal(e)
	}
	if e := s.SaveSettings(domain.DefaultSettings()); e != nil {
		t.Fatal(e)
	}
	v, e := s.GetSettings()
	if e != nil || string(v.Extra["future"]) != `{"enabled":true}` {
		t.Fatalf("fresh save lost unknown field %s %v", v.Extra["future"], e)
	}
}
func TestReorderAndRestoreDatabaseAtomicity(t *testing.T) {
	s := openTest(t)
	one := createTestSite(t, s)
	two, e := s.CreateSite(domain.CreateSiteInput{Name: "Two", BaseURL: "https://two.test", APIKey: "two-secret"})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.ReorderSites([]string{two.ID, one.ID}); e != nil {
		t.Fatal(e)
	}
	if e = s.ReorderSites([]string{one.ID, "unknown"}); e == nil {
		t.Fatal("accepted missing site")
	}
	sites, e := s.ListSites()
	if e != nil || len(sites) != 2 || sites[0].ID != two.ID {
		t.Fatalf("partial reorder %+v %v", sites, e)
	}
	if e = s.ReorderSites([]string{one.ID, one.ID}); e == nil {
		t.Fatal("accepted duplicate site")
	}
	if e = s.ReorderSites([]string{one.ID}); e == nil {
		t.Fatal("accepted incomplete order")
	}
	recordID := domain.NewID()
	r := domain.ApplyRecordDTO{ID: recordID, Target: "pi", SiteID: &one.ID, Status: "applied"}
	if e = s.InsertRecord(r, domain.TouchedKeys{}); e != nil {
		t.Fatal(e)
	}
	if e = s.UpsertBinding(domain.TargetBinding{Target: domain.Codex, SiteID: &one.ID}); e != nil {
		t.Fatal(e)
	}
	if e = s.ClearApplication(domain.Codex, recordID); e == nil {
		t.Fatal("removed another target record")
	}
	b, e := s.GetBinding(domain.Codex)
	if e != nil || b == nil {
		t.Fatalf("binding lost after failure %v", e)
	}
	if e = s.ClearApplication(domain.Codex, ""); e != nil {
		t.Fatal(e)
	}
	b, e = s.GetBinding(domain.Codex)
	if e != nil || b != nil {
		t.Fatalf("binding survived restore %v", e)
	}
}
func TestVersionOneUpgradePreservesData(t *testing.T) {
	root := t.TempDir()
	s, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	v := createTestSite(t, s)
	if _, e = s.db.Exec("DROP TABLE target_presets; PRAGMA user_version=1;"); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var version int
	if e = s.db.QueryRow("PRAGMA user_version").Scan(&version); e != nil || version != 2 {
		t.Fatalf("version %d %v", version, e)
	}
	got, e := s.GetSite(v.ID)
	if e != nil || got.Name != v.Name {
		t.Fatalf("lost v1 site %+v %v", got, e)
	}
	files, e := filepath.Glob(filepath.Join(root, "backups", "app", "pre_migration_v1_*", "xiaobai-switch.db"))
	if e != nil || len(files) != 1 {
		t.Fatalf("v1 snapshot %+v %v", files, e)
	}
}
