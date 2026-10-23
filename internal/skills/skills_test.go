package skills

import (
	"archive/zip"
	"bytes"
	"context"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	dev, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("XIAOBAI_DEV_HOME", dev)
	settings := domain.DefaultSettings()
	settings.ProxyMode = "none"
	client, e := netx.NewClient(settings, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	m, e := New(settings, client)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func skill(name, version string) string {
	return "---\nname: " + name + "\ndescription: |\n  Multi-line description\n  including YAML scalars\nmetadata:\n  author: Example\n  version: " + version + "\n---\n# Body\n"
}

type zipItem struct {
	name, content string
	mode          os.FileMode
}

func archiveBytes(t *testing.T, items []zipItem) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, item := range items {
		h := &zip.FileHeader{Name: item.name, Method: zip.Deflate}
		h.SetMode(item.mode)
		w, e := z.CreateHeader(h)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write([]byte(item.content)); e != nil {
			t.Fatal(e)
		}
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestLocalYAMLDetailToggleAndUninstall(t *testing.T) {
	m := testManager(t)
	root := m.roots[domain.SkillPi]
	dir := filepath.Join(root, "example")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(dir, enabledFile)
	if e := os.WriteFile(source, []byte(skill("example", "1.2")), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "script.sh"), []byte("echo hi"), 0700); e != nil {
		t.Fatal(e)
	}
	items, e := m.List()
	if e != nil || len(items) != 1 || items[0].Target != domain.SkillPi || items[0].Author == nil || *items[0].Version != "1.2" {
		t.Fatalf("list %+v %v", items, e)
	}
	detail, e := m.Detail(domain.SkillPi, source)
	if e != nil || len(detail.Files) != 2 || !strings.Contains(detail.Content, "# Body") {
		t.Fatalf("detail %+v %v", detail, e)
	}
	if e = m.SetEnabled(domain.SkillPi, source, false); e != nil {
		t.Fatal(e)
	}
	disabled := filepath.Join(dir, disabledFile)
	items, e = m.List()
	if e != nil || items[0].Enabled {
		t.Fatal("disable did not persist")
	}
	if e = m.SetEnabled(domain.SkillPi, disabled, true); e != nil {
		t.Fatal(e)
	}
	if e = m.Uninstall(domain.SkillPi, source); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(dir); !os.IsNotExist(e) {
		t.Fatal("uninstall retained directory")
	}
}
func TestArchiveSelectionPermissionsAndManagedUpdate(t *testing.T) {
	m := testManager(t)
	archive := archiveBytes(t, []zipItem{{"repo/skills/one/SKILL.md", skill("one", "1"), 0600}, {"repo/skills/one/run.sh", "#!/bin/sh\n", 0755}, {"repo/skills/two/SKILL.md", skill("two", "1"), 0600}})
	result, e := m.InstallArchive(domain.SkillCodex, archive, "owner/repo", "one")
	if e != nil || len(result.Names) != 1 || result.Names[0] != "one" {
		t.Fatalf("select %+v %v", result, e)
	}
	script := filepath.Join(result.Paths[0], "run.sh")
	info, e := os.Stat(script)
	if e != nil {
		t.Fatal(e)
	}
	if !info.Mode().IsRegular() {
		t.Fatal("installed script is not a regular file")
	}
	content, e := os.ReadFile(script)
	if e != nil || string(content) != "#!/bin/sh\n" {
		t.Fatalf("installed script changed: %q %v", content, e)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0711 {
		t.Fatalf("script permissions: %v, want 0711", info.Mode())
	}
	manifest, e := readInstallation(filepath.Join(result.Paths[0], installManifest))
	if e != nil || manifest.SourceRef != "owner/repo" || manifest.SkillName == nil || *manifest.SkillName != "one" || manifest.InstalledAt == "" {
		t.Fatalf("legacy manifest %+v %v", manifest, e)
	}
	if _, e = m.InstallArchive(domain.SkillCodex, archive, "different/repo", "one"); e == nil {
		t.Fatal("overwrote another source")
	}
	if _, e = m.InstallArchive(domain.SkillCodex, archive, "OWNER/REPO", "one"); e != nil {
		t.Fatal(e)
	}
	unmanaged := filepath.Join(m.roots[domain.SkillPrime], "one")
	if e = os.MkdirAll(unmanaged, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e = m.InstallArchive(domain.SkillPrime, archive, "owner/repo", ""); e == nil {
		t.Fatal("overwrote unmanaged directory")
	}
	if _, e = os.Stat(filepath.Join(m.roots[domain.SkillPrime], "two")); !os.IsNotExist(e) {
		t.Fatal("partial publish on invalid destination")
	}
}
func TestRejectArchiveTraversalSymlinkDuplicateAndDeepNesting(t *testing.T) {
	var e error
	m := testManager(t)
	for _, items := range [][]zipItem{{{"repo/../escape", "x", 0600}}, {{"repo/link", "/outside", os.ModeSymlink | 0600}}, {{"repo/SKILL.md", skill("one", "1"), 0600}, {"repo/skill.md", "duplicate", 0600}}, {{strings.Repeat("nested/", 13) + "SKILL.md", skill("one", "1"), 0600}}} {
		if _, e := m.InstallArchive(domain.SkillCodex, archiveBytes(t, items), "owner/repo", ""); e == nil {
			t.Fatal("accepted unsafe archive")
		}
	}
	outside := t.TempDir()
	root := m.roots[domain.SkillClaudeCode]
	if e := os.MkdirAll(filepath.Dir(root), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(outside, root); e != nil {
		t.Fatal(e)
	}
	if _, e = m.List(); e == nil {
		t.Fatal("followed escaped skills root")
	}
	if e = m.Uninstall(domain.SkillCodex, filepath.Join(outside, enabledFile)); e == nil {
		t.Fatal("accepted external source")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestMarketplaceAndVisibleDownloadFallback(t *testing.T) {
	m := testManager(t)
	payload := archiveBytes(t, []zipItem{{"repo/skills/one/SKILL.md", skill("one", "1"), 0600}})
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/search":
			io.WriteString(w, `{"skills":[{"name":"one","source":"owner/repo","installs":42}]}`)
		case r.URL.Path == "/search/repositories":
			io.WriteString(w, `{"items":[{"name":"repo","full_name":"owner/repo","stargazers_count":7}]}`)
		case strings.Contains(r.URL.Path, "HEAD.zip"):
			calls++
			w.WriteHeader(404)
		default:
			w.Write(payload)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	base := m.client.Transport
	m.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		copy := r.Clone(r.Context())
		target := *r.URL
		target.Host = u.Host
		target.Scheme = u.Scheme
		copy.URL = &target
		copy.Host = u.Host
		return base.RoundTrip(copy)
	})
	installed, e := m.Install(context.Background(), "owner/repo", domain.SkillAgents, "one")
	if e != nil || len(installed.Names) != 1 || len(installed.Warnings) != 1 || calls != 1 {
		t.Fatalf("fallback %+v %v", installed, e)
	}
	found, e := m.Search(context.Background(), "one", "skills.sh")
	if e != nil || len(found) != 1 || found[0].Installs != 42 || len(found[0].InstalledTargets) != 1 || found[0].InstalledTargets[0] != domain.SkillAgents {
		t.Fatalf("market %+v %v", found, e)
	}
	found, e = m.Search(context.Background(), "", "github")
	if e != nil || found[0].Stars != 7 {
		t.Fatalf("GitHub %+v %v", found, e)
	}
}
