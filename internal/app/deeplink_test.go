package app

import (
	"strings"
	"testing"
)

func TestDeepLinkWithoutKeyPrefillsEditor(t *testing.T) {
	s := testShell(t)
	h := &host{shell: s}
	h.importURL("xiaobaiswitch://sites?name=Imported&baseurl=https%3A%2F%2Fexample.test%2Fv1&protocol=anthropic&notes=Note&codex-vision=true")
	if s.Editor == nil || s.Editor.Name != "Imported" || s.Editor.Key != "" || s.Editor.Protocol != "anthropic" || s.Editor.Notes != "Note" || !s.Editor.Capabilities["codex-vision"] {
		t.Fatal("missing-key deep link was not prefilled")
	}
	if len(s.Sites) != 0 {
		t.Fatal("missing-key import wrote data before user supplied a key")
	}
}
func TestDeepLinkConfirmationRedactsKeyAndCanCancel(t *testing.T) {
	s := testShell(t)
	h := &host{shell: s}
	h.importURL("xiaobaiswitch://sites?name=Imported&baseurl=https%3A%2F%2Fexample.test%2Fv1&apikey=fixture-import-secret&protocol=anthropic&notes=Note&codex-vision=true")
	if s.Confirm == nil {
		t.Fatal("missing import confirmation")
	}
	if strings.Contains(s.Confirm.Detail, "fixture-import-secret") || !strings.Contains(s.Confirm.Detail, "anthropic") || !strings.Contains(s.Confirm.Detail, "codex-vision") {
		t.Fatal("confirmation leaked key or lost import details")
	}
	s.Confirm.Dismiss()
	s.Confirm = nil
	if len(s.Sites) != 0 {
		t.Fatal("cancelled import wrote a site")
	}
}

func TestDeepLinkWithoutProtocolKeepsEditorDefault(t *testing.T) {
	s := testShell(t)
	s.editSite(nil)
	expected := s.Editor.Protocol
	s.clearEditor()
	h := &host{shell: s}
	h.importURL("xiaobaiswitch://sites?name=Imported&baseurl=https%3A%2F%2Fexample.test%2Fv1")
	if s.Editor == nil || s.Editor.Protocol != expected {
		t.Fatal("missing protocol overwrote editor default")
	}
}
