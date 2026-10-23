package skills

import (
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicationFailureRestoresAndRetainsRecoveryOnRollbackFailure(t *testing.T) {
	for _, failRollback := range []bool{false, true} {
		t.Run(map[bool]string{false: "rollback", true: "recovery"}[failRollback], func(t *testing.T) {
			m := testManager(t)
			old := archiveBytes(t, []zipItem{{"repo/skills/one/SKILL.md", skill("one", "old"), 0600}, {"repo/skills/two/SKILL.md", skill("two", "old"), 0600}})
			if _, e := m.InstallArchive(domain.SkillCodex, old, "owner/repo", ""); e != nil {
				t.Fatal(e)
			}
			updated := archiveBytes(t, []zipItem{{"repo/skills/one/SKILL.md", skill("one", "new"), 0600}, {"repo/skills/two/SKILL.md", skill("two", "new"), 0600}})
			m.rename = func(source, dest string) error {
				if strings.Contains(source, string(filepath.Separator)+"extract"+string(filepath.Separator)) && filepath.Base(dest) == "two" {
					return errors.New("injected second publish failure")
				}
				if failRollback && strings.Contains(source, "previous-0") {
					return errors.New("injected rollback failure")
				}
				return os.Rename(source, dest)
			}
			_, e := m.InstallArchive(domain.SkillCodex, updated, "owner/repo", "")
			if e == nil {
				t.Fatal("reported installation success")
			}
			if failRollback {
				matches, ge := filepath.Glob(filepath.Join(m.roots[domain.SkillCodex], ".xiaobai-skill-*", "previous-0", enabledFile))
				if ge != nil || len(matches) != 1 || !strings.Contains(e.Error(), "recovery files retained") {
					t.Fatalf("lost recovery %+v %v", matches, e)
				}
				b, re := os.ReadFile(matches[0])
				if re != nil || !strings.Contains(string(b), "old") {
					t.Fatal("deleted old skill backup")
				}
			} else {
				b, re := os.ReadFile(filepath.Join(m.roots[domain.SkillCodex], "one", enabledFile))
				if re != nil || !strings.Contains(string(b), "old") {
					t.Fatal("failed to restore first installation")
				}
			}
		})
	}
}
