package store

import (
	"database/sql"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"strings"
)

func (s *Store) migrateLegacy() error {
	legacy, e := s.columnExists("sites", "api_key_encrypted")
	if e != nil {
		return e
	}
	if !legacy {
		return domain.NewError("invalid_config", "schema version 0 has no legacy secret column")
	}
	if _, e = s.db.Exec("PRAGMA foreign_keys=OFF"); e != nil {
		return e
	}
	e = s.transaction(func(tx *sql.Tx) error {
		if e := migrateLegacyTx(tx); e != nil {
			return e
		}
		rows, e := tx.Query("PRAGMA foreign_key_check")
		if e != nil {
			return e
		}
		defer rows.Close()
		if rows.Next() {
			return domain.NewError("invalid_config", "foreign key violation during migration")
		}
		return rows.Err()
	})
	_, enableErr := s.db.Exec("PRAGMA foreign_keys=ON")
	if e != nil {
		return e
	}
	if enableErr != nil {
		return enableErr
	}
	rows, e := s.db.Query("PRAGMA foreign_key_check")
	if e != nil {
		return e
	}
	defer rows.Close()
	if rows.Next() {
		return domain.NewError("invalid_config", "foreign key violation after migration")
	}
	return rows.Err()
}
func migrateLegacyTx(tx *sql.Tx) error {
	// Historical version-zero schemas gained these optional columns before versioning
	for _, c := range []struct{ name, typ string }{{"base_urls_json", "TEXT"}, {"capabilities_json", "TEXT"}} {
		if e := ensureColumn(tx, "sites", c.name, c.typ); e != nil {
			return e
		}
	}
	if e := ensureColumn(tx, "site_models", "is_manual", "INTEGER NOT NULL DEFAULT 0"); e != nil {
		return e
	}
	keySchema := schema[strings.Index(schema, "CREATE TABLE IF NOT EXISTS site_api_keys"):strings.Index(schema, "CREATE UNIQUE INDEX IF NOT EXISTS idx_site_api_keys_one_active")]
	if _, e := tx.Exec(keySchema); e != nil {
		return e
	}
	rows, e := tx.Query("SELECT id FROM sites")
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		_, e = tx.Exec(`INSERT INTO site_api_keys (id,site_id,label,api_key_encrypted,key_prefix,is_active,selected_model_id,last_model_fetch_at,last_model_fetch_latency_ms,last_model_fetch_error,created_at,updated_at) SELECT ?,id,'K 1',api_key_encrypted,key_prefix,1,selected_model_id,last_model_fetch_at,last_model_fetch_latency_ms,last_model_fetch_error,created_at,updated_at FROM sites WHERE id=?`, domain.NewID(), id)
		if e != nil {
			return e
		}
	}
	if e = migrateModels(tx); e != nil {
		return e
	}
	for _, table := range []string{"target_bindings", "apply_records"} {
		for _, column := range []string{"site_api_key_id", "site_api_key_label_snapshot", "site_api_key_prefix_snapshot"} {
			if e = ensureColumn(tx, table, column, "TEXT"); e != nil {
				return e
			}
		}
		_, e = tx.Exec("UPDATE " + table + " SET site_api_key_id=(SELECT id FROM site_api_keys WHERE site_id=" + table + ".site_id),site_api_key_label_snapshot='K 1',site_api_key_prefix_snapshot=(SELECT key_prefix FROM site_api_keys WHERE site_id=" + table + ".site_id) WHERE site_id IS NOT NULL")
		if e != nil {
			return e
		}
	}
	start := strings.Index(schema, "CREATE TABLE IF NOT EXISTS sites (")
	end := strings.Index(schema[start:], ";\n") + start
	create := strings.Replace(schema[start:end], "IF NOT EXISTS sites", "sites_new", 1)
	if _, e = tx.Exec(create); e != nil {
		return e
	}
	cols := "id,name,base_url,protocol,claude_auth_key_style,notes,enabled,sort_order,created_at,updated_at,base_urls_json,capabilities_json"
	if _, e = tx.Exec("INSERT INTO sites_new (" + cols + ") SELECT " + cols + " FROM sites; DROP TABLE sites; ALTER TABLE sites_new RENAME TO sites;"); e != nil {
		return e
	}
	if _, e = tx.Exec(schema); e != nil {
		return e
	}
	if _, e = tx.Exec("UPDATE sites SET base_urls_json=json_array(base_url) WHERE base_urls_json IS NULL OR base_urls_json='' "); e != nil {
		return e
	}
	_, e = tx.Exec("PRAGMA user_version=2")
	return e
}
func ensureColumn(tx *sql.Tx, table, column, typ string) error {
	rows, e := tx.Query("PRAGMA table_info(" + table + ")")
	if e != nil {
		return e
	}
	found := false
	for rows.Next() {
		var n, nn, pk int
		var name, t string
		var def any
		if e = rows.Scan(&n, &name, &t, &nn, &def, &pk); e != nil {
			rows.Close()
			return e
		}
		found = found || name == column
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if found {
		return nil
	}
	_, e = tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, typ))
	return e
}
func migrateModels(tx *sql.Tx) error {
	for _, table := range []string{"site_models", "site_model_exclusions"} {
		start := strings.Index(schema, "CREATE TABLE IF NOT EXISTS "+table+" (")
		end := strings.Index(schema[start:], ";\n") + start
		create := strings.Replace(schema[start:end], "IF NOT EXISTS "+table, table+"_new", 1)
		if _, e := tx.Exec(create); e != nil {
			return e
		}
		cols := "site_id,model_id"
		if table == "site_models" {
			cols = "id,site_id,model_id,display_name,owned_by,raw_json,is_manual"
		}
		if _, e := tx.Exec("INSERT INTO " + table + "_new (" + cols + ",api_key_id) SELECT " + cols + ",(SELECT id FROM site_api_keys WHERE site_id=" + table + ".site_id) FROM " + table + "; DROP TABLE " + table + "; ALTER TABLE " + table + "_new RENAME TO " + table); e != nil {
			return e
		}
	}
	return nil
}
