package app

import (
	"errors"
	"github.com/licoy/xiaobai-switch/native"
	"strings"
	"testing"
)

func TestErrorDetailsRedactCredentialsAndKeepCause(t *testing.T) {
	s := testShell(t)
	s.editSite(nil)
	s.Editor.Key = "opaque-fixture-value"
	s.Editor.ExtraKeys = []string{"extra-fixture-value"}
	raw := "rollback failed: permission denied; key=opaque-fixture-value; api_key=extra-fixture-value; Authorization: Bearer fixture-token; https://username:fixture-password@example.test/path?apikey=query-fixture; sk-fixture-token"
	s.result(errors.New(raw))
	for _, secret := range []string{"opaque-fixture-value", "extra-fixture-value", "fixture-token", "fixture-password", "query-fixture", "sk-fixture-token"} {
		if strings.Contains(s.ErrorDetail, secret) {
			t.Fatalf("fake credential was not redacted")
		}
	}
	if !strings.Contains(s.ErrorDetail, "permission denied") || !strings.Contains(s.ErrorDetail, "rollback failed") {
		t.Fatal("technical cause was hidden")
	}
	for _, file := range []string{"icons/LOBE-LICENSE", "icons/REMIX-LICENSE"} {
		if _, err := native.Icons.ReadFile(file); err != nil {
			t.Fatal(err)
		}
	}
}
