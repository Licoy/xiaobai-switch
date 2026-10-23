package fileops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Locks struct {
	files []*os.File
	dirs  []string
}

// Acquire takes old Rust apply locks first, then Pi/Prime directory locks in path order.
func Acquire(root string, targets, configPaths []string) (*Locks, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	held := &Locks{}
	fail := func(err error) (*Locks, error) { return nil, errors.Join(err, held.Close()) }
	keys := append([]string(nil), targets...)
	sort.Strings(keys)
	for i, key := range keys {
		if i > 0 && key == keys[i-1] {
			continue
		}
		if strings.ContainsAny(key, "/\\") {
			return fail(fmt.Errorf("invalid lock name"))
		}
		fileKey := key
		if runtime.GOOS == "windows" {
			fileKey = strings.ReplaceAll(key, ":", "%3A")
		}
		f, e := os.OpenFile(filepath.Join(root, "apply_"+fileKey+".lock"), os.O_CREATE|os.O_RDWR, 0600)
		if e != nil {
			return fail(e)
		}
		if e = lockFile(f); e != nil {
			return fail(errors.Join(fmt.Errorf("lock_busy: %s", key), f.Close()))
		}
		held.files = append(held.files, f)
	}
	dirs := append([]string(nil), configPaths...)
	sort.Strings(dirs)
	for _, p := range dirs {
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			return fail(e)
		}
		d := p + ".lock"
		if e := os.Mkdir(d, 0700); e != nil {
			return fail(fmt.Errorf("lock_busy: %s: %w", p, e))
		}
		held.dirs = append(held.dirs, d)
	}
	return held, nil
}
func (h *Locks) Close() error {
	var out error
	for i := len(h.dirs) - 1; i >= 0; i-- {
		out = errors.Join(out, os.Remove(h.dirs[i]))
	}
	h.dirs = nil
	for i := len(h.files) - 1; i >= 0; i-- {
		out = errors.Join(out, unlockFile(h.files[i]), h.files[i].Close())
	}
	h.files = nil
	return out
}
