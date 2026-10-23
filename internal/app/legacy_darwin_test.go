//go:build darwin

package app

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func shortSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "xbs-legacy-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return filepath.Join(dir, "si.sock")
}
func TestLegacyStaleSocketRecoveryAndForwarding(t *testing.T) {
	path := shortSocket(t)
	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	stale.SetUnlinkOnClose(false)
	if err = stale.Close(); err != nil {
		t.Fatal(err)
	}
	received := make(chan []string, 1)
	release, err := reserveLegacySocket(path, func(args []string) { received <- args })
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	}()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Write([]byte("/tmp\x00\x00old-app\x00xiaobaiswitch://sites?name=Test&apikey=fake")); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	select {
	case args := <-received:
		if len(args) != 2 || !strings.HasPrefix(args[1], "xiaobaiswitch://sites?") {
			t.Fatal("deep link not forwarded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("activation not delivered")
	}
}
func TestLegacyActiveSocketIsNeverUnlinked(t *testing.T) {
	path := shortSocket(t)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reserveLegacySocket(path, func([]string) {}); err == nil {
		t.Fatal("claimed active legacy socket")
	}
	after, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("active legacy endpoint was replaced")
	}
}
