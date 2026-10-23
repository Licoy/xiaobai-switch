package appbackup

import (
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func prepared(t *testing.T) (string, string, []operation) {
	t.Helper()
	root, s, m, _ := fixture(t)
	created, e := m.Create("manual", 30)
	if e != nil {
		t.Fatal(e)
	}
	if e = QueuePending(created.Path, root); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	pending := filepath.Join(root, pendingName)
	ops, e := prepareOperations(root, pending)
	if e != nil {
		t.Fatal(e)
	}
	j := applyJournal{}
	for _, op := range ops {
		j.TargetExisted = append(j.TargetExisted, op.existed)
	}
	raw, e := json.Marshal(j)
	if e != nil {
		t.Fatal(e)
	}
	if e = fileops.AtomicWrite(filepath.Join(pending, journalName), raw); e != nil {
		t.Fatal(e)
	}
	return root, pending, ops
}
func TestOldJournalInterruptedPublishRecovers(t *testing.T) {
	root, _, ops := prepared(t)
	if e := publishOperations(ops[:2]); e != nil {
		t.Fatal(e)
	}
	out, e := ApplyPending(root)
	if e != nil || out == nil || out.Status != "applied" {
		t.Fatalf("recover %+v %v", out, e)
	}
	s, e := store.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	sites, e := s.ListSites()
	if e != nil || len(sites) != 1 {
		t.Fatalf("corrupt DB %+v %v", sites, e)
	}
	if _, e = s.GetSiteAPIKey(sites[0].ID, ""); e != nil {
		t.Fatal(e)
	}
}
func TestCommittedCleanupFailureStaysAppliedWithVisibleWarning(t *testing.T) {
	var e error
	root, pending, ops := prepared(t)
	if e := publishOperations(ops); e != nil {
		t.Fatal(e)
	}
	if e = fileops.AtomicWrite(filepath.Join(pending, "committed"), nil); e != nil {
		t.Fatal(e)
	}
	blocked := ops[1].rollback
	if e = os.Mkdir(blocked, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(blocked, "keep"), []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	out, e := ApplyPending(root)
	if e != nil || out == nil || out.Status != "applied" || !strings.Contains(out.Message, "cleanup") {
		t.Fatalf("committed status %+v %v", out, e)
	}
	if _, e = os.Stat(pending); e != nil {
		t.Fatal("removed pending recovery data")
	}
	s, e := store.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	quarantined, _ := filepath.Glob(filepath.Join(root, ".failed-restore-*"))
	if len(quarantined) != 0 {
		t.Fatal("quarantined committed data")
	}
}
func TestInvalidJournalDoesNotBecomeMissing(t *testing.T) {
	var e error
	root, pending, _ := prepared(t)
	if e := os.Remove(filepath.Join(pending, journalName)); e != nil {
		t.Fatal(e)
	}
	if e = os.Mkdir(filepath.Join(pending, journalName), 0700); e != nil {
		t.Fatal(e)
	}
	if _, e = ApplyPending(root); e == nil {
		t.Fatal("treated invalid journal as absent")
	}
}
