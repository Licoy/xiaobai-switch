//go:build windows

package fileops

import "testing"

func assertPrivate(t *testing.T, path string) {
	t.Helper()
	if e := Secure(path); e != nil {
		t.Fatal(e)
	}
}
