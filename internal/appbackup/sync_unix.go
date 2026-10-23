//go:build !windows

package appbackup

import (
	"errors"
	"os"
)

func syncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	return errors.Join(f.Sync(), f.Close())
}
