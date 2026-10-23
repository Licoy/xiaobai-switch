package service

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/clidetect"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestStartupRecoversUncommittedFileDatabaseBoundary(t *testing.T) {
	s, st, site := fixtureService(t)
	v, e := s.load(applyRequest(site, domain.ClaudeCode))
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.build(v)
	if e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(s.options.Root, "backups", "claude_code", "100")
	if e = os.MkdirAll(filepath.Dir(dir), 0700); e != nil {
		t.Fatal(e)
	}
	recordID := "uncommitted-record"
	if e = saveServiceJournal(dir+".service.json", serviceJournal{Target: domain.ClaudeCode, SiteID: site.ID, RecordID: recordID, Operation: "apply", Phase: "prepared"}); e != nil {
		t.Fatal(e)
	}
	if _, e = adapters.ExecuteHeld(p, dir); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(p.Changes[0].Path); e != nil {
		t.Fatal(e)
	}
	empty := clidetect.ProbeEnv{}
	reopened, e := New(st, Options{Root: s.options.Root, Home: s.options.Home, CLIEnv: &empty})
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if _, e = os.Stat(p.Changes[0].Path); !os.IsNotExist(e) {
		t.Fatal("startup left config without committed binding")
	}
	binding, e := st.GetBinding(domain.ClaudeCode)
	if e != nil || binding != nil {
		t.Fatal("recovery fabricated binding")
	}
}
func TestStartupKeepsFilesWhenDatabaseCommitAlreadySucceeded(t *testing.T) {
	s, st, site := fixtureService(t)
	result := applyFixture(t, s, site, domain.ClaudeCode)
	dir := result.Results[0].BackupPaths[0]
	binding, e := st.GetBinding(domain.ClaudeCode)
	if e != nil {
		t.Fatal(e)
	}
	if e = saveServiceJournal(dir+".service.json", serviceJournal{Target: domain.ClaudeCode, SiteID: site.ID, RecordID: *binding.ApplyRecordID, Operation: "apply", Phase: "prepared"}); e != nil {
		t.Fatal(e)
	}
	empty := clidetect.ProbeEnv{}
	reopened, e := New(st, Options{Root: s.options.Root, Home: s.options.Home, CLIEnv: &empty})
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	states, e := reopened.Status(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, state := range states {
		if state.Target == domain.ClaudeCode && state.Status != domain.Applied {
			t.Fatal("committed application rolled back on restart")
		}
	}
}
func TestRevertOfficialBackupRestoreAndPruning(t *testing.T) {
	s, st, site := fixtureService(t)
	settings, _ := st.GetSettings()
	settings.MaxBackupCopies = 2
	if e := s.SaveSettings(context.Background(), settings); e != nil {
		t.Fatal(e)
	}
	first := applyFixture(t, s, site, domain.ClaudeCode)
	if e := s.Revert(context.Background(), domain.ClaudeCode); e != nil {
		t.Fatal(e)
	}
	binding, e := st.GetBinding(domain.ClaudeCode)
	if e != nil || binding != nil {
		t.Fatal("revert left binding")
	}
	applyFixture(t, s, site, domain.ClaudeCode)
	backups, e := s.ListBackups(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(backups) > 2 {
		t.Fatal("backup retention exceeded")
	}
	for _, b := range backups {
		if _, e = s.PreviewBackup(context.Background(), b.ID); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.RestoreOfficial(context.Background(), domain.ClaudeCode); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(first.Results[0].BackupPaths[0]); !os.IsNotExist(e) {
		t.Fatal("old backup not pruned")
	}
}
