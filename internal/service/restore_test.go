package service

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
)

func TestPrimeOfficialRestoreAfterConfigReformat(t *testing.T) {
	s, st, site := fixtureService(t)
	applyFixture(t, s, site, domain.Prime)
	binding, err := st.GetBinding(domain.Prime)
	if err != nil || binding == nil {
		t.Fatalf("missing Prime binding: %v", err)
	}
	before := reformatPrimeConfig(t, binding.ManagedPaths)
	if err = s.RestoreOfficial(context.Background(), domain.Prime); err != nil {
		t.Fatal(err)
	}
	if binding, err = st.GetBinding(domain.Prime); err != nil || binding != nil {
		t.Fatalf("restore left Prime binding: %v", err)
	}
	for path := range before {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("xiaobai_")) {
			t.Fatalf("managed provider remains in %s", filepath.Base(path))
		}
		var fields map[string]any
		if err = json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		if filepath.Base(path) == "settings.json" && (len(fields) != 1 || fields["theme"] != "dark") {
			t.Fatal("restore must preserve theme and remove managed defaults")
		}
		if filepath.Base(path) == "auth.json" {
			auth, ok := fields["official"].(map[string]any)
			if !ok || len(fields) != 1 || auth["type"] != "oauth" || auth["refresh"] != "fixture-refresh" {
				t.Fatal("restore changed official OAuth credentials")
			}
		}
	}
	assertRestoreBackup(t, s, before)
}

func reformatPrimeConfig(t *testing.T, paths []string) map[string][]byte {
	t.Helper()
	before := map[string][]byte{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err = json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		switch filepath.Base(path) {
		case "settings.json":
			fields["theme"] = "dark"
		case "auth.json":
			fields["official"] = map[string]any{"type": "oauth", "refresh": "fixture-refresh"}
		}
		raw, err = json.MarshalIndent(fields, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		before[path] = append(raw, '\n')
		if err = os.WriteFile(path, before[path], 0600); err != nil {
			t.Fatal(err)
		}
	}
	return before
}

func assertRestoreBackup(t *testing.T, s *Service, before map[string][]byte) {
	t.Helper()
	backups, err := s.ListBackups(context.Background())
	if err != nil || len(backups) != 2 {
		t.Fatalf("expected apply and restore backups: %v", err)
	}
	files, err := fileops.InspectBackup(backups[0].Dir)
	if err != nil || len(files) != len(before) {
		t.Fatalf("restore backup missing files: %v", err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(filepath.Join(backups[0].Dir, file.Name))
		if err != nil {
			t.Fatal(err)
		}
		if !file.Existed || !bytes.Equal(raw, before[file.OriginalPath]) {
			t.Fatalf("restore backup changed original %s", filepath.Base(file.OriginalPath))
		}
	}
}
