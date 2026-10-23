package fileops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite stages beside the destination so replacement stays on one volume.
func AtomicWrite(path string, content []byte) (err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".xiaobai-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer func() {
		if e := os.Remove(temp); e != nil && !errors.Is(e, os.ErrNotExist) {
			err = errors.Join(err, e)
		}
	}()
	if err = Secure(temp); err != nil {
		return errors.Join(err, f.Close())
	}
	if _, err = f.Write(content); err != nil {
		return errors.Join(err, f.Close())
	}
	if err = f.Sync(); err != nil {
		return errors.Join(err, f.Close())
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = replace(temp, path); err != nil {
		return fmt.Errorf("atomic replacement: %w", err)
	}
	return syncParent(filepath.Dir(path))
}
