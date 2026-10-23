package appbackup

import (
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"os"
	"path/filepath"
	"strings"
)

const pendingName = ".pending-restore"
const journalName = "apply-journal.json"

type applyJournal struct {
	TargetExisted []bool `json:"target_existed"`
}

func QueuePending(archive, root string) (err error) {
	root, e := paths.ValidatePath(root)
	if e != nil {
		return e
	}
	locks, e := fileops.Acquire(filepath.Join(root, "locks"), []string{"restore"}, nil)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, locks.Close()) }()
	pending := filepath.Join(root, pendingName)
	if _, e = os.Lstat(pending); e == nil {
		return domain.NewError("restore_pending", "another restore is already pending")
	} else if !os.IsNotExist(e) {
		return e
	}
	staging, e := os.MkdirTemp(root, pendingName+".*.migrating")
	if e != nil {
		return e
	}
	defer func() {
		if staging != "" {
			err = errors.Join(err, os.RemoveAll(staging))
		}
	}()
	if e = fileops.SecureDirectory(staging); e != nil {
		return e
	}
	if _, e = ValidateAndExtract(archive, filepath.Join(staging, "payload")); e != nil {
		return e
	}
	if e = os.Rename(staging, pending); e != nil {
		return e
	}
	staging = ""
	return syncDir(root)
}

// ApplyPending must run before Store.Open; only application DB/key files are replaced
func ApplyPending(root string) (result *domain.RestoreStartupResult, err error) {
	root, e := paths.ValidatePath(root)
	if e != nil {
		return nil, e
	}
	if _, e = os.Lstat(root); os.IsNotExist(e) {
		return nil, nil
	} else if e != nil {
		return nil, e
	}
	locks, e := fileops.Acquire(filepath.Join(root, "locks"), []string{"restore"}, nil)
	if e != nil {
		return nil, e
	}
	defer func() { err = errors.Join(err, locks.Close()) }()
	if e = cleanupStaging(root); e != nil {
		return nil, e
	}
	pending := filepath.Join(root, pendingName)
	info, e := os.Lstat(pending)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, invalid("pending restore must be a regular directory")
	}
	e = applyPendingInner(root, pending)
	outcome := domain.RestoreStartupResult{Status: "applied", Message: "Application data was restored successfully."}
	var committed *committedCleanupError
	if errors.As(e, &committed) {
		outcome.Message = "Application data was restored; cleanup remains pending: " + committed.Error()
	} else if e != nil {
		if recovery := recoverUncommitted(root, pending); recovery != nil {
			return nil, errors.Join(e, recovery)
		}
		quarantine := filepath.Join(root, ".failed-restore-"+domain.NewID())
		if _, statErr := os.Lstat(pending); statErr == nil {
			if renameErr := os.Rename(pending, quarantine); renameErr != nil {
				return nil, errors.Join(e, renameErr)
			}
		} else if !os.IsNotExist(statErr) {
			return nil, errors.Join(e, statErr)
		}
		outcome.Status = "failed"
		outcome.Message = "Restore failed; previous application data was kept: " + e.Error()
	}
	raw, e := json.Marshal(outcome)
	if e != nil {
		return nil, e
	}
	if e = fileops.AtomicWrite(filepath.Join(root, "restore-result.json"), raw); e != nil {
		return nil, e
	}
	return &outcome, nil
}

type committedCleanupError struct{ error }

func applyPendingInner(root, pending string) error {
	journal, e := exists(filepath.Join(pending, journalName))
	if e != nil {
		return e
	}
	committed, e := exists(filepath.Join(pending, "committed"))
	if e != nil {
		return e
	}
	if committed {
		if e = cleanupCommitted(root, pending); e != nil {
			return &committedCleanupError{e}
		}
		return nil
	}
	if journal {
		if e = recoverUncommitted(root, pending); e != nil {
			return e
		}
	}
	payload := filepath.Join(pending, "payload")
	if e = guard(root, payload); e != nil {
		return e
	}
	if _, e = ValidatePayload(payload); e != nil {
		return e
	}
	ops, e := prepareOperations(root, pending)
	if e != nil {
		return e
	}
	j := applyJournal{}
	for _, op := range ops {
		j.TargetExisted = append(j.TargetExisted, op.existed)
	}
	raw, e := json.Marshal(j)
	if e != nil {
		return e
	}
	if e = fileops.AtomicWrite(filepath.Join(pending, journalName), raw); e != nil {
		return e
	}
	if e = publishOperations(ops); e != nil {
		return errors.Join(e, rollbackOperations(ops))
	}
	if e = fileops.AtomicWrite(filepath.Join(pending, "committed"), []byte{}); e != nil {
		has, statErr := exists(filepath.Join(pending, "committed"))
		if statErr != nil {
			return errors.Join(e, statErr)
		}
		if has {
			return &committedCleanupError{e}
		}
		return e
	}
	if e = cleanupCommitted(root, pending); e != nil {
		return &committedCleanupError{e}
	}
	return nil
}

func recoverUncommitted(root, pending string) error {
	path := filepath.Join(pending, journalName)
	present, e := exists(path)
	if e != nil {
		return e
	}
	if !present {
		return nil
	}
	committed, e := exists(filepath.Join(pending, "committed"))
	if e != nil {
		return e
	}
	if committed {
		return nil
	}
	info, e := regular(path)
	if e != nil {
		return e
	}
	if info.Size() > maxManifestBytes {
		return invalid("restore journal is too large")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	var journal applyJournal
	if e = json.Unmarshal(raw, &journal); e != nil {
		return invalid("restore journal is invalid")
	}
	ops := desiredOperations(root, pending)
	if len(journal.TargetExisted) != len(ops) {
		return invalid("restore journal does not match payload")
	}
	for i := range ops {
		ops[i].existed = journal.TargetExisted[i]
	}
	if e = rollbackOperations(ops); e != nil {
		return e
	}
	if e = removeIfPresent(path); e != nil {
		return e
	}
	return syncDir(pending)
}
func cleanupCommitted(root, pending string) error {
	for _, op := range desiredOperations(root, pending) {
		if e := removeIfPresent(op.rollback); e != nil {
			return e
		}
		if op.stage != "" {
			if e := removeIfPresent(op.stage); e != nil {
				return e
			}
		}
	}
	if e := os.RemoveAll(pending); e != nil {
		return e
	}
	return syncDir(root)
}
func cleanupStaging(root string) error {
	entries, e := os.ReadDir(root)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if entry.IsDir() && (strings.HasPrefix(entry.Name(), ".local-restore-") || strings.HasPrefix(entry.Name(), ".webdav-restore-")) {
			if e = os.RemoveAll(filepath.Join(root, entry.Name())); e != nil {
				return e
			}
		}
	}
	return nil
}

func exists(path string) (bool, error) {
	info, e := os.Lstat(path)
	if os.IsNotExist(e) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if !info.Mode().IsRegular() {
		return false, invalid("restore journal/marker must be a regular file")
	}
	return true, nil
}

func removeIfPresent(path string) error {
	e := os.Remove(path)
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
