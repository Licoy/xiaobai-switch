package appbackup

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabaseValidationSpecialPathRemainsReadOnly(t *testing.T) {
	root, s, _, _ := fixture(t)
	path := filepath.Join(root, "snapshot #100% & 中文.db")
	if err := s.SnapshotTo(path); err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(filepath.Join(root, "master.key"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDatabase(path, key); err != nil {
		t.Fatal(err)
	}
	uri, err := readOnlyDatabaseURI(path)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(uri)
	if err != nil || u.Host != "" || u.Fragment != "" || u.Query().Get("mode") != "ro" || u.Query().Get("immutable") != "1" {
		t.Fatalf("invalid read-only URI %q: %v", uri, err)
	}
	if strings.TrimPrefix(u.Path, "/") != strings.TrimPrefix(filepath.ToSlash(path), "/") {
		t.Fatalf("URI changed database path: %q", uri)
	}
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE unexpected_write (id INTEGER)"); err == nil {
		t.Fatal("validation connection accepted a database write")
	}
	missing := filepath.Join(root, "missing #100%.db")
	if err := validateDatabase(missing, key); err == nil {
		t.Fatal("validation accepted a missing database")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("validation created a database: %v", err)
	}
}
