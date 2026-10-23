package envinject

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"os"
	"path/filepath"
)

type userJournal struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Existed bool   `json:"existed"`
	State   string `json:"state"`
}

func saveUser(path string, j userJournal) error {
	raw, e := json.Marshal(j)
	if e != nil {
		return e
	}
	return fileops.AtomicWrite(path, raw)
}

// Execute includes user-environment mutation in the file transaction's rollback boundary.
func Execute(p Plan, backupDir string) (fileops.Result, error) {
	return execute(p, backupDir, ReadUser, WriteUser)
}
func execute(p Plan, backupDir string, read func(string) (string, bool, error), write func(string, string, bool) error) (fileops.Result, error) {
	if p.Mode != "user_env" {
		return fileops.Execute(p.Files, backupDir)
	}
	result := fileops.Result{}
	value, exists, e := read(p.UserKey)
	if e != nil {
		return result, e
	}
	if p.UserExpectedFingerprint != "" {
		h := sha256.Sum256([]byte(value))
		if !exists || hex.EncodeToString(h[:]) != p.UserExpectedFingerprint {
			return result, errors.New("config_changed: user environment was modified externally")
		}
	}
	j := userJournal{Key: p.UserKey, Value: value, Existed: exists, State: "prepared"}
	// The registry snapshot is private and exists before files or registry are changed.
	parent := filepath.Dir(backupDir)
	if e = os.MkdirAll(parent, 0700); e != nil {
		return result, e
	}
	snapshot := backupDir + ".user-env.json"
	if _, e = os.Lstat(snapshot); !errors.Is(e, os.ErrNotExist) {
		return result, errors.New("user environment recovery snapshot already exists or is unreadable")
	}
	if e = saveUser(snapshot, j); e != nil {
		return result, e
	}
	result, e = fileops.Execute(p.Files, backupDir)
	if e != nil {
		j.State = "rolled_back"
		return result, errors.Join(e, saveUser(snapshot, j))
	}
	if e = write(p.UserKey, p.UserValue, p.UserRemove); e == nil {
		var got string
		var present bool
		got, present, e = read(p.UserKey)
		if e == nil && (present == p.UserRemove || (!p.UserRemove && got != p.UserValue)) {
			e = errors.New("user environment post-write verification failed")
		}
	}
	if e != nil {
		return result, errors.Join(e, write(j.Key, j.Value, !j.Existed), fileops.Rollback(backupDir))
	}
	j.State = "committed"
	if e = saveUser(snapshot, j); e != nil {
		return result, errors.Join(e, write(j.Key, j.Value, !j.Existed), fileops.Rollback(backupDir))
	}
	return result, nil
}

// Recover rolls back both participants when the registry journal did not commit.
func Recover(backupDir string) error {
	path := backupDir + ".user-env.json"
	raw, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return fileops.Recover(backupDir)
	}
	if e != nil {
		return e
	}
	var j userJournal
	if e = json.Unmarshal(raw, &j); e != nil {
		return e
	}
	if !ValidKey(j.Key) {
		return errors.New("invalid environment recovery key")
	}
	if j.State == "committed" || j.State == "rolled_back" {
		return fileops.Recover(backupDir)
	}
	if e = WriteUser(j.Key, j.Value, !j.Existed); e != nil {
		return e
	}
	if e = fileops.Rollback(backupDir); e != nil {
		return e
	}
	j.State = "rolled_back"
	return saveUser(path, j)
}

// Rollback restores both participants after a later database commit fails.
func Rollback(backupDir string) error {
	path := backupDir + ".user-env.json"
	raw, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return fileops.Rollback(backupDir)
	}
	if e != nil {
		return e
	}
	var j userJournal
	if e = json.Unmarshal(raw, &j); e != nil {
		return e
	}
	if !ValidKey(j.Key) {
		return errors.New("invalid recovery environment key")
	}
	if e = WriteUser(j.Key, j.Value, !j.Existed); e != nil {
		return e
	}
	if e = fileops.Rollback(backupDir); e != nil {
		return e
	}
	j.State = "rolled_back"
	return saveUser(path, j)
}
