package webdav

import (
	"context"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/appbackup"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/store"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func settings() domain.AppSettings { s := domain.DefaultSettings(); s.ProxyMode = "none"; return s }
func clientFor(t *testing.T, base string) *Client {
	t.Helper()
	c, e := NewClient(RuntimeConfig{BaseURL: base, Username: "user", Password: "private-password", RemotePath: "backups"}, settings())
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func backupName(stamp, device string) string {
	return appbackup.Prefix + stamp + "." + device + ".12345678.zip"
}
func xmlList(names []string) string {
	var b strings.Builder
	b.WriteString(`<D:multistatus xmlns:D="DAV:">`)
	for _, name := range names {
		fmt.Fprintf(&b, `<D:response><D:href>/backups/%s</D:href><D:propstat><D:prop><D:getcontentlength>3</D:getcontentlength><D:getlastmodified>Thu, 01 Jan 2026 00:00:00 GMT</D:getlastmodified><D:resourcetype/></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`, name)
	}
	b.WriteString(`</D:multistatus>`)
	return b.String()
}
func TestClientMKCOLUploadDownloadListAndOwnDeviceCleanup(t *testing.T) {
	ownNew := backupName("20260102_000000", appbackup.DeviceName())
	ownOld := backupName("20260101_000000", appbackup.DeviceName())
	foreign := backupName("20250101_000000", "different-device")
	var mu sync.Mutex
	folder := false
	deleted := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "user" || password != "private-password" {
			t.Error("missing basic auth")
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "MKCOL":
			folder = true
			w.WriteHeader(201)
		case "PROPFIND":
			if !folder {
				w.WriteHeader(404)
				return
			}
			if r.Header.Get("Depth") == "1" {
				w.WriteHeader(207)
				io.WriteString(w, xmlList([]string{ownNew, ownOld, foreign}))
				return
			}
			w.WriteHeader(207)
		case "PUT":
			b, _ := io.ReadAll(r.Body)
			if string(b) != "ZIP" {
				t.Error("incorrect upload")
			}
			w.WriteHeader(201)
		case "GET":
			w.Write([]byte("ZIP"))
		case "DELETE":
			deleted = append(deleted, filepath.Base(r.URL.Path))
			w.WriteHeader(204)
		}
	}))
	defer srv.Close()
	c := clientFor(t, srv.URL)
	file := filepath.Join(t.TempDir(), "source.zip")
	if e := os.WriteFile(file, []byte("ZIP"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := c.Upload(context.Background(), ownNew, file); e != nil {
		t.Fatal(e)
	}
	list, e := c.List(context.Background())
	if e != nil || len(list) != 3 || list[0].Size != 3 {
		t.Fatalf("list %+v %v", list, e)
	}
	dest := filepath.Join(t.TempDir(), "download.zip")
	if e = c.Download(context.Background(), ownNew, dest); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(dest); e != nil || string(b) != "ZIP" {
		t.Fatal("download corrupted")
	}
	n, e := c.Cleanup(context.Background(), 1)
	if e != nil || n != 1 {
		t.Fatalf("cleanup %d %v", n, e)
	}
	mu.Lock()
	if len(deleted) != 1 || deleted[0] != ownOld {
		t.Errorf("removed other device %+v", deleted)
	}
	mu.Unlock()
	if e = c.Delete(context.Background(), foreign); e != nil {
		t.Fatal("manual foreign-device delete blocked", e)
	}
}
func TestUnsafeHrefInvalidXMLAndAuthenticationRedirect(t *testing.T) {
	c := clientFor(t, "http://example.test")
	name := backupName("20260101_000000", appbackup.DeviceName())
	for _, raw := range []string{`<D:multistatus xmlns:D="DAV:"><D:response><D:href>https://attacker.test/backups/` + name + `</D:href></D:response></D:multistatus>`, `<D:multistatus xmlns:D="DAV:"><D:response><D:href>/outside/` + name + `</D:href></D:response></D:multistatus>`, `<!DOCTYPE data SYSTEM "file:///etc/passwd"><multistatus/>`, `not XML`} {
		if _, e := c.parseListing([]byte(raw)); e == nil {
			t.Fatal("accepted unsafe WebDAV XML")
		}
	}
	var leaks atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaks.Add(1) }))
	defer other.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, 302) }))
	defer origin.Close()
	c = clientFor(t, origin.URL)
	if e := c.Check(context.Background()); e == nil {
		t.Fatal("followed authenticated cross-origin redirect")
	}
	if leaks.Load() != 0 {
		t.Fatal("credentials reached another origin")
	}
	for _, raw := range []string{"https://u:p@example.test", "https://example.test?x=1", "file:///tmp/x"} {
		if _, _, e := ValidateURLs(RuntimeConfig{BaseURL: raw, Username: "u", RemotePath: "b"}); e == nil {
			t.Fatal("accepted unsafe URL")
		}
	}
}
func managerFixture(t *testing.T) (*store.Store, *Manager) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s, e := store.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.SaveSettings(settings()); e != nil {
		t.Fatal(e)
	}
	m, e := NewManager(root, s, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close(); s.Close() })
	return s, m
}
func saveConfig(t *testing.T, m *Manager, base string, auto bool) {
	t.Helper()
	password := "private-password"
	_, e := m.SaveConfig(context.Background(), domain.SaveWebDavConfigInput{BaseURL: base, Username: "user", Password: &password, RemotePath: "backups", SyncIntervalMinutes: 15, MaxRemoteBackups: 1, AutoSyncEnabled: auto})
	if e != nil {
		t.Fatal(e)
	}
}
func TestUploadedBackupCleanupFailureIsWarningAndSavePreservesPassword(t *testing.T) {
	s, m := managerFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			if r.Header.Get("Depth") == "1" {
				w.WriteHeader(500)
			} else {
				w.WriteHeader(207)
			}
			return
		}
		w.WriteHeader(201)
	}))
	defer srv.Close()
	saveConfig(t, m, srv.URL, false)
	result, e := m.Backup(context.Background(), "webdav", "manual")
	if e != nil || !result.Uploaded || result.Warning == nil || strings.Contains(*result.Warning, "private-password") {
		t.Fatalf("upload %+v %v", result, e)
	}
	status, e := s.GetWebDAVSyncStatus()
	if e != nil || status.Status != "warning" || status.LastSuccessAt == nil {
		t.Fatalf("status %+v %v", status, e)
	}
	empty := ""
	if _, e = m.SaveConfig(context.Background(), domain.SaveWebDavConfigInput{BaseURL: srv.URL, Username: "user", Password: &empty, RemotePath: "backups", SyncIntervalMinutes: 15, MaxRemoteBackups: 1}); e != nil {
		t.Fatal(e)
	}
	password, e := s.GetWebDAVPassword()
	if e != nil || password != "private-password" {
		t.Fatal("empty password replaced existing secret")
	}
	now := time.Now().UnixMilli()
	last := now - 20*60000
	if got := NextScheduledAt(&last, now, 15*60000); got != now {
		t.Fatal("overdue schedule not immediate")
	}
	last = now - 5*60000
	if got := NextScheduledAt(&last, now, 15*60000); got != last+15*60000 {
		t.Fatal("schedule ignored last attempt")
	}
}
func TestCloseCancelsOperationAndWaitsForDatabaseFinalization(t *testing.T) {
	s, m := managerFixture(t)
	entered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			io.Copy(io.Discard, r.Body)
			close(entered)
			<-r.Context().Done()
			return
		}
		w.WriteHeader(207)
	}))
	defer srv.Close()
	saveConfig(t, m, srv.URL, true)
	var callbacks atomic.Int32
	m.onChange = func() { callbacks.Add(1) }
	finished := make(chan error, 1)
	go func() { _, e := m.Backup(context.Background(), "webdav", "manual"); finished <- e }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("backup did not reach upload")
	}
	if e := m.Close(); e != nil {
		t.Fatal(e)
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-finished:
		if e == nil {
			t.Fatal("cancelled upload reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("Close returned before operation finished")
	}
	if callbacks.Load() != 0 {
		t.Fatal("sent callback after close")
	}
}
func TestHTTPDownloadLimitAndRuntimeRecordingFailureAreVisible(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(appbackup.MaxArchiveBytes+1))
		w.WriteHeader(200)
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "too-big.zip")
	if e := clientFor(t, srv.URL).Download(context.Background(), backupName("20260101_000000", appbackup.DeviceName()), dest); e == nil {
		t.Fatal("accepted oversized download")
	}
	if _, e := os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("left partial oversized archive")
	}
	s, m := managerFixture(t)
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	m.reportSchedulerError(fmt.Errorf("operation failed"))
	if _, e := m.Overview(); e == nil {
		t.Fatal("discarded scheduler recording error")
	}
	if e := m.Close(); e == nil {
		t.Fatal("Close discarded asynchronous persistence error")
	}
}
