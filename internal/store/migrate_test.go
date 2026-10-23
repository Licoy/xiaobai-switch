package store

import (
	"database/sql"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/secure"
	"os"
	"path/filepath"
	"testing"
)

func installLegacy(t *testing.T, root string, broken bool) {
	t.Helper()
	db, e := sql.Open("sqlite", filepath.Join(root, "xiaobai-switch.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	sqlBytes, e := os.ReadFile("testdata/legacy.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(string(sqlBytes)); e != nil {
		t.Fatal(e)
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = 7
	}
	c, e := secure.FromKey(key)
	if e != nil {
		t.Fatal(e)
	}
	enc, e := c.Encrypt("sk-legacy")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "master.key"), key, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO sites(id,name,base_url,api_key_encrypted,key_prefix,selected_model_id,created_at,updated_at) VALUES('s1','Legacy','https://example.test',?,'sk…cy','legacy',1,2)`, enc); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO site_models(id,site_id,model_id,display_name) VALUES('m1','s1','legacy','Legacy')`); e != nil {
		t.Fatal(e)
	}
	siteID := "s1"
	if broken {
		siteID = "missing"
		if _, e = db.Exec("PRAGMA foreign_keys=OFF"); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = db.Exec(`INSERT INTO target_bindings(target,site_id,site_name_snapshot,model_id,key_fingerprint,managed_paths_json,managed_env_keys_json,expected_fields_json,applied_at) VALUES('codex',?,'Legacy','legacy','fp','[]','[]','{}',1)`, siteID); e != nil {
		t.Fatal(e)
	}
}
func TestLegacyMigrationAndSnapshot(t *testing.T) {
	root := t.TempDir()
	installLegacy(t, root, false)
	s, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	v, e := s.GetSite("s1")
	if e != nil {
		t.Fatal(e)
	}
	if len(v.APIKeys) != 1 || v.APIKeys[0].Label != "K 1" || *v.SelectedModelID != "legacy" {
		t.Fatalf("migration %+v", v)
	}
	key, e := s.GetSiteAPIKey(v.ID, "")
	if e != nil || key != "sk-legacy" {
		t.Fatalf("decrypt %q %v", key, e)
	}
	models, e := s.ListModels(v.ID)
	if e != nil || len(models) != 1 || models[0].APIKeyID != *v.ActiveAPIKeyID {
		t.Fatalf("models %+v %v", models, e)
	}
	b, e := s.GetBinding(domain.Codex)
	if e != nil || b == nil || b.APIKey.ID == nil || *b.APIKey.ID != *v.ActiveAPIKeyID {
		t.Fatalf("binding %+v %v", b, e)
	}
	files, e := filepath.Glob(filepath.Join(root, "backups", "app", "pre_migration_v0_*", "xiaobai-switch.db"))
	if e != nil || len(files) != 1 {
		t.Fatalf("snapshot %+v %v", files, e)
	}
	old, e := sql.Open("sqlite", files[0])
	if e != nil {
		t.Fatal(e)
	}
	defer old.Close()
	var version int
	if e = old.QueryRow("PRAGMA user_version").Scan(&version); e != nil || version != 0 {
		t.Fatalf("snapshot version %d %v", version, e)
	}
	var secret string
	if e = old.QueryRow("SELECT api_key_encrypted FROM sites").Scan(&secret); e != nil {
		t.Fatal(e)
	}
	c, e := secure.LoadOrCreate(filepath.Join(filepath.Dir(files[0]), "master.key"), true)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := c.Decrypt(secret); e != nil || got != "sk-legacy" {
		t.Fatalf("inconsistent snapshot %q %v", got, e)
	}
}
func TestLegacyMigrationForeignKeyFailureRollsBack(t *testing.T) {
	root := t.TempDir()
	installLegacy(t, root, true)
	s, e := Open(root)
	if e == nil {
		s.Close()
		t.Fatal("accepted corrupted legacy binding")
	}
	db, e := sql.Open("sqlite", filepath.Join(root, "xiaobai-switch.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var version int
	if e = db.QueryRow("PRAGMA user_version").Scan(&version); e != nil || version != 0 {
		t.Fatalf("schema changed after failure %d %v", version, e)
	}
	var secret string
	if e = db.QueryRow("SELECT api_key_encrypted FROM sites").Scan(&secret); e != nil {
		t.Fatalf("legacy table changed after rollback %v", e)
	}
	var n int
	if e = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='site_api_keys'").Scan(&n); e != nil || n != 0 {
		t.Fatalf("partial schema %d %v", n, e)
	}
}
