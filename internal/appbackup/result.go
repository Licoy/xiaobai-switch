package appbackup

import (
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"os"
	"path/filepath"
)

func TakeRestoreResult(root string) (*domain.RestoreStartupResult, error) {
	root, e := paths.ValidatePath(root)
	if e != nil {
		return nil, e
	}
	path := filepath.Join(root, "restore-result.json")
	info, e := regular(path)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if info.Size() > maxManifestBytes {
		return nil, invalid("restore result is too large")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var result domain.RestoreStartupResult
	if e = json.Unmarshal(b, &result); e != nil {
		return nil, e
	}
	if e = os.Remove(path); e != nil {
		return nil, e
	}
	return &result, nil
}
