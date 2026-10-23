package store

import (
	"database/sql"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"os"
	"path/filepath"
	"strings"
)

// SnapshotTo uses SQLite's backup mechanism rather than copying a live WAL file
func (s *Store) SnapshotTo(destination string) error {
	var err error
	destination, err = paths.ValidatePath(destination)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(destination) {
		return domain.NewError("validation_failed", "snapshot destination must be absolute")
	}
	if filepath.Clean(destination) == filepath.Join(s.root, "xiaobai-switch.db") {
		return domain.NewError("validation_failed", "snapshot cannot overwrite live database")
	}
	if _, e := os.Stat(destination); e == nil {
		return domain.NewError("backup_failed", "snapshot destination already exists")
	} else if !os.IsNotExist(e) {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(destination), 0700); e != nil {
		return e
	}
	escaped := strings.ReplaceAll(destination, "'", "''")
	if _, e := s.db.Exec("VACUUM INTO '" + escaped + "'"); e != nil {
		return fmt.Errorf("backup_failed: %w", e)
	}
	return fileops.Secure(destination)
}
func (s *Store) ReorderSites(ids []string) error {
	return s.transaction(func(tx *sql.Tx) error {
		var count int
		if e := tx.QueryRow("SELECT count(*) FROM sites").Scan(&count); e != nil {
			return e
		}
		if count != len(ids) {
			return domain.NewError("validation_failed", "reorder must contain every site exactly once")
		}
		seen := map[string]bool{}
		for i, id := range ids {
			if seen[id] {
				return domain.NewError("validation_failed", "duplicate site in reorder")
			}
			seen[id] = true
			if e := requireChanged(tx.Exec("UPDATE sites SET sort_order=?,updated_at=? WHERE id=?", i, now(), id)); e != nil {
				return e
			}
		}
		return nil
	})
}

// ClearApplication commits a completed restore's database changes together
func (s *Store) ClearApplication(target domain.TargetKind, recordID string) error {
	if !target.Valid() {
		return domain.NewError("validation_failed", "unknown target")
	}
	return s.transaction(func(tx *sql.Tx) error {
		if recordID != "" {
			var stored string
			if e := tx.QueryRow("SELECT target FROM apply_records WHERE id=?", recordID).Scan(&stored); e != nil {
				return e
			}
			if stored != string(target) {
				return domain.NewError("validation_failed", "record belongs to another target")
			}
			if e := requireChanged(tx.Exec("DELETE FROM apply_records WHERE id=?", recordID)); e != nil {
				return e
			}
		}
		_, e := tx.Exec("DELETE FROM target_bindings WHERE target=?", target)
		return e
	})
}
