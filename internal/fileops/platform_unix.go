//go:build !windows

package fileops

import (
	"golang.org/x/sys/unix"
	"os"
)

func Secure(path string) error          { return os.Chmod(path, 0600) }
func replace(source, dest string) error { return os.Rename(source, dest) }
func syncParent(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func lockFile(f *os.File) error         { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
func unlockFile(f *os.File) error       { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
func SecureDirectory(path string) error { return os.Chmod(path, 0700) }
