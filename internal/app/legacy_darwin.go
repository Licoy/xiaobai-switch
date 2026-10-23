//go:build darwin

package app

import (
	"errors"
	"net"
	"os"
	"syscall"
	"time"
)

// The socket and framing are from tauri-plugin-single-instance 2.4.3
func activateLegacy() (bool, error) {
	const socket = "/tmp/com_github_licoy_xiaobai_switch_app_si.sock"
	info, err := os.Lstat(socket)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return false, errors.New("legacy instance endpoint is not a socket")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || int(stat.Uid) != os.Getuid() {
		return false, errors.New("legacy instance socket owner does not match current user")
	}
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer conn.Close()
	if err = conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		return true, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return true, err
	}
	_, err = conn.Write([]byte(cwd + "\x00\x00XiaoBaiSwitch"))
	return true, err
}
