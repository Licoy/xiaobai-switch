package fileops

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransactionCommitBackupAndPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "auth.json")
	if e := os.WriteFile(path, []byte("old"), 0644); e != nil {
		t.Fatal(e)
	}
	change, e := Snapshot(path, []byte("new"))
	if e != nil {
		t.Fatal(e)
	}
	backup := filepath.Join(root, "backup")
	r, e := Execute([]Change{change}, backup)
	if e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(path)
	if e != nil || string(got) != "new" {
		t.Fatalf("live mismatch %v", e)
	}
	old, e := os.ReadFile(filepath.Join(backup, "000.bak"))
	if e != nil || string(old) != "old" {
		t.Fatalf("backup mismatch %v", e)
	}
	raw, _ := os.ReadFile(r.Journal)
	var j journal
	if e = json.Unmarshal(raw, &j); e != nil || j.State != "committed" {
		t.Fatal("missing committed journal")
	}
	assertPrivate(t, path)
	assertPrivate(t, filepath.Join(backup, "000.bak"))
}
func TestPartialFailureRollsBackAndRemovesCreatedFile(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{true: "existing", false: "created"}[existing], func(t *testing.T) {
			root := t.TempDir()
			a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
			if existing {
				if e := os.WriteFile(a, []byte("old-a"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if e := os.WriteFile(b, []byte("old-b"), 0600); e != nil {
				t.Fatal(e)
			}
			ca, _ := Snapshot(a, []byte("new-a"))
			cb, _ := Snapshot(b, []byte("new-b"))
			_, e := execute([]Change{ca, cb}, filepath.Join(root, "backup"), func(path string, content []byte) error {
				if path == b {
					return errors.New("injected write failure")
				}
				return AtomicWrite(path, content)
			})
			if e == nil {
				t.Fatal("expected failure")
			}
			got, exists, e := Read(a)
			if e != nil || exists != existing || (existing && string(got) != "old-a") {
				t.Fatal("first file not restored")
			}
			got, _, e = Read(b)
			if e != nil || string(got) != "old-b" {
				t.Fatal("second file damaged")
			}
		})
	}
}
func TestRejectChangedPreview(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "config")
	c, _ := Snapshot(p, []byte("new"))
	if e := os.WriteFile(p, []byte("external"), 0600); e != nil {
		t.Fatal(e)
	}
	_, e := Execute([]Change{c}, filepath.Join(root, "backup"))
	if e == nil {
		t.Fatal("expected stale preview")
	}
	got, _ := os.ReadFile(p)
	if string(got) != "external" {
		t.Fatal("overwrote external change")
	}
}
func TestExternalChangeBetweenReplacementsIsPreserved(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	ca, _ := Snapshot(a, []byte("a"))
	cb, _ := Snapshot(b, []byte("b"))
	_, e := execute([]Change{ca, cb}, filepath.Join(root, "backup"), func(path string, content []byte) error {
		if e := AtomicWrite(path, content); e != nil {
			return e
		}
		return os.WriteFile(b, []byte("external"), 0600)
	})
	if e == nil {
		t.Fatal("expected conflict")
	}
	got, _ := os.ReadFile(b)
	if string(got) != "external" {
		t.Fatal("external change destroyed")
	}
	if _, e = os.Stat(a); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("first change not rolled back")
	}
}
func TestRecoverInterruptedTransaction(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "live")
	if e := os.WriteFile(p, []byte("new"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := AtomicWrite(filepath.Join(root, "000.bak"), []byte("old")); e != nil {
		t.Fatal(e)
	}
	j := journal{State: "prepared", Files: []entry{{Path: p, Backup: "000.bak", Existed: true, Applied: true}}}
	if e := saveJournal(filepath.Join(root, "transaction.json"), j); e != nil {
		t.Fatal(e)
	}
	if e := Recover(root); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, []byte("old")) {
		t.Fatal("not recovered")
	}
}
func TestBackupFailureLeavesLiveUntouched(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "live")
	if e := os.WriteFile(p, []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	c, _ := Snapshot(p, []byte("new"))
	if _, e := Execute([]Change{c}, filepath.Join(root, "missing", "backup")); e == nil {
		t.Fatal("expected backup directory failure")
	}
	got, _ := os.ReadFile(p)
	if string(got) != "old" {
		t.Fatal("live changed before backup")
	}
}
func TestTargetLocksConflict(t *testing.T) {
	root := t.TempDir()
	one, e := Acquire(root, []string{"codex", "site:abc"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer one.Close()
	two, e := Acquire(root, []string{"codex"}, nil)
	if e == nil {
		two.Close()
		t.Fatal("duplicate lock acquired")
	}
}
func TestLegacyBackupRestoreUsesExplicitDestinations(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(root, "legacy")
	if e := os.Mkdir(backup, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(backup, "settings.json"), []byte(`{"model":"old"}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(backup, ".origins.json"), []byte(`{"settings.json":"/not-authorized/user/config"}`), 0600); e != nil {
		t.Fatal(e)
	}
	files, e := InspectBackup(backup)
	if e != nil || len(files) != 1 {
		t.Fatalf("inspect %v %v", files, e)
	}
	dest := filepath.Join(root, "restored")
	changes, e := BuildRestore(backup, map[string]string{"settings.json": dest})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Execute(changes, filepath.Join(root, "restore-backup")); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(dest)
	if string(raw) != `{"model":"old"}` {
		t.Fatal("legacy backup not restored")
	}
	if _, e = BuildRestore(backup, nil); e == nil {
		t.Fatal("metadata implicitly authorized destination")
	}
}
func TestRollbackFailureRemainsRecoverableAndVisible(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	if e := os.WriteFile(a, []byte("old-a"), 0600); e != nil {
		t.Fatal(e)
	}
	ca, _ := Snapshot(a, []byte("new-a"))
	cb, _ := Snapshot(b, []byte("new-b"))
	backup := filepath.Join(root, "backup")
	_, e := execute([]Change{ca, cb}, backup, func(path string, content []byte) error {
		if path == b {
			if err := os.Remove(filepath.Join(backup, "000.bak")); err != nil {
				return err
			}
			return errors.New("injected second write failure")
		}
		return AtomicWrite(path, content)
	})
	if e == nil || !strings.Contains(e.Error(), "rollback failed") {
		t.Fatalf("rollback failure hidden %v", e)
	}
	raw, _ := os.ReadFile(filepath.Join(backup, "transaction.json"))
	var j journal
	if err := json.Unmarshal(raw, &j); err != nil {
		t.Fatal(err)
	}
	if j.State != "prepared" {
		t.Fatal("failed rollback marked terminal")
	}
	if e = AtomicWrite(filepath.Join(backup, "000.bak"), []byte("old-a")); e != nil {
		t.Fatal(e)
	}
	if e = Recover(backup); e != nil {
		t.Fatal(e)
	}
	raw, _ = os.ReadFile(a)
	if string(raw) != "old-a" {
		t.Fatal("recovery could not retry rollback")
	}
}
func TestRollbackPreservesExternalEditToAlreadyReplacedFile(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	if e := os.WriteFile(a, []byte("old-a"), 0600); e != nil {
		t.Fatal(e)
	}
	ca, _ := Snapshot(a, []byte("new-a"))
	cb, _ := Snapshot(b, []byte("new-b"))
	_, e := execute([]Change{ca, cb}, filepath.Join(root, "backup"), func(path string, content []byte) error {
		if path == b {
			return errors.New("injected second failure")
		}
		if e := AtomicWrite(path, content); e != nil {
			return e
		}
		return os.WriteFile(a, []byte("external-after-write"), 0600)
	})
	if e == nil || !strings.Contains(e.Error(), "rollback_conflict") {
		t.Fatalf("external conflict hidden %v", e)
	}
	got, _ := os.ReadFile(a)
	if string(got) != "external-after-write" {
		t.Fatal("rollback destroyed user's later edit")
	}
}
