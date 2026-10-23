//go:build windows

package app

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestLegacyWindowCopyDataPointerCallback(t *testing.T) {
	ready := make(chan legacyWindowResult, 1)
	done := make(chan error, 1)
	activated := make(chan []string, 1)
	go legacyWindowLoop(ready, done, func(args []string) { activated <- args })
	result := <-ready
	if result.err != nil {
		t.Fatal(result.err)
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	t.Cleanup(func() {
		ok, _, err := user32.NewProc("PostMessageW").Call(result.window, 0x0010, 0, 0)
		if ok == 0 {
			t.Errorf("close legacy window: %v", err)
			return
		}
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("legacy window did not close")
		}
	})
	payload := []byte("directory|XiaoBaiSwitch|xiaobaiswitch://sites\x00")
	data := struct {
		Tag  uintptr
		Size uint32
		Data unsafe.Pointer
	}{1542, uint32(len(payload)), unsafe.Pointer(&payload[0])}
	accepted, _, _ := user32.NewProc("SendMessageW").Call(result.window, 0x004A, 0, uintptr(unsafe.Pointer(&data)))
	if accepted != 1 {
		t.Fatal("legacy window rejected COPYDATA")
	}
	for i := range payload {
		payload[i] = 'x'
	}
	select {
	case args := <-activated:
		if !reflect.DeepEqual(args, []string{"XiaoBaiSwitch", "xiaobaiswitch://sites"}) {
			t.Fatalf("activation did not copy arguments: %q", args)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("legacy window did not activate")
	}
}
