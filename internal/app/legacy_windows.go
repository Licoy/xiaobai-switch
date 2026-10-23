//go:build windows

package app

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"syscall"
	"unsafe"
)

func activateLegacy() (bool, error) {
	const id = "com.github.licoy.xiaobai-switch.app"
	mutex, err := windows.OpenMutex(windows.SYNCHRONIZE, false, windows.StringToUTF16Ptr(id+"-sim"))
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(mutex)
	user32 := windows.NewLazySystemDLL("user32.dll")
	find := user32.NewProc("FindWindowW")
	send := user32.NewProc("SendMessageTimeoutW")
	hwnd, _, _ := find.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(id+"-sic"))), uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(id+"-siw"))))
	if hwnd == 0 {
		return true, fmt.Errorf("legacy instance exists but activation window is unavailable")
	}
	payload := []byte("|XiaoBaiSwitch\x00")
	data := struct {
		Tag  uintptr
		Size uint32
		Data unsafe.Pointer
	}{1542, uint32(len(payload)), unsafe.Pointer(&payload[0])}
	var result uintptr
	ok, _, callErr := send.Call(hwnd, 0x004A, 0, uintptr(unsafe.Pointer(&data)), 0x2, 1000, uintptr(unsafe.Pointer(&result)))
	if ok == 0 {
		if callErr == syscall.Errno(0) {
			return true, fmt.Errorf("legacy instance activation timed out")
		}
		return true, callErr
	}
	return true, nil
}
