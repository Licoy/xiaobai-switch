package skills

import (
	"context"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateUsesLegacyManifestAndPreservesDisabledOriginalDirectory(t *testing.T) {
	m := testManager(t)
	root := m.roots[domain.SkillCodex]
	legacy := filepath.Join(root, "legacy-repository")
	original := filepath.Join(legacy, "nested", "one")
	if e := os.MkdirAll(original, 0700); e != nil {
		t.Fatal(e)
	}
	manifest := installation{SourceRef: "owner/repo", InstalledAt: "2026-01-01T00:00:00Z"}
	raw, _ := json.Marshal(manifest)
	if e := os.WriteFile(filepath.Join(legacy, installManifest), raw, 0600); e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(original, disabledFile)
	if e := os.WriteFile(source, []byte(skill("one", "old")), 0600); e != nil {
		t.Fatal(e)
	}
	payload := archiveBytes(t, []zipItem{{"repo/skills/one/SKILL.md", skill("one", "new"), 0600}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(payload) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	base := m.client.Transport
	m.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		clone := r.Clone(r.Context())
		target := *r.URL
		target.Host = u.Host
		target.Scheme = u.Scheme
		clone.URL = &target
		return base.RoundTrip(clone)
	})
	result, e := m.Update(context.Background(), domain.SkillCodex, source)
	if e != nil || len(result.Paths) != 1 || result.Paths[0] != original {
		t.Fatalf("update %+v %v", result, e)
	}
	updated, e := os.ReadFile(source)
	if e != nil || !strings.Contains(string(updated), "new") {
		t.Fatal("did not update disabled original file")
	}
	if _, e = os.Stat(filepath.Join(original, enabledFile)); !os.IsNotExist(e) {
		t.Fatal("update enabled a disabled skill")
	}
	if _, e = os.Stat(filepath.Join(root, "one")); !os.IsNotExist(e) {
		t.Fatal("update installed a duplicate at another path")
	}
	if _, e = m.Update(context.Background(), domain.SkillPi, source); e == nil {
		t.Fatal("updated another target source")
	}
}
func TestManagedInstallUpdatePreservesDisabledState(t *testing.T) {
	m := testManager(t)
	payload := archiveBytes(t, []zipItem{{"repo/SKILL.md", skill("one", "old"), 0600}})
	r, e := m.InstallArchive(domain.SkillPrime, payload, "owner/repo", "")
	if e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(r.Paths[0], enabledFile)
	if e = m.SetEnabled(domain.SkillPrime, source, false); e != nil {
		t.Fatal(e)
	}
	payload = archiveBytes(t, []zipItem{{"repo/SKILL.md", skill("one", "new"), 0600}})
	if _, e = m.InstallArchive(domain.SkillPrime, payload, "owner/repo", ""); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(r.Paths[0], disabledFile)); e != nil {
		t.Fatal("disabled state lost")
	}
}
