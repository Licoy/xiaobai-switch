package envinject

import (
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestModes(t *testing.T) {
	for _, row := range []struct {
		mode, platform, want string
		fail                 bool
	}{{"auto", "windows", "user_env", false}, {"auto", "darwin", "shell_rc", false}, {"shell_rc", "linux", "shell_rc", false}, {"user_env", "windows", "user_env", false}, {"user_env", "linux", "", true}, {"file_only", "windows", "file_only", false}, {"bad", "linux", "", true}} {
		got, e := EffectiveMode(row.mode, row.platform)
		if (e != nil) != row.fail || got != row.want {
			t.Fatalf("mode %s/%s: %s %v", row.mode, row.platform, got, e)
		}
	}
}
func TestEnvShellLiteralHasNoExpansion(t *testing.T) {
	key := "XIAOBAI_SITE_A_API_KEY"
	value := "sk-'\"$HOME`echo exposed`$(echo exposed)\\"
	out, e := Upsert([]byte("# keep\nexport OTHER='ok'\n"), key, value, false)
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	path := filepath.Join(root, "codex.env")
	if e = os.WriteFile(path, out, 0600); e != nil {
		t.Fatal(e)
	}
	shell, e := exec.LookPath("sh")
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(shell, "-c", ". \"$1\"; printf %s \"$XIAOBAI_SITE_A_API_KEY\"", "test", filepath.ToSlash(path))
	got, e := cmd.Output()
	if e != nil || string(got) != value {
		t.Fatalf("literal changed or executed: %v", e)
	}
	if !strings.Contains(string(out), "OTHER='ok'") {
		t.Fatal("other key lost")
	}
	out, e = Upsert(out, key, "next", false)
	if e != nil || strings.Count(string(out), key+"=") != 1 {
		t.Fatal("duplicate key")
	}
	out, e = Upsert(out, key, "", true)
	if e != nil || strings.Contains(string(out), key+"=") {
		t.Fatal("key not removed")
	}
}
func TestSourceBlockLegacyAndEscaping(t *testing.T) {
	original := []byte("alias ll='ls'\n# >>> Xiaobai Switch >>>\nold\n# <<< Xiaobai Switch <<<\nkeep\n")
	envPath := filepath.Join(t.TempDir(), "my 'env'$file")
	out, e := SourceBlock(original, envPath)
	if e != nil {
		t.Fatal(e)
	}
	again, e := SourceBlock(out, envPath)
	if e != nil || string(again) != string(out) {
		t.Fatal("not idempotent")
	}
	if !strings.Contains(string(out), "alias ll='ls'") || !strings.Contains(string(out), "keep") {
		t.Fatal("user content lost")
	}
	if _, e = SourceBlock([]byte(begin), envPath); e == nil {
		t.Fatal("unterminated block accepted")
	}
}
func TestBuildOnlyUsesGivenFixturePaths(t *testing.T) {
	root := t.TempDir()
	p, e := Build(Request{Mode: "shell_rc", Platform: "darwin", Home: root, EnvPath: filepath.Join(root, "codex.env"), Key: "KEY", Value: "secret"})
	if e != nil || len(p.Files) != 2 {
		t.Fatalf("build %v %d", e, len(p.Files))
	}
	if _, e = os.Stat(filepath.Join(root, ".zshrc")); !os.IsNotExist(e) {
		t.Fatal("preview changed filesystem")
	}
}
func TestUserEnvironmentFailureRollsFilesAndRegistryBack(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "codex.env")
	if e := os.WriteFile(path, []byte("old-file"), 0600); e != nil {
		t.Fatal(e)
	}
	change, e := fileops.Snapshot(path, []byte("new-file"))
	if e != nil {
		t.Fatal(e)
	}
	value := "old-key"
	writes := 0
	read := func(string) (string, bool, error) { return value, true, nil }
	write := func(key, v string, remove bool) error {
		writes++
		value = v
		if writes == 1 {
			return errors.New("injected registry failure")
		}
		return nil
	}
	_, e = execute(Plan{Mode: "user_env", Files: []fileops.Change{change}, UserKey: "KEY", UserValue: "new-key"}, filepath.Join(root, "backup"), read, write)
	if e == nil {
		t.Fatal("expected registry failure")
	}
	if value != "old-key" {
		t.Fatal("registry snapshot not restored")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "old-file" {
		t.Fatal("files not restored")
	}
}
func TestUserEnvironmentTransactionSuccess(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "codex.env")
	change, e := fileops.Snapshot(path, []byte("new-file"))
	if e != nil {
		t.Fatal(e)
	}
	value := "old-key"
	read := func(string) (string, bool, error) { return value, true, nil }
	write := func(key, v string, remove bool) error { value = v; return nil }
	_, e = execute(Plan{Mode: "user_env", Files: []fileops.Change{change}, UserKey: "KEY", UserValue: "new-key"}, filepath.Join(root, "backup"), read, write)
	if e != nil {
		t.Fatal(e)
	}
	if value != "new-key" {
		t.Fatal("registry unchanged")
	}
	raw, _ := os.ReadFile(filepath.Join(root, "backup") + ".user-env.json")
	if !strings.Contains(string(raw), `"state":"committed"`) {
		t.Fatal("missing registry journal")
	}
}
func TestTerminalRegistrySnapshotCannotHidePendingFileRecovery(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(root, "backup")
	if e := os.Mkdir(backup, 0700); e != nil {
		t.Fatal(e)
	}
	live := filepath.Join(root, "live")
	if e := os.WriteFile(live, []byte("new"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(backup, "000.bak"), []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(map[string]any{"state": "prepared", "files": []any{map[string]any{"path": live, "backup": "000.bak", "existed": true, "applied": true}}})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(backup, "transaction.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	if e = saveUser(backup+".user-env.json", userJournal{Key: "KEY", State: "rolled_back"}); e != nil {
		t.Fatal(e)
	}
	if e = Recover(backup); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(live)
	if string(got) != "old" {
		t.Fatal("terminal registry snapshot hid file recovery")
	}
}
