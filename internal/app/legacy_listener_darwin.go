//go:build darwin

package app

import (
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

func reserveLegacy(activate func([]string)) (func() error, error) {
	return reserveLegacySocket("/tmp/com_github_licoy_xiaobai_switch_app_si.sock", activate)
}
func ownedSocket(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if info.Mode()&os.ModeSocket == 0 || !ok || int(stat.Uid) != os.Getuid() {
		return nil, errors.New("legacy endpoint is not a socket owned by this user")
	}
	return info, nil
}
func removeStaleSocket(path string) error {
	original, err := ownedSocket(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for range 2 {
		conn, err := net.DialTimeout("unix", path, 300*time.Millisecond)
		if err == nil {
			conn.Close()
			return errors.New("legacy instance endpoint is active")
		}
		if !errors.Is(err, syscall.ECONNREFUSED) {
			return err
		}
		current, err := ownedSocket(path)
		if err != nil {
			return err
		}
		if !os.SameFile(original, current) {
			return errors.New("legacy endpoint changed during stale verification")
		}
	}
	return os.Remove(path)
}
func reserveLegacySocket(path string, activate func([]string)) (func() error, error) {
	if err := removeStaleSocket(path); err != nil {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	listener.SetUnlinkOnClose(false)
	owned, err := ownedSocket(path)
	if err != nil {
		listener.Close()
		return nil, err
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					log.Printf("legacy activation accept failed: %v", err)
				}
				return
			}
			go readLegacyActivation(conn, activate)
		}
	}()
	release := func() error {
		closeErr := listener.Close()
		current, statErr := ownedSocket(path)
		if errors.Is(statErr, os.ErrNotExist) {
			return closeErr
		}
		if statErr != nil {
			return errors.Join(closeErr, statErr)
		}
		if !os.SameFile(owned, current) {
			return errors.Join(closeErr, errors.New("legacy endpoint changed owner during runtime"))
		}
		return errors.Join(closeErr, os.Remove(path))
	}
	return release, nil
}
func readLegacyActivation(conn net.Conn, activate func([]string)) {
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		log.Printf("legacy activation deadline failed: %v", err)
		return
	}
	data, err := io.ReadAll(io.LimitReader(conn, 64*1024+1))
	if err != nil {
		log.Printf("legacy activation read failed: %v", err)
		return
	}
	if len(data) > 64*1024 || !utf8.Valid(data) {
		log.Print("legacy activation rejected: invalid framing")
		return
	}
	_, args, ok := strings.Cut(string(data), "\x00\x00")
	if !ok {
		if len(data) > 0 {
			log.Print("legacy activation rejected: missing delimiter")
		}
		return
	}
	activate(strings.Split(args, "\x00"))
}
