package appbackup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) (string, *store.Store, *Manager, domain.SiteDTO) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s, e := store.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	m, e := New(root, s)
	if e != nil {
		t.Fatal(e)
	}
	site, e := s.CreateSite(domain.CreateSiteInput{Name: "Before", BaseURL: "https://example.test", APIKey: "private-key-material"})
	if e != nil {
		t.Fatal(e)
	}
	return root, s, m, site
}
func TestBackupQueueAndStartupRestore(t *testing.T) {
	root, s, m, site := fixture(t)
	created, e := m.Create("manual", 30)
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.OpenReader(created.Path)
	if e != nil {
		t.Fatal(e)
	}
	if len(z.File) != 3 {
		t.Fatal("format1 entry count")
	}
	z.Close()
	name := "After"
	if _, e = s.UpdateSite(site.ID, domain.UpdateSiteInput{Name: &name}); e != nil {
		t.Fatal(e)
	}
	queued, e := m.QueueLocalRestore(created.FileName, 1)
	if e != nil || !queued.RelaunchRequired {
		t.Fatalf("queue %+v %v", queued, e)
	}
	live, e := s.GetSite(site.ID)
	if e != nil || live.Name != "After" {
		t.Fatal("queue modified open database")
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	result, e := ApplyPending(root)
	if e != nil || result == nil || result.Status != "applied" {
		t.Fatalf("restore %+v %v", result, e)
	}
	restored, e := store.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	live, e = restored.GetSite(site.ID)
	if e != nil || live.Name != "Before" {
		t.Fatalf("restored %+v %v", live, e)
	}
	secret, e := restored.GetSiteAPIKey(site.ID, "")
	if e != nil || secret != "private-key-material" {
		t.Fatalf("mismatched key %v", e)
	}
	result, e = TakeRestoreResult(root)
	if e != nil || result == nil || result.Status != "applied" {
		t.Fatal("missing restore result")
	}
	result, e = TakeRestoreResult(root)
	if e != nil || result != nil {
		t.Fatal("restore result not consumed")
	}
	files, e := m.List()
	if e != nil || len(files) != 1 || files[0].Reason == nil || *files[0].Reason != "pre_restore" {
		t.Fatalf("pre-restore retention %+v %v", files, e)
	}
}
func TestBundleRejectsUnsafeEntriesAndWrongKey(t *testing.T) {
	root, _, m, _ := fixture(t)
	created, e := m.Create("manual", 30)
	if e != nil {
		t.Fatal(e)
	}
	valid, e := ValidateAndExtract(created.Path, filepath.Join(root, "valid"))
	if e != nil {
		t.Fatal(e)
	}
	key := bytes.Repeat([]byte{7}, 32)
	manifest := valid.Manifest
	manifest.MasterKeySHA256 = hashBytes(key)
	db, e := os.ReadFile(valid.DatabasePath)
	if e != nil {
		t.Fatal(e)
	}
	wrong := filepath.Join(root, "wrong.zip")
	raw, _ := json.Marshal(manifest)
	writeTestZIP(t, wrong, []testEntry{{"xiaobai-switch.db", db, 0600}, {"master.key", key, 0600}, {"manifest.json", raw, 0600}})
	if _, e = ValidateAndExtract(wrong, filepath.Join(root, "wrong-payload")); e == nil {
		t.Fatal("accepted mismatched DB+key")
	}
	for _, entries := range [][]testEntry{{{"../xiaobai-switch.db", nil, 0600}, {"master.key", nil, 0600}, {"manifest.json", nil, 0600}}, {{"xiaobai-switch.db", nil, 0600}, {"master.key", nil, os.ModeSymlink | 0600}, {"manifest.json", nil, 0600}}, {{"master.key", nil, 0600}, {"master.key", nil, 0600}, {"manifest.json", nil, 0600}}} {
		path := filepath.Join(t.TempDir(), "unsafe.zip")
		writeTestZIP(t, path, entries)
		if _, e = ValidateAndExtract(path, filepath.Join(t.TempDir(), "payload")); e == nil {
			t.Fatal("accepted malicious bundle")
		}
	}
}

type testEntry struct {
	name string
	data []byte
	mode os.FileMode
}

func writeTestZIP(t *testing.T, path string, entries []testEntry) {
	t.Helper()
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	for _, entry := range entries {
		h := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		h.SetMode(entry.mode)
		w, e := z.CreateHeader(h)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(entry.data); e != nil {
			t.Fatal(e)
		}
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
}
func TestDevelopmentBackupSymlinkEscapeIsRejected(t *testing.T) {
	dev, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("XIAOBAI_DEV_HOME", dev)
	root := filepath.Join(dev, ".xiaobai-switch")
	s, e := store.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	m, e := New(root, s)
	if e != nil {
		t.Fatal(e)
	}
	outside := t.TempDir()
	if e = os.MkdirAll(filepath.Join(root, "backups"), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(outside, m.Dir()); e != nil {
		t.Fatal(e)
	}
	name := Prefix + "20260101_000000.device.12345678.zip"
	outsideFile := filepath.Join(outside, name)
	if e = os.WriteFile(outsideFile, []byte("sentinel"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = m.List(); e == nil {
		t.Fatal("listed escaped backup directory")
	}
	if e = m.Delete(name); e == nil {
		t.Fatal("deleted escaped backup")
	}
	if _, e = m.Create("manual", 1); e == nil {
		t.Fatal("created escaped backup")
	}
	if b, e := os.ReadFile(outsideFile); e != nil || string(b) != "sentinel" {
		t.Fatal("touched external directory")
	}
}
