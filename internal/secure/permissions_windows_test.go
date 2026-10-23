//go:build windows

package secure

import (
	"golang.org/x/sys/windows"
	"testing"
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
	// Require a protected DACL granting only the current user full access.
	if actual.String() != expected.String() {
		t.Fatalf("master key DACL %s, want %s", actual.String(), expected.String())
	}
}
