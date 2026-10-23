package appbackup

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const Prefix = "xiaobai-switch-backup-"
const Suffix = ".zip"
const MaxArchiveBytes int64 = 512 * 1024 * 1024
const maxManifestBytes int64 = 64 * 1024

type Manifest struct {
	FormatVersion   uint32 `json:"formatVersion"`
	AppVersion      string `json:"appVersion"`
	CreatedAt       int64  `json:"createdAt"`
	DeviceName      string `json:"deviceName"`
	Reason          string `json:"reason"`
	DatabaseSize    uint64 `json:"databaseSize"`
	DatabaseSHA256  string `json:"databaseSha256"`
	MasterKeySize   uint64 `json:"masterKeySize"`
	MasterKeySHA256 string `json:"masterKeySha256"`
}
type CreatedBackup struct {
	Warning  *string `json:"warning,omitempty"`
	FileName string  `json:"fileName"`
	Path     string  `json:"path"`
}
type ValidatedBackup struct {
	DatabasePath  string
	MasterKeyPath string
	Manifest      Manifest
}

func invalid(message string) error { return domain.NewError("backup_invalid", message) }
func hashFile(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func hashBytes(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func regular(path string) (os.FileInfo, error) {
	if _, e := paths.ValidatePath(path); e != nil {
		return nil, e
	}
	info, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, invalid("backup source must be a regular file")
	}
	return info, nil
}
func safeName(name string) bool {
	return filepath.Base(name) == name && strings.HasPrefix(name, Prefix) && strings.HasSuffix(name, Suffix) && !strings.ContainsAny(name, "/\\\x00") && !strings.Contains(name, "..")
}
func writeBundle(path, db string, key []byte, m Manifest) (err error) {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	if e = fileops.Secure(path); e != nil {
		return e
	}
	z := zip.NewWriter(f)
	header := &zip.FileHeader{Name: "xiaobai-switch.db", Method: zip.Deflate}
	header.SetMode(0600)
	w, e := z.CreateHeader(header)
	if e != nil {
		return e
	}
	input, e := os.Open(db)
	if e != nil {
		return e
	}
	_, copyErr := io.Copy(w, input)
	e = errors.Join(copyErr, input.Close())
	if e != nil {
		return e
	}
	raw, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	for _, entry := range []struct {
		name string
		data []byte
	}{{"master.key", key}, {"manifest.json", raw}} {
		h := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		h.SetMode(0600)
		w, e = z.CreateHeader(h)
		if e != nil {
			return e
		}
		if _, e = w.Write(entry.data); e != nil {
			return e
		}
	}
	if e = z.Close(); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	info, e := f.Stat()
	if e != nil {
		return e
	}
	if info.Size() > MaxArchiveBytes {
		return invalid("backup archive is too large")
	}
	return nil
}
func ValidateAndExtract(archive, destination string) (out ValidatedBackup, err error) {
	info, e := regular(archive)
	if e != nil {
		return out, e
	}
	if info.Size() > MaxArchiveBytes {
		return out, invalid("backup archive is too large")
	}
	z, e := zip.OpenReader(archive)
	if e != nil {
		return out, invalid("invalid ZIP archive")
	}
	defer z.Close()
	if len(z.File) != 3 {
		return out, invalid("backup requires exactly three entries")
	}
	limits := map[string]int64{"xiaobai-switch.db": MaxArchiveBytes, "master.key": 32, "manifest.json": maxManifestBytes}
	seen := map[string]bool{}
	for _, entry := range z.File {
		limit, ok := limits[entry.Name]
		if !ok || seen[entry.Name] || !entry.Mode().IsRegular() || entry.UncompressedSize64 > uint64(limit) {
			return out, invalid("unsupported, duplicate, linked, or oversized ZIP entry")
		}
		seen[entry.Name] = true
	}
	if _, e = paths.ValidatePath(destination); e != nil {
		return out, e
	}
	if e = os.Mkdir(destination, 0700); e != nil {
		return out, e
	}
	if e = fileops.SecureDirectory(destination); e != nil {
		return out, e
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(destination))
		}
	}()
	for _, entry := range z.File {
		r, e := entry.Open()
		if e != nil {
			return out, e
		}
		path := filepath.Join(destination, entry.Name)
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			r.Close()
			return out, e
		}
		if e = fileops.Secure(path); e != nil {
			f.Close()
			r.Close()
			return out, e
		}
		copied, copyErr := io.Copy(f, io.LimitReader(r, limits[entry.Name]+1))
		e = errors.Join(copyErr, f.Sync(), f.Close(), r.Close())
		if e != nil {
			return out, e
		}
		if copied > limits[entry.Name] || uint64(copied) != entry.UncompressedSize64 {
			return out, invalid("backup entry size mismatch")
		}
	}
	return ValidatePayload(destination)
}
func ValidatePayload(dir string) (ValidatedBackup, error) {
	if _, e := paths.ValidatePath(dir); e != nil {
		return ValidatedBackup{}, e
	}
	info, e := os.Lstat(dir)
	if e != nil {
		return ValidatedBackup{}, e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ValidatedBackup{}, invalid("payload directory cannot be a symlink")
	}
	out := ValidatedBackup{DatabasePath: filepath.Join(dir, "xiaobai-switch.db"), MasterKeyPath: filepath.Join(dir, "master.key")}
	manifestPath := filepath.Join(dir, "manifest.json")
	mi, e := regular(manifestPath)
	if e != nil {
		return out, e
	}
	if mi.Size() > maxManifestBytes {
		return out, invalid("manifest is too large")
	}
	raw, e := os.ReadFile(manifestPath)
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(raw, &out.Manifest); e != nil {
		return out, invalid("invalid backup manifest")
	}
	m := out.Manifest
	if m.FormatVersion != 1 || m.DatabaseSize > uint64(MaxArchiveBytes) || m.MasterKeySize != 32 {
		return out, invalid("unsupported backup format or size")
	}
	di, e := regular(out.DatabasePath)
	if e != nil {
		return out, e
	}
	if uint64(di.Size()) != m.DatabaseSize {
		return out, invalid("database size mismatch")
	}
	dbHash, e := hashFile(out.DatabasePath)
	if e != nil {
		return out, e
	}
	if dbHash != m.DatabaseSHA256 {
		return out, invalid("database checksum mismatch")
	}
	ki, e := regular(out.MasterKeyPath)
	if e != nil {
		return out, e
	}
	if ki.Size() != 32 {
		return out, invalid("master.key must contain 32 bytes")
	}
	key, e := os.ReadFile(out.MasterKeyPath)
	if e != nil {
		return out, e
	}
	if hashBytes(key) != m.MasterKeySHA256 {
		return out, invalid("master key checksum mismatch")
	}
	if e = validateDatabase(out.DatabasePath, key); e != nil {
		return out, fmt.Errorf("backup_invalid: %w", e)
	}
	return out, nil
}
