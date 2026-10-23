//go:build windows

package app

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

type legacyPoint struct{ X, Y int32 }
type legacyMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          legacyPoint
	Private        uint32
}
type legacyWindowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Class                        *uint16
	SmallIcon                          uintptr
}
type legacyWindowResult struct {
	window uintptr
	err    error
}

// Class, title, mutex and WM_COPYDATA tag match tauri-plugin-single-instance 2.4.3
func reserveLegacy(activate func([]string)) (func() error, error) {
	const id = "com.github.licoy.xiaobai-switch.app"
	var mutex windows.Handle
	ready := make(chan legacyWindowResult, 1)
	done := make(chan error, 1)
	go legacyWindowLoop(ready, done, activate)
	result := <-ready
	if result.err != nil {
		return nil, result.err
	}
	close := func() error {
		post := windows.NewLazySystemDLL("user32.dll").NewProc("PostMessageW")
		ok, _, postErr := post.Call(result.window, 0x0010, 0, 0)
		if ok == 0 {
			return errors.Join(postErr, closeLegacyMutex(mutex))
		}
		select {
		case loopErr := <-done:
			return errors.Join(loopErr, closeLegacyMutex(mutex))
		case <-time.After(3 * time.Second):
			return errors.Join(errors.New("legacy activation window did not close"), closeLegacyMutex(mutex))
		}
	}
	mutex, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(id+"-sim"))
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, errors.Join(errors.New("legacy instance mutex is already held"), close())
	}
	if err != nil {
		return nil, errors.Join(err, close())
	}
	return close, nil
}
func legacyWindowLoop(ready chan<- legacyWindowResult, done chan<- error, activate func([]string)) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	user32 := windows.NewLazySystemDLL("user32.dll")
	def := user32.NewProc("DefWindowProcW")
	destroy := user32.NewProc("DestroyWindow")
	quit := user32.NewProc("PostQuitMessage")
	callback := syscall.NewCallback(func(window uintptr, message uint32, wparam, lparam uintptr) uintptr {
		switch message {
		case 0x004A:
			if lparam != 0 && *(*uintptr)(unsafe.Pointer(lparam)) == 1542 {
				data := (*struct {
					Tag  uintptr
					Size uint32
					Data unsafe.Pointer
				})(unsafe.Pointer(lparam))
				if data.Size > 0 && data.Size <= 64*1024 && data.Data != nil {
					raw := unsafe.Slice((*byte)(data.Data), data.Size)
					if utf8.Valid(raw) {
						text := strings.TrimSuffix(string(raw), "\x00")
						_, args, ok := strings.Cut(text, "|")
						if ok {
							values := strings.Split(args, "|")
							go activate(values)
						}
					}
				}
				return 1
			}
		case 0x0010:
			destroy.Call(window)
			return 0
		case 0x0002:
			quit.Call(0)
			return 0
		}
		result, _, _ := def.Call(window, uintptr(message), wparam, lparam)
		return result
	})
	const id = "com.github.licoy.xiaobai-switch.app"
	class := windows.StringToUTF16Ptr(id + "-sic")
	title := windows.StringToUTF16Ptr(id + "-siw")
	module, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	wc := legacyWindowClass{Proc: callback, Instance: module, Class: class}
	wc.Size = uint32(unsafe.Sizeof(wc))
	registered, _, err := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))
	if registered == 0 {
		ready <- legacyWindowResult{err: fmt.Errorf("register legacy activation window: %w", err)}
		return
	}
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(class)), module)
	window, _, err := user32.NewProc("CreateWindowExW").Call(0x00000080|0x08000000, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0, 0, 0, 0, 0, 0, 0, module, 0)
	if window == 0 {
		ready <- legacyWindowResult{err: fmt.Errorf("create legacy activation window: %w", err)}
		return
	}
	ready <- legacyWindowResult{window: window}
	get := user32.NewProc("GetMessageW")
	dispatch := user32.NewProc("DispatchMessageW")
	translate := user32.NewProc("TranslateMessage")
	var message legacyMessage
	for {
		result, _, callErr := get.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) == -1 {
			done <- callErr
			return
		}
		if result == 0 {
			done <- nil
			return
		}
		translate.Call(uintptr(unsafe.Pointer(&message)))
		dispatch.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func closeLegacyMutex(handle windows.Handle) error {
	if handle == 0 {
		return nil
	}
	return windows.CloseHandle(handle)
}
