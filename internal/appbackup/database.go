package appbackup

import (
	"database/sql"
	"github.com/licoy/xiaobai-switch/internal/secure"
	"github.com/licoy/xiaobai-switch/internal/store"
	_ "modernc.org/sqlite"
	"net/url"
	"path/filepath"
	"strings"
)

func validateDatabase(path string, key []byte) error {
	uri, e := readOnlyDatabaseURI(path)
	if e != nil {
		return e
	}
	db, e := sql.Open("sqlite", uri)
	if e != nil {
		return e
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	rows, e := db.Query("PRAGMA quick_check")
	if e != nil {
		return e
	}
	for rows.Next() {
		var result string
		if e = rows.Scan(&result); e != nil {
			rows.Close()
			return e
		}
		if result != "ok" {
			rows.Close()
			return invalid("database integrity check failed")
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	var version int
	if e = db.QueryRow("PRAGMA user_version").Scan(&version); e != nil {
		return e
	}
	if version < 0 || version > store.SchemaVersion {
		return invalid("unsupported database schema")
	}
	crypto, e := secure.FromKey(key)
	if e != nil {
		return e
	}
	tables := [][2]string{{"webdav_config", "password_encrypted"}}
	if version == 0 {
		tables = append(tables, [2]string{"sites", "api_key_encrypted"})
	} else {
		tables = append(tables, [2]string{"site_api_keys", "api_key_encrypted"})
	}
	for _, name := range []string{"settings", "sites", "site_models", "target_bindings", "apply_records"} {
		var n int
		if e = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n); e != nil {
			return e
		}
		if n != 1 {
			return invalid("database schema is incomplete")
		}
	}
	for _, pair := range tables {
		var n int
		if e = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", pair[0]).Scan(&n); e != nil {
			return e
		}
		if n == 0 && pair[0] == "webdav_config" {
			continue
		}
		rows, e = db.Query("SELECT " + pair[1] + " FROM " + pair[0])
		if e != nil {
			return invalid("database secret schema is invalid")
		}
		for rows.Next() {
			var encrypted string
			if e = rows.Scan(&encrypted); e != nil {
				rows.Close()
				return e
			}
			if encrypted != "" {
				if _, e = crypto.Decrypt(encrypted); e != nil {
					rows.Close()
					return invalid("database and master.key do not belong together")
				}
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
	}
	rows, e = db.Query("PRAGMA foreign_key_check")
	if e != nil {
		return e
	}
	defer rows.Close()
	if rows.Next() {
		return invalid("database foreign keys are invalid")
	}
	return rows.Err()
}

func readOnlyDatabaseURI(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	uriPath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath, RawQuery: url.Values{"mode": {"ro"}, "immutable": {"1"}}.Encode()}
	return u.String(), nil
}
