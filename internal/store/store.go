package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"github.com/licoy/xiaobai-switch/internal/secure"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const SchemaVersion = 2

//go:embed schema.sql
var schema string

type Store struct {
	db     *sql.DB
	crypto *secure.Crypto
	root   string
	mu     sync.Mutex
}

func Open(root string) (*Store, error) {
	if root == "" {
		return nil, domain.NewError("validation_failed", "explicit application directory required")
	}
	root, err := paths.ValidatePath(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "xiaobai-switch.db"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, root: root}
	if err = s.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) initialize() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version < 0 || version > SchemaVersion {
		return domain.NewError("invalid_config", fmt.Sprintf("unsupported database schema %d", version))
	}
	if _, err := s.db.Exec("PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;"); err != nil {
		return err
	}
	values, err := s.encryptedValues()
	if err != nil {
		return err
	}
	s.crypto, err = secure.LoadOrCreate(filepath.Join(s.root, "master.key"), len(values) > 0)
	if err != nil {
		return err
	}
	for _, v := range values {
		if _, err = s.crypto.Decrypt(v); err != nil {
			return err
		}
	}
	exists, err := s.tableExists("sites")
	if err != nil {
		return err
	}
	if version < SchemaVersion && exists {
		if err = s.snapshot(version); err != nil {
			return err
		}
	}
	if version == 0 && exists {
		return s.migrateLegacy()
	}
	return s.transaction(func(tx *sql.Tx) error {
		if _, e := tx.Exec(schema); e != nil {
			return e
		}
		_, e := tx.Exec("PRAGMA user_version=2")
		return e
	})
}
func (s *Store) tableExists(name string) (bool, error) {
	var n int
	e := s.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n)
	return n > 0, e
}
func (s *Store) columnExists(table, column string) (bool, error) {
	rows, e := s.db.Query("PRAGMA table_info(" + table + ")")
	if e != nil {
		return false, e
	}
	defer rows.Close()
	for rows.Next() {
		var n, notnull, pk int
		var name, typ string
		var d any
		if e = rows.Scan(&n, &name, &typ, &notnull, &d, &pk); e != nil {
			return false, e
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
func (s *Store) encryptedValues() ([]string, error) {
	out := []string{}
	for _, pair := range [][2]string{{"sites", "api_key_encrypted"}, {"site_api_keys", "api_key_encrypted"}, {"webdav_config", "password_encrypted"}} {
		ok, e := s.tableExists(pair[0])
		if e != nil {
			return nil, e
		}
		if !ok {
			continue
		}
		ok, e = s.columnExists(pair[0], pair[1])
		if e != nil {
			return nil, e
		}
		if !ok {
			continue
		}
		rows, e := s.db.Query("SELECT " + pair[1] + " FROM " + pair[0])
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var v string
			if e = rows.Scan(&v); e != nil {
				rows.Close()
				return nil, e
			}
			if v != "" {
				out = append(out, v)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}
func (s *Store) snapshot(version int) error {
	dir := filepath.Join(s.root, "backups", "app", fmt.Sprintf("pre_migration_v%d_%s", version, domain.NewID()))
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	// VACUUM INTO includes committed WAL pages in a consistent SQLite snapshot
	dest := strings.ReplaceAll(filepath.Join(dir, "xiaobai-switch.db"), "'", "''")
	if _, e := s.db.Exec("VACUUM INTO '" + dest + "'"); e != nil {
		return fmt.Errorf("backup_failed: %w", e)
	}
	b, e := os.ReadFile(filepath.Join(s.root, "master.key"))
	if e != nil {
		return e
	}
	if e = fileops.Secure(filepath.Join(dir, "xiaobai-switch.db")); e != nil {
		return e
	}
	return fileops.AtomicWrite(filepath.Join(dir, "master.key"), b)
}
func (s *Store) transaction(fn func(*sql.Tx) error) (err error) {
	tx, e := s.db.BeginTx(context.Background(), nil)
	if e != nil {
		return e
	}
	defer func() {
		if err != nil {
			if re := tx.Rollback(); re != nil && re != sql.ErrTxDone {
				err = fmt.Errorf("%w; rollback: %v", err, re)
			}
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func now() int64 { return time.Now().UnixMilli() }
func requireChanged(result sql.Result, e error) error {
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 {
		return domain.NewError("not_found", "record not found")
	}
	return nil
}
