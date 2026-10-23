//go:build !windows

package fileops

import (
	"os"
	"testing"
)

func assertPrivate(t *testing.T, path string) {
	t.Helper()
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("expected 0600: %v", e)
	}
}
