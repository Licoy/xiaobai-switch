//go:build windows

package secure

import (
	"golang.org/x/sys/windows"
	"testing"
	"unsafe"
)

func assertPrivateKey(t *testing.T, path string) {
	t.Helper()
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	expected, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	actual, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := actual.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("master key DACL allows inheritance")
	}
	acl, _, err := actual.DACL()
	if err != nil || acl == nil || acl.AceCount != 1 {
		t.Fatalf("master key must have exactly one access rule: %v, %s", err, actual.String())
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err = windows.GetAce(acl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	expectedACL, _, err := expected.DACL()
	if err != nil {
		t.Fatal(err)
	}
	var expectedACE *windows.ACCESS_ALLOWED_ACE
	if err = windows.GetAce(expectedACL, 0, &expectedACE); err != nil {
		t.Fatal(err)
	}
	// Auto-inheritance metadata on the descriptor does not grant inherited access.
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags != 0 || ace.Mask != expectedACE.Mask {
		t.Fatalf("master key access rule must explicitly allow full access: %s", actual.String())
	}
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !sid.Equals(user.User.Sid) {
		t.Fatalf("master key access granted to another user: %s", actual.String())
	}
}
