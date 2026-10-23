package appbackup

import (
	"errors"
	"github.com/licoy/xiaobai-switch/internal/buildinfo"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"github.com/licoy/xiaobai-switch/internal/store"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	root  string
	store *store.Store
	mu    sync.Mutex
}

func New(root string, s *store.Store) (*Manager, error) {
	root, e := paths.ValidatePath(root)
	if e != nil {
		return nil, e
	}
	if s == nil {
		return nil, domain.NewError("validation_failed", "backup store is required")
	}
	return &Manager{root: root, store: s}, nil
}
func (m *Manager) Dir() string { return filepath.Join(m.root, "backups", "app") }
func ensureDir(path string) error {
	if _, e := paths.ValidatePath(path); e != nil {
		return e
	}
	info, e := os.Lstat(path)
	if errors.Is(e, os.ErrNotExist) {
		if e = os.MkdirAll(path, 0700); e != nil {
			return e
		}
		return fileops.SecureDirectory(path)
	}
	if e != nil {
		return e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return invalid("backup directory must be a regular directory")
	}
	return fileops.SecureDirectory(path)
}
func DeviceName() string {
	host, e := os.Hostname()
	if e != nil {
		host = "unknown"
	}
	var out strings.Builder
	for _, r := range host {
		if r < 128 && ((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			out.WriteRune(r)
			if out.Len() == 48 {
				break
			}
		}
	}
	if out.Len() == 0 {
		return "unknown"
	}
	return out.String()
}
func ParseDeviceFromFilename(name string) string {
	if !safeName(name) {
		return "unknown"
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(name, Prefix), Suffix), ".")
	if len(parts) < 3 {
		return "unknown"
	}
	return parts[len(parts)-2]
}
func (m *Manager) Create(reason string, maxCopies uint32) (CreatedBackup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	created, e := m.CreateIn(m.Dir(), reason)
	if e != nil {
		return created, e
	}
	_, e = Prune(m.Dir(), maxCopies)
	if e != nil {
		message := "Backup created; local retention cleanup failed: " + e.Error()
		created.Warning = &message
	}
	return created, nil
}
func (m *Manager) CreateIn(dir, reason string) (out CreatedBackup, err error) {
	if e := guard(m.root, dir); e != nil {
		return out, e
	}
	dir, e := paths.ValidatePath(dir)
	if e != nil {
		return out, e
	}
	if e = ensureDir(dir); e != nil {
		return out, e
	}
	tmp, e := os.MkdirTemp(dir, ".snapshot-")
	if e != nil {
		return out, e
	}
	defer func() { err = errors.Join(err, os.RemoveAll(tmp)) }()
	if e = fileops.SecureDirectory(tmp); e != nil {
		return out, e
	}
	db := filepath.Join(tmp, "xiaobai-switch.db")
	if e = m.store.SnapshotTo(db); e != nil {
		return out, e
	}
	keyPath := filepath.Join(m.root, "master.key")
	if e = guard(m.root, keyPath); e != nil {
		return out, e
	}
	if _, e = regular(keyPath); e != nil {
		return out, e
	}
	key, e := os.ReadFile(keyPath)
	if e != nil {
		return out, e
	}
	if len(key) != 32 {
		return out, invalid("master.key must contain 32 bytes")
	}
	if e = validateDatabase(db, key); e != nil {
		return out, e
	}
	info, e := regular(db)
	if e != nil {
		return out, e
	}
	if info.Size() > MaxArchiveBytes {
		return out, invalid("database exceeds backup limit")
	}
	hash, e := hashFile(db)
	if e != nil {
		return out, e
	}
	ts := time.Now().UTC()
	device := DeviceName()
	name := Prefix + ts.Format("20060102_150405") + "." + device + "." + domain.NewID()[:8] + Suffix
	mft := Manifest{FormatVersion: 1, AppVersion: buildinfo.Version, CreatedAt: ts.UnixMilli(), DeviceName: device, Reason: reason, DatabaseSize: uint64(info.Size()), DatabaseSHA256: hash, MasterKeySize: 32, MasterKeySHA256: hashBytes(key)}
	stage := filepath.Join(tmp, "bundle.zip")
	if e = writeBundle(stage, db, key, mft); e != nil {
		return out, e
	}
	dest := filepath.Join(dir, name)
	if e = os.Rename(stage, dest); e != nil {
		return out, e
	}
	if e = syncDir(dir); e != nil {
		return out, e
	}
	return CreatedBackup{FileName: name, Path: dest}, nil
}
func (m *Manager) QueueLocalRestore(name string, maxCopies uint32) (domain.RestoreQueueResult, error) {
	path, e := m.Resolve(name)
	if e != nil {
		return domain.RestoreQueueResult{}, e
	}
	return m.QueueRestore(path, maxCopies)
}
func (m *Manager) QueueRestore(archive string, maxCopies uint32) (result domain.RestoreQueueResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	temp, e := os.MkdirTemp(m.root, ".local-restore-")
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, os.RemoveAll(temp)) }()
	if e = fileops.SecureDirectory(temp); e != nil {
		return result, e
	}
	staged := filepath.Join(temp, "backup.zip")
	if e = copyPrivate(archive, staged, MaxArchiveBytes); e != nil {
		return result, e
	}
	if _, e = m.CreateIn(m.Dir(), "pre_restore"); e != nil {
		return result, e
	}
	var warning *string
	if _, e = Prune(m.Dir(), maxCopies); e != nil {
		message := "Pre-restore backup created; retention cleanup failed: " + e.Error()
		warning = &message
	}
	if e = QueuePending(staged, m.root); e != nil {
		return result, e
	}
	return domain.RestoreQueueResult{RelaunchRequired: true, FileName: filepath.Base(archive), Warning: warning}, nil
}
func copyPrivate(source, target string, limit int64) error {
	info, e := regular(source)
	if e != nil {
		return e
	}
	if info.Size() > limit {
		return invalid("source exceeds size limit")
	}
	in, e := os.Open(source)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	if e = fileops.Secure(target); e != nil {
		out.Close()
		return e
	}
	n, copyErr := io.Copy(out, io.LimitReader(in, limit+1))
	e = errors.Join(copyErr, out.Sync(), out.Close())
	if n > limit {
		return invalid("source exceeds size limit")
	}
	return e
}

func guard(root, path string) error {
	if _, e := paths.ValidatePath(path); e != nil {
		return e
	}
	rel, e := filepath.Rel(root, path)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return invalid("backup path escaped application root")
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, e := os.Lstat(current)
		if e == nil && info.Mode()&os.ModeSymlink != 0 {
			return invalid("backup paths cannot contain symbolic links")
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	return nil
}
