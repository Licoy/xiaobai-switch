package appbackup

import (
	"fmt"
	"os"
	"path/filepath"
)

type operation struct {
	target, source, stage, rollback string
	existed                         bool
}

func desiredOperations(root, pending string) []operation {
	names := []string{"master.key", "xiaobai-switch.db-wal", "xiaobai-switch.db-shm", "xiaobai-switch.db-journal", "xiaobai-switch.db"}
	out := []operation{}
	for i, name := range names {
		op := operation{target: filepath.Join(root, name), rollback: filepath.Join(root, fmt.Sprintf(".%s.restore-%d.rollback", name, i))}
		if i == 0 || i == 4 {
			op.source = filepath.Join(pending, "payload", name)
			op.stage = filepath.Join(root, fmt.Sprintf(".%s.restore-%d.new", name, i))
		}
		out = append(out, op)
	}
	return out
}
func prepareOperations(root, pending string) ([]operation, error) {
	ops := desiredOperations(root, pending)
	for _, op := range ops {
		if e := guard(root, op.target); e != nil {
			return nil, e
		}
		if e := guard(root, op.rollback); e != nil {
			return nil, e
		}
		if op.stage != "" {
			if e := guard(root, op.stage); e != nil {
				return nil, e
			}
		}
	}
	for i := range ops {
		op := &ops[i]
		info, e := os.Lstat(op.target)
		if e == nil {
			if !info.Mode().IsRegular() {
				return nil, invalid("restore target must be a regular file")
			}
			op.existed = true
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		if e = removeIfPresent(op.rollback); e != nil {
			return nil, e
		}
		if op.source != "" {
			if e = removeIfPresent(op.stage); e != nil {
				return nil, e
			}
			if e = copyPrivate(op.source, op.stage, MaxArchiveBytes); e != nil {
				return nil, e
			}
		}
	}
	return ops, nil
}
func publishOperations(ops []operation) error {
	for _, op := range ops {
		if op.existed {
			if _, e := regular(op.target); e != nil {
				return e
			}
			if e := os.Rename(op.target, op.rollback); e != nil {
				return e
			}
		} else {
			if _, e := os.Lstat(op.target); e == nil {
				return invalid("restore target changed")
			} else if !os.IsNotExist(e) {
				return e
			}
		}
		if op.stage != "" {
			if e := os.Rename(op.stage, op.target); e != nil {
				return e
			}
		}
		if e := syncDir(filepath.Dir(op.target)); e != nil {
			return e
		}
	}
	return nil
}
func rollbackOperations(ops []operation) error {
	for i := len(ops) - 1; i >= 0; i-- {
		op := ops[i]
		if _, e := os.Lstat(op.rollback); e == nil {
			if _, e = regular(op.rollback); e != nil {
				return e
			}
			if e = removeIfPresent(op.target); e != nil {
				return e
			}
			if e = os.Rename(op.rollback, op.target); e != nil {
				return e
			}
		} else if !os.IsNotExist(e) {
			return e
		} else if op.existed {
			if _, e = regular(op.target); e != nil {
				return invalid("restore rollback is missing")
			}
		} else {
			if e = removeIfPresent(op.target); e != nil {
				return e
			}
		}
		if op.stage != "" {
			if e := removeIfPresent(op.stage); e != nil {
				return e
			}
		}
	}
	if len(ops) > 0 {
		return syncDir(filepath.Dir(ops[0].target))
	}
	return nil
}
